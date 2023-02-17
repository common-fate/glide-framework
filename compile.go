package glide

import (
	"fmt"

	"github.com/common-fate/glide/pkg/jsoncel"
	"github.com/common-fate/glide/pkg/node"
	"github.com/common-fate/glide/pkg/noderr"
	"github.com/common-fate/glide/pkg/step"
	"github.com/dominikbraun/graph"
	"github.com/google/cel-go/cel"
	"github.com/pkg/errors"
)

// DefaultMaxDepth is the default maximum depth
// that statements can be nested in Glide workflows.
// It can be customised by setting 'MaxDepth'
// on the Compiler.
const DefaultMaxDepth = 10

type Compiler struct {
	Program     *Program
	InputSchema *jsoncel.Schema
	// MaxDepth is set to 10 by default if not provided.
	MaxDepth int
}

// Compile statements into an execution graph.
func (c *Compiler) Compile() (*Graph, error) {
	// set a default MaxDepth if it isn't provided.
	if c.MaxDepth == 0 {
		c.MaxDepth = DefaultMaxDepth
	}

	// set up the type for the 'input' object,
	// based on the provided JSON schema.
	p := jsoncel.NewProvider("input", c.InputSchema)

	env, err := cel.NewEnv(
		cel.CustomTypeProvider(p),
		cel.Variable("input", cel.ObjectType("input")),
	)
	if err != nil {
		return nil, err
	}

	g := NewGraph()

	for passID, pd := range c.Program.Workflow {
		p := pd
		err = compilePass(compilePassOpts{
			G:          g,
			PassID:     passID,
			Env:        env,
			Statements: p.Steps,
			MaxDepth:   c.MaxDepth,
		})
		if err != nil {
			return nil, err
		}
	}

	return g, nil
}

type compilePassOpts struct {
	G *Graph
	// PassID is the ID of the current workflow pass
	//
	// e.g.
	// 	workflow:
	//	  default: <- PassID='default'
	//      - A
	//      - B
	PassID     string
	Env        *cel.Env
	Statements []step.Step
	MaxDepth   int
}

// compilePass compiles a particular pass over the workflow graph into.
func compilePass(opts compilePassOpts) error {

	// validate statement ordering.

	// a workflow must always contain at least 2 statements
	if len(opts.Statements) < 2 {
		return fmt.Errorf("workflow must contain at least 2 statements: got %d statements", len(opts.Statements))
	}

	// the first statement must always be a Start node.
	err := assertNode(opts.Statements[0], node.Start)
	if err != nil {
		return err
	}

	// the last statement must always be an End node.
	err = assertNode(opts.Statements[len(opts.Statements)-1], node.Outcome)
	if err != nil {
		return err
	}

	var prev *step.Step
	for i, sd := range opts.Statements {
		s := sd

		// visit each statement to build out the execution graph.
		err := visitStatement(&VisitOpts{
			Statement:     &s,
			G:             opts.G,
			Previous:      prev,
			Index:         i,
			Env:           opts.Env,
			MaxDepth:      opts.MaxDepth,
			NumStatements: len(opts.Statements),
		})
		if err != nil {
			return noderr.Wrap(err, s.Node)
		}

		prev = &s
	}

	return nil
}

// assertNode asserts that a particular statement
// contains a reference to a node, and that the
// node is a particular type.
func assertNode(s step.Step, wantType node.Type) error {
	r, ok := s.Body.(step.Ref)
	if !ok {
		return fmt.Errorf("statement %s must be a reference to a %s node, but wasn't a reference", s.Body, wantType)
	}
	if r.Node.Type != wantType {
		return fmt.Errorf("statement %s must be a reference to a %s node", s.Body, wantType)
	}
	return nil
}

type VisitOpts struct {
	Statement *step.Step
	G         *Graph
	Index     int
	// Depth starts at 0 and increases as
	// visitStatement() is recursively called
	// to visit child nodes.
	//
	// e.g.
	//	- and:	<- depth = 0
	//	  - A	<- depth = 1
	//	  - or:	<- depth = 1
	//		- B	<- depth = 2
	//		- C	<- depth = 2
	Depth int
	// MaxDepth is the depth which cannot be exceeded by the compiler.
	// Prevents users creating large nested resources to exhaust server resources.
	MaxDepth int
	Env      *cel.Env // the CEL env

	// NumStatements is the number of statements in the workflow.
	// When visiting this is used to assert that an End node MUST be at
	// the end of a workflow only.
	NumStatements int

	Parent   *step.Step
	Previous *step.Step
}

