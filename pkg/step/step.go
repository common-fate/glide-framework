package step

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/node"
	"github.com/common-fate/glide/pkg/noderr"
	"github.com/pkg/errors"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

type StepType int

const (
	CheckType   StepType = iota // a 'check'
	BooleanType                 // an 'and' or an 'or'
	RefType                     // a reference to a node (e.g. 'request' or 'approve')
	ActionType                  // an action to execute as part of a workflow
)

type Body interface {
	Type() StepType
	fmt.Stringer // implemented for debugging purposes
}

// Step is either a Node, or a boolean operation
type Step struct {
	// Position of the node in the list of statements.
	// This is set during graph compilation.
	// e.g. 0 is the first statement
	// 1 is the next
	// 1, 0 is the first child statement of the first statement
	//	- 0:
	//    - 1:
	//      - 1.1
	Position []int

	// Name is the friendly display name of the step.
	Name string

	// Body of the step
	Body     Body
	Children []Step

	// Node is the underlying YAML Node.
	// Used to pretty-print errors.
	Node ast.Node

	// Pass is the name of the Pass the statement is associated with.
	Pass string
}

// Label prints a human-friendly label for the step, to be used
// in graph representations.
func (e *Step) Label() string {
	if e.Name != "" {
		return e.Name
	}
	// otherwise, fall back to a string representation of the body
	return e.Body.String()
}

func (e *Step) UnmarshalYAML(ctx context.Context, b []byte) error {
	// the dialect must be defined in the context
	d, ok := dialect.FromContext(ctx)
	if !ok {
		return errors.New("glide dialect must be defined in context using glide.Use()")
	}

	// try and parse as a map (for a Check, Start, Action, or Outcome)
	//
	// e.g.
	// - check: condition
	//    ↑
	//   this node

	var mapNode map[string]ast.Node
	err := yaml.Unmarshal(b, &mapNode)
	if err == nil {
		// the value looks like this:
		// - foo: B
		// 'foo' might be 'start'
		body, ok := mapNode["start"]
		e.setNodePath(body)
		if ok {
			// it's a Start node
			return e.parseNodeRef(body, d, node.Start)
		}

		// the value looks like this:
		// - foo: B
		// 'foo' might be 'outcome'
		body, ok = mapNode["outcome"]
		e.setNodePath(body)
		if ok {
			// it's an Outcome node
			return e.parseNodeRef(body, d, node.Outcome)
		}

		// try and set the name of the node
		// the value might look like this:
		// - name: My node name
		//   foo: B

		nameNode, ok := mapNode["name"]
		if ok {
			// the 'name' key is present
			err = yaml.NodeToValue(nameNode, &e.Name)
			if err != nil {
				return errors.Wrap(err, "unmarshalling name")
			}
		}

		// the value looks like this:
		// - foo: B
		// 'foo' might be 'check'

		body, ok = mapNode["check"]
		e.setNodePath(body)
		if ok {
			// it's an If node
			var expr string
			err = yaml.NodeToValue(body, &expr)
			if err != nil {
				return err
			}

			e.Body = Check{Expression: expr}
			return nil
		}

		// check if we have an Action
		// e.g.
		// - action: approval

		body, ok = mapNode["action"]
		e.setNodePath(body)

		if ok {
			// it's an Action node

			if d.Actions == nil {
				err = errors.New("no actions are defined for this Glide dialect")
				return noderr.Wrap(err, body)
			}

			// extract the contents of the action field
			// e.g. 'approval'
			var actionType string
			err = yaml.NodeToValue(body, &actionType)
			if err != nil {
				return noderr.Wrap(err, body)
			}

			actions := d.Actions()
			action, ok := actions[actionType]
			if !ok {
				err := fmt.Errorf("unknown action type %s", actionType)
				return noderr.Wrap(err, body)
			}

			if action == nil {
				// unmarshalling a nil ast.Node causes a panic,
				// so avoid it by checking here.
				err := fmt.Errorf("action %s had no properties defined", actionType)
				return noderr.Wrap(err, body)
			}

			with, ok := mapNode["with"]
			if ok {
				// unmarshal the YAML onto the action
				dec := yaml.NewDecoder(&bytes.Buffer{})
				err = dec.DecodeFromNodeContext(ctx, with, action)
				if err != nil {
					return noderr.Wrap(err, body)
				}
			}

			e.Body = Action{Name: actionType, Action: action}
			return nil

		}
	}

	// try and parse as a Boolean
	//
	// e.g.
	// - and:  <- this node
	//     - A
	//     - B

	var m map[string][]ast.Node
	err = yaml.Unmarshal(b, &m)
	if err != nil {
		// if it doesn't decode here, return an error
		return noderr.Wrap(err, e.Node)
	}

	// the value looks like this:
	// - foo:
	//    - B
	//    - C
	// 'foo' might be 'and', or 'or'

	var op string
	if _, ok := m["and"]; ok {
		op = "and"
		e.Body = Boolean{Op: And}
	}
	if _, ok := m["or"]; ok {
		if op != "" {
			return errors.New("entry cannot have both 'and' and 'or' together")
		}
		e.Body = Boolean{Op: Or}
		op = "or"
	}
	if op == "" {
		return errors.New("entry must be either 'and' or 'or'")
	}

	for _, child := range m[op] {
		e.setNodePath(child)
		childEntry := Step{Node: child, Pass: e.Pass}

		// set up a new decoder. Usually we'd provide the bytes to be
		// read in the buffer, but because we're only using
		// DecodeFromNodeContext (which doesn't need the buffer)
		// it can be empty.
		dec := yaml.NewDecoder(&bytes.Buffer{})

		err = dec.DecodeFromNodeContext(ctx, child, &childEntry)
		if err != nil {
			return err
		}
		e.Children = append(e.Children, childEntry)
	}

	return nil
}

