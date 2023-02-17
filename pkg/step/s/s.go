// package 's' contains helper methods for building steps.
// It is used as a convenience method when writing tests for
// the Glide compiler.
package s

import (
	"github.com/common-fate/glide/pkg/node"
	"github.com/common-fate/glide/pkg/step"
)

// NewRef creates a new node reference to an unknown node.
// Unknown nodes result in a compile error are are just used to
// help test statement parsing.
func Ref(name string) step.Step {
	return step.Step{Body: step.Ref{Node: node.Node{Type: node.Unknown, ID: name}}}
}

// Start creates a new node reference to a Start node.
func Start(name string) step.Step {
	return step.Step{Body: step.Ref{Node: node.Node{Type: node.Start, ID: name}}}
}

// Outcome creates a new node reference to an End node.
func Outcome(name string) step.Step {
	return step.Step{Body: step.Ref{Node: node.Node{Type: node.Outcome, ID: name}}}
}

func Boolean(op step.Operation, children ...step.Step) step.Step {
	return step.Step{Body: step.Boolean{Op: op}, Children: children}
}

func Check(expression string) step.Step {
	return step.Step{Body: step.Check{Expression: expression}}
}

func Action(name string, action any) step.Step {
	return step.Step{Body: step.Action{Name: name, Action: action}}
}

type StepBuilder struct {
	Name         string
	NodePriority int
}

// Named returns a step with a set name.
//
// Usage:
//
//	s.Named("my-name").Check("<expression>")
func Named(name string) *StepBuilder {
	return &StepBuilder{Name: name}
}

// Priority of the step.
// This is only applied to Outcome steps.
func (sb *StepBuilder) Priority(priority int) *StepBuilder {
	sb.NodePriority = priority
	return sb
}

// NewRef creates a new node reference to an unknown node.
// Unknown nodes result in a compile error are are just used to
// help test statement parsing.
func (sb StepBuilder) Ref(id string) step.Step {
	return step.Step{Name: sb.Name, Body: step.Ref{Node: node.Node{Type: node.Unknown, ID: id, Name: sb.Name}}}
}

// Start creates a new node reference to a Start node.
func (sb StepBuilder) Start(id string) step.Step {
	return step.Step{Name: sb.Name, Body: step.Ref{Node: node.Node{Type: node.Start, ID: id, Name: sb.Name}}}
}

// Outcome creates a new node reference to an End node.
func (sb StepBuilder) Outcome(id string) step.Step {
	return step.Step{Name: sb.Name, Body: step.Ref{Node: node.Node{Type: node.Outcome, ID: id, Priority: sb.NodePriority, Name: sb.Name}}}
}

func (sb StepBuilder) Boolean(op step.Operation, children ...step.Step) step.Step {
	return step.Step{Body: step.Boolean{Op: op}, Children: children}
}

func (sb StepBuilder) Check(expression string) step.Step {
	return step.Step{Name: sb.Name, Body: step.Check{Expression: expression}}
}

func (sb StepBuilder) Action(name string, action any) step.Step {
	return step.Step{Name: sb.Name, Body: step.Action{Name: name, Action: action}}
}
