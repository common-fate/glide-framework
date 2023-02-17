package glide

import (
	"fmt"

	"github.com/common-fate/glide/pkg/node"
	"github.com/common-fate/glide/pkg/step"
	"github.com/dominikbraun/graph"
	"github.com/pkg/errors"
)

type State int

const (
	Inactive State = iota
	Complete
	Active
)

func (s State) String() string {
	switch s {
	case Complete:
		return "complete"
	case Active:
		return "active"
	case Inactive:
		return "inactive"
	}
	return "unknown"
}

// Result of a workflow execution
type Result struct {
	// CG is the Completion Graph.
	// The Completion Graph contains the same vertices as the policy graph,
	// but only contains edges between nodes that are complete.
	CG graph.Graph[string, step.Step]

	// A map of vertex hashes to their corresponding state.
	State map[string]State

	// Outcome is the end state of the workflow.
	// If empty, the workflow is considered in an indeterminate, ongoing state.
	Outcome string
}

type Completer interface {
	Complete(input any) (bool, error)
}

// Execute a policy graph.
// The 'start' argument is the ID of a node to start execution from.
func (g *Graph) Execute(start string, input map[string]any) (*Result, error) {
	// build the input map for evaluating CEL expressions
	// this map contains dot separated keys,
	// such as 'input.group.id' -> 'test'
	inputMap := NewInputMap("input", input)

	// initialise the completion graph
	// this is a graph which contains the same vertices as our input graph,
	// but only has edges between nodes which are both Complete.
	//
	// e.g.
	// graph:
	// 	request >> if(on_call) >> if(in_admin_group) >> approved
	//
	// input: on_call=true, in_admin_group=false
	//
	// the completion graph would look like this:
	//
	// request [complete] >> if(on_call) [complete] . if(in_admin_group) . approved

	cg := graph.New(step.Hash, graph.Directed(), graph.PreventCycles())

	pres, err := g.G.PredecessorMap()
	if err != nil {
		return nil, err
	}

	// the provided 'start' argument must always be a Start node
	startVertex, err := g.G.Vertex(start)
	if err != nil {
		return nil, err
	}
	startNode, ok := startVertex.Body.(step.Ref)
	if !ok {
		return nil, fmt.Errorf("provided start %s was not a node reference", start)
	}
	if startNode.Node.Type != node.Start {
		return nil, fmt.Errorf("provided start %s was not a start node (got %s)", start, startNode.Node.Type.String())
	}

	// a map to track the state nodes
	state := map[string]State{}

	// outcome is set if there is a completed End node.
	var outcome node.Node

	var verr error // used to track errors occurred during visiting
	graph.BFS(g.G, start, func(k string) bool {
		// node is inactive by default
		state[k] = Inactive

		// start nodes are complete by default
		if k == start {
			state[k] = Complete
		}

		v, err := g.G.Vertex(k)
		if err != nil {
			verr = err
			return true // stop traversal
		}

		err = cg.AddVertex(v)
		if err != nil {
			verr = err
			return true // stop traversal
		}

		// create edges between the current node and all completed predecessors
		//
		// e.g.
		// request [complete] >> if(on_call) . if(in_admin_group) . approved
		//					  ↑		↑
		//	   create this edge	    current node
		predecessors := pres[k]

		// count the number of completed predecessors
		// so that if the node is a Boolean, we can determine
		// whether it should be complete.
		var completedCount int
		for _, edge := range predecessors {
			vstate, ok := state[edge.Source]
			if ok && vstate == Complete {
				completedCount++
				err = cg.AddEdge(edge.Source, k)
				if err != nil {
					verr = errors.Wrap(err, "adding edge to complete graph")
					return true // stop traversal
				}
			}
		}

		switch t := v.Body.(type) {
		case step.Check:
			if completedCount == 0 {
				// if no vertexes are completed before this one,
				// this vertex cannot be complete.
				return false // continue traversal
			}

			// get the CEL program
			prg, ok := g.programs[k]
			if !ok {
				verr = fmt.Errorf("could not find CEL program for %s", k)
				return true // stop traversal
			}

			val, _, err := prg.Eval(inputMap.Data)
			if err != nil {
				verr = err
				return true // stop traversal
			}

			valbool, ok := val.Value().(bool)
			if !ok {
				verr = fmt.Errorf("could not convert CEL to bool: %s", val)
				return true // stop traversal
			}

			if valbool {
				state[k] = Complete
			}

		case step.Boolean:
			// for the AND node to be complete, all previous nodes must be complete.
			if t.Op == step.And && completedCount == len(predecessors) {
				state[k] = Complete
			}

			// for the OR node to be complete, any previous node must be complete.
			if t.Op == step.Or && completedCount > 0 {
				state[k] = Complete
			}

		case step.Action:
			// if any predecessor is complete, the action is activated.
			// note that in regular graph constructions, actions should only have
			// a single predecessor anyway.
			if completedCount > 0 {
				state[k] = Active
			}

			// if the action supports it, evaluate it to determine
			// whether the workflow step is complete.
			// a step can only be complete if one of it's predecessors is complete,
			// so check that too with completedCount > 0
			if c, ok := t.Action.(Completer); ok && completedCount > 0 {
				complete, err := c.Complete(input)
				if err != nil {
					verr = err
					return true // stop traversal
				}
				if complete {
					state[k] = Complete
				}
			}
		case step.Ref:
			var isComplete bool
			isEndNode := t.Node.Type == node.Outcome

			// if any predecessor is complete, the output is complete.
			if completedCount > 0 {
				state[k] = Complete
				isComplete = true
			}

			// if it's an End node, set it as the outcome if it's higher priority
			if isComplete && isEndNode && outcome.Priority < t.Node.Priority {
				outcome = t.Node
			}
		}

		return false
	})

	if verr != nil {
		return nil, verr
	}

	res := Result{
		CG:      cg,
		State:   state,
		Outcome: outcome.ID,
	}

	return &res, nil
}

// InputMap is a map of flattened input keys to their corresponding values,
// e.g.
//
//	'group.id' -> 'test'
//
// Used as the input for CEL evaluation, because CEL requires keys
// to be dot separated.
type InputMap struct {
	// The flattened mapping - e.g. 'group.id' -> 'test'
	Data map[string]any
}

// NewInputMap creates a new input map. The 'key' field is
// the root name of the input - such as 'input'.
func NewInputMap(key string, data map[string]any) *InputMap {
	im := InputMap{}
	im.build(key, data)
	return &im
}

func (im *InputMap) build(key string, input map[string]any) {
	if im.Data == nil {
		im.Data = map[string]any{}
	}

	for k, v := range input {
		childKey := key + "." + k // 'group.id'
		im.Data[childKey] = v

		// if the value is also a map, call the Build method
		// again to register all child fields.
		if child, ok := v.(map[string]any); ok {
			im.build(childKey, child)
		}
	}
}