func visitStatement(opts *VisitOpts) error {
	// validate that MaxDepth hasn't been exceeded
	if opts.Depth > opts.MaxDepth {
		return fmt.Errorf("compiler max depth of %v was exceeded (depth=%v)", opts.MaxDepth, opts.Depth)
	}

	e := opts.Statement
	g := opts.G

	if opts.Parent != nil {
		e.Position = opts.Parent.Position
	}

	e.Position = append(e.Position, opts.Index)
	err := g.G.AddVertex(*e, graph.VertexAttribute("label", e.Debug()))

	// it's okay if we've already inserted the vertex on an earlier pass.
	// this logic might need to be changed if the hashing function changes for nodes,
	// because at the moment the pass ID is included as part of the hash.
	// This ensures that we only have collisions for ref nodes.
	if err != nil && err != graph.ErrVertexAlreadyExists {
		return err
	}

	key := opts.Statement.Hash()

	// if there is a parent, link the current node to it
	if opts.Parent != nil {
		err = g.G.AddEdge(key, opts.Parent.Hash())
		if err != nil {
			return err
		}
	}

	// if there are no children and we have a node from the previous statement,
	// link the previous statement node to the entry
	if len(e.Children) == 0 && opts.Previous != nil {
		err = g.G.AddEdge(opts.Previous.Hash(), key)
		if err != nil {
			return errors.Wrapf(err, "adding edge to previous node %s", key)
		}
	}

	// node-specific compilation steps
	switch t := e.Body.(type) {
	case step.Check:
		ast, issues := opts.Env.Compile(t.Expression)
		if issues != nil && issues.Err() != nil {
			return fmt.Errorf("CEL type-check error: %s", issues.Err())
		}
		if ast.OutputType() != cel.BoolType {
			return fmt.Errorf("CEL expression must return a boolean (returned %s instead)", ast.OutputType())
		}

		prg, err := opts.Env.Program(ast)
		if err != nil {
			return fmt.Errorf("CEL program construction error: %s", err)
		}
		g.programs[key] = prg
	case step.Ref:
		// unknown refs cannot be compiled - a node reference must be to a start or an end node.
		if t.Node.Type == node.Unknown {
			return fmt.Errorf("invalid node %s: did not match any known start or end nodes", e.Body)
		}

		// if it's a Start, it MUST be at index=0 and depth=0
		if t.Node.Type == node.Start {
			if opts.Index != 0 {
				return fmt.Errorf("invalid node %s: start nodes can only be referenced at the beginning of a workflow: start node had index %v but need index %v", e.Body, opts.Index, 0)
			}

			if opts.Depth != 0 {
				return fmt.Errorf("invalid node %s: start nodes can only be referenced at the beginning of a workflow: start node had depth %v but need depth %v", e.Body, opts.Depth, 0)
			}
		}

		// if it's an End, it MUST be the last statement and depth=0
		if t.Node.Type == node.Outcome {
			if opts.Index != opts.NumStatements-1 {
				return fmt.Errorf("invalid node %s: end nodes can only be referenced at the end of a workflow: end node had index %v but need index %v", e.Body, opts.Index, opts.NumStatements-1)
			}

			if opts.Depth != 0 {
				return fmt.Errorf("invalid node %s: end nodes can only be referenced at the end of a workflow: end node had depth %v but need depth %v", e.Body, opts.Depth, 0)
			}
		}
	}

	for i, child := range e.Children {
		err = visitStatement(&VisitOpts{
			Statement:     &child,
			G:             g,
			Index:         i,
			Parent:        e,
			Previous:      opts.Previous,
			Env:           opts.Env,
			Depth:         opts.Depth + 1,
			MaxDepth:      opts.MaxDepth,
			NumStatements: opts.NumStatements,
		})
		if err != nil {
			return noderr.Wrap(err, child.Node)
		}
	}

	return nil
}
