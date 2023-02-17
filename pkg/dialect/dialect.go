// Package dialect contain definitions for Glide dialects.
// A dialect provides the allowed actions, starts, and outcomes
// for a workflow.
package dialect

import (
	"context"
	"fmt"

	"github.com/common-fate/glide/pkg/node"
)

type contextKey int

const (
	dialectKey contextKey = iota
)

// Dialect configures the workflow language
// with the allowed start and end nodes
// and the allowed action types.
type Dialect struct {
	// Nodes are predefined nodes which can
	// belong in a workflow for the start and end.
	Nodes   map[string]node.Node
	Actions func() map[string]any
}

// Context returns a copy of the parent context,
// with the Glide dialect defined.
func Context(parent context.Context, d Dialect) context.Context {
	return context.WithValue(parent, dialectKey, d)
}

// FromContext loads the Glide dialect from context.
// It returns false if the dialect does not exist in the context.
func FromContext(ctx context.Context) (Dialect, bool) {
	d, ok := ctx.Value(dialectKey).(Dialect)
	return d, ok
}

// New creates a new empty dialect.
func New() *Dialect {
	return &Dialect{
		Nodes: map[string]node.Node{},
	}
}

func (d *Dialect) Validate() error {
	// each end node must have a unique priority
	priorityMap := map[int]bool{}

	for _, n := range d.Nodes {
		if n.Type == node.Outcome {
			if n.Priority <= 0 {
				return fmt.Errorf("dialect error: all end nodes must have a priority greater than 0: found node with priority %v", n.Priority)
			}

			_, ok := priorityMap[n.Priority]
			if ok {
				return fmt.Errorf("dialect error: each end node must have a unique priority: found two nodes with priority %v", n.Priority)
			}
			priorityMap[n.Priority] = true
		}
	}
	// all good if we get here
	return nil
}

// Start sets start nodes with names.
func (d *Dialect) Start(names ...string) *Dialect {
	for i, n := range names {
		d.Nodes[n] = node.Node{Type: node.Start, Priority: i}
	}
	return d
}

// End sets end nodes with names.
// Nodes are ordered in priority with later names having higher priority.
func (d *Dialect) End(names ...string) *Dialect {
	for i, n := range names {
		d.Nodes[n] = node.Node{Type: node.Outcome, Priority: i}
	}
	return d
}
