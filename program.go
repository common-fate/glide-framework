package glide

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/step"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/pkg/errors"
)

// Program is a Glide workflow definition.
type Program struct {
	Workflow map[string]Path
}

func (p *Program) UnmarshalYAML(ctx context.Context, b []byte) error {
	// validate the dialect
	_, ok := dialect.FromContext(ctx)
	if !ok {
		return errors.New("glide dialect must be defined in context using glide.Use()")
	}

	if p.Workflow == nil {
		p.Workflow = map[string]Path{}
	}

	var tmp struct {
		Workflow map[string]ast.Node `yaml:"workflow"`
	}

	err := yaml.Unmarshal(b, &tmp)
	if err != nil {
		return err
	}

	for id, node := range tmp.Workflow {
		if node == nil {
			continue
		}

		pass := Path{id: id}

		// set up a new decoder. Usually we'd provide the bytes to be
		// read in the buffer, but because we're only using
		// DecodeFromNodeContext (which doesn't need the buffer)
		// it can be empty.
		dec := yaml.NewDecoder(&bytes.Buffer{})

		err = dec.DecodeFromNodeContext(ctx, node, &pass)
		if err != nil {
			return err
		}

		p.Workflow[id] = pass
	}

	return nil
}

// Path is a group of statements.
// Each pass in a Glide program builds the workflow graph from
// Start nodes to End nodes.
type Path struct {
	id    string
	Steps []step.Step
	// Node  ast.Node
}

func (p *Path) UnmarshalYAML(ctx context.Context, b []byte) error {
	// validate the dialect
	d, ok := dialect.FromContext(ctx)
	if !ok {
		return errors.New("glide dialect must be defined in context using glide.Use()")
	}
	err := d.Validate()
	if err != nil {
		return err
	}

	// the YAML structure looks like this
	//
	// workflow:			<- program
	//   default:			<- path
	//    steps:
	//      - start: A		<- step
	//      - outcome: B	<- step
	//

	// parse the 'steps' field of the path.
	var nodeMap map[string]ast.Node
	err = yaml.Unmarshal(b, &nodeMap)
	if err != nil {
		return errors.Wrapf(err, "path %s must contain a 'steps' field", p.id)
	}

	node, ok := nodeMap["steps"]
	if !ok {
		return fmt.Errorf("path %s must contain a 'steps' field", p.id)
	}

	// a Path should contain an array of Steps
	var steps []ast.Node

	err = yaml.NodeToValue(node, &steps)
	if err != nil {
		return err
	}

	for _, n := range steps {
		fullPath := strings.Replace(n.GetPath(), "$", "$.workflow."+p.id, 1)
		n.SetPath(fullPath)

		s := step.Step{Pass: p.id, Node: n}

		// set up a new decoder. Usually we'd provide the bytes to be
		// read in the buffer, but because we're only using
		// DecodeFromNodeContext (which doesn't need the buffer)
		// it can be empty.
		dec := yaml.NewDecoder(&bytes.Buffer{})

		err = dec.DecodeFromNodeContext(ctx, n, &s)
		if err != nil {
			return err
		}

		p.Steps = append(p.Steps, s)
	}

	return nil
}

// SimpleProgram creates a program with one 'default' pass only.
func SimpleProgram(statements ...step.Step) *Program {
	p := NewProgram()
	p = p.Pass("default", statements...)
	return p
}

func NewProgram() *Program {
	return &Program{Workflow: map[string]Path{}}
}

// Pass adds a pass to the workflow. Used to build test Programs.
func (p *Program) Pass(name string, statements ...step.Step) *Program {
	pass := Path{id: name}

	for _, s := range statements {
		s = setPass(s, name)
		pass.Steps = append(pass.Steps, s)
	}

	p.Workflow[name] = pass
	return p
}

func setPass(s step.Step, pass string) step.Step {
	s.Pass = pass
	for i, child := range s.Children {
		s.Children[i] = setPass(child, pass)
	}
	return s
}
