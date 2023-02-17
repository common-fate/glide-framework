package glide

import (
	"github.com/common-fate/glide/pkg/step"
	"github.com/dominikbraun/graph"
	"github.com/google/cel-go/cel"
)

type Graph struct {
	// G is the underlying graph data structure.
	G graph.Graph[string, step.Step]

	// programs is a map of graph vertex hashes to compiled CEL programs.
	programs map[string]cel.Program
}

func NewGraph() *Graph {
	return &Graph{
		G:        graph.New(step.Hash, graph.Directed(), graph.PreventCycles()),
		programs: map[string]cel.Program{},
	}
}