// parseNodeRef parses a fixed node reference from a Glide workflow statement.
// the value looks like this:
//   - start: B
//
// or like this:
//   - outcome: B
func (e *Step) parseNodeRef(body ast.Node, d dialect.Dialect, nodeType node.Type) error {
	// it's an Outcome node
	var expr string
	err := yaml.NodeToValue(body, &expr)
	if err != nil {
		return err
	}

	n := node.Node{ID: expr, Type: nodeType}

	// look up the node value from our dialect to see if it's
	// defined as a start or an end.

	// d.Nodes looks like the following:
	//
	//	Nodes: map[string]node.Node{
	// 	"request":  {Type: node.Start, Name: "Request"},
	// 	"approved": {Type: node.End, Priority: 1, Name: "Approved"},
	// }
	//
	// 'expr' might be "request" or "approved"
	// we need to look up the corresponding node value.

	if def, ok := d.Nodes[expr]; ok {
		if n.Type != nodeType {
			return fmt.Errorf("%s can only be used as a %s step", expr, nodeType)
		}

		n = def // set the node to be the value from the map, e.g. {Type: node.Start, Name: "Request"}

		// ensure that the ID is set to the value we looked up from the map.
		// this avoids an edge case where the node ID isn't set in the node object,
		// as is the case in the example map above (which doesn't have an ID field in the nodes).
		n.ID = expr

		// the statement name is always the node's name.
		//
		// This avoids a situation like follows:
		//
		// workflow:
		//   path_a:
		//     - name: A
		//       start: request
		//
		//   path_b:
		//     - name: B
		//       start: request
		//
		// where the start node is named ambiguously
		// in two different paths.
		//
		// to avoid this, we use the name of the node
		// as specified in the Glide dialect.
		e.Name = def.Name
	}

	e.Body = Ref{Node: n}
	return nil
}

// setNodePath hacks the YAMLPath of nested nodes to be the full path,
// rather than a partial path.
//
// Due to the way that we are (ab)using the UnmarshalYAML with lots of custom business
// logic to parse the syntax, it seems the the YAML library gives partial
// references to YAML nodes rather than a full reference.
//
// For example, errors come through as $.steps[0] rather than $.workflow.default.steps[0].
//
// This causes issues with display lint errors correctly, as we do not know the range
// to be highlighting.
// setNodePath fixes this in a very hacky way by manipulating the YAMLPath of these nodes.
//
// If there are problems with this method, you can add new tests to errors_test.go
// in the main package to verify that errors are coming through as expected.
func (e Step) setNodePath(n ast.Node) {
	if n != nil {
		existing := n.GetPath()                // "$.and[0].check"
		toReplace := e.Node.GetPath()          // "$.workflow.default.steps[0].and"
		parts := strings.Split(toReplace, ".") // [$, workflow, default, steps[0], and]
		parts = parts[:len(parts)-1]           // [$, workflow, default, steps[0]
		joined := strings.Join(parts, ".")

		// fullPath := strings.Replace(n.GetPath(), "$", "$.workflow."+e.Pass, 1)
		fullPath := strings.Replace(existing, "$", joined, 1)
		n.SetPath(fullPath)
	}
}

func (e Step) Debug() string {
	return fmt.Sprintf("[%s] %s", Hash(e), e.Body.String())
}

func (e Step) Hash() string {
	return Hash(e)
}

var Hash = func(s Step) string {
	// ref nodes always have a fixed hash, regardless of their position in the statements.
	// This allows us to combine ref nodes across multiple passes together into a single graph.
	if n, ok := s.Body.(Ref); ok {
		return n.Node.ID
	}

	// not very efficient - there is most likely a better approach for this!
	var posString []string
	for _, p := range s.Position {
		posString = append(posString, strconv.Itoa(p))
	}
	return s.Pass + "." + strings.Join(posString, ".")
}

// Operation are boolean operations
// to combine workflow steps.
// They are either AND or OR.
type Operation int

const (
	And Operation = iota
	Or
)

type Boolean struct {
	// Op is the operation ('and' or 'or')
	Op Operation
}

func (b Boolean) Type() StepType {
	return BooleanType
}

func (b Boolean) String() string {
	if b.Op == And {
		return "AND"
	} else {
		return "OR"
	}
}

type Check struct {
	Expression string
}

func (b Check) Type() StepType {
	return CheckType
}

func (b Check) String() string {
	expr := strings.ReplaceAll(b.Expression, `"`, `\"`)
	return fmt.Sprintf("if: %s", expr)
}

type Ref struct {
	Node node.Node
}

func (b Ref) Type() StepType {
	return RefType
}

func (b Ref) String() string {
	return fmt.Sprintf("%s: %s", b.Node.Type, b.Node.ID)
}

type Action struct {
	Name   string
	Action any
}

func (b Action) Type() StepType {
	return ActionType
}

func (b Action) String() string {
	// return the string representation of the underlying action if it exists
	if s, ok := b.Action.(fmt.Stringer); ok {
		return s.String()
	}

	return fmt.Sprintf("action: %s", b.Name)
}

func (b Action) PrintAction() string {
	// return the PrintAction representation of the underlying action if it exists
	if s, ok := b.Action.(PrintActioner); ok {
		return s.PrintAction()
	}
	// fall back to the String() representation
	return b.String()
}

// PrintActioner can print information about what the action
// will do.
//
// This is implemented as a separate interface to fmt.Stringer
// so that we can control when this is shown to users.
type PrintActioner interface {
	PrintAction() string
}
