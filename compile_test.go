package glide

import (
	"fmt"
	"sort"
	"testing"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/jsoncel"
	"github.com/common-fate/glide/pkg/step"
	"github.com/common-fate/glide/pkg/step/s"
	"github.com/dominikbraun/graph"
	"github.com/stretchr/testify/assert"
)

// To visually show the links between nodes in the graph,
// the output of the test follows the pattern:
//
//	[FROM_ID] <from_node> -> [TO_ID] <to_node>
//
// Where FROM_ID is the ID of the from node, TO_ID is the ID of the
// node being linked to,
// and <from_node> and <to_node> are string descriptions of each node.
//
// This helps us verify that the DAG is being built as we expect for
// particular sets of statements.
func Test_Compile(t *testing.T) {
	tests := []struct {
		name    string
		give    Compiler
		dialect *dialect.Dialect
		want    []string
		wantErr bool
	}{
		{
			name: "ok",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Outcome("B"),
				),
			},
			want: []string{
				"[A] start: A -> [B] outcome: B",
			},
		},
		{
			name: "three nodes",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Action("B", nil),
					s.Outcome("C"),
				),
			},
			want: []string{
				"[A] start: A -> [default.1] action: B",
				"[default.1] action: B -> [C] outcome: C",
			},
		},
		{
			name: "nested nodes",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"), // 0
					s.Boolean(step.And, // 1 AND
						s.Check("true"),  // 1.0
						s.Check("false"), // 1.1
					),
					s.Outcome("D"), // 2
				),
			},
			want: []string{
				"[A] start: A -> [default.1.0] if: true",
				"[A] start: A -> [default.1.1] if: false",
				"[default.1.0] if: true -> [default.1] AND",
				"[default.1.1] if: false -> [default.1] AND",
				"[default.1] AND -> [D] outcome: D",
			},
		},
		{
			name: "if statements",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Check(`input.name == "test"`),
					s.Outcome("B"),
				),
				InputSchema: &jsoncel.Schema{
					Properties: map[string]*jsoncel.Schema{
						"name": {
							Type: jsoncel.String,
						},
					},
				},
			},
			want: []string{
				`[A] start: A -> [default.1] if: input.name == \"test\"`,
				`[default.1] if: input.name == \"test\" -> [B] outcome: B`,
			},
		},
		{
			name: "invalid CEL in if statement",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Check(`aaaa`),
					s.Outcome("B"),
				),
			},
			wantErr: true,
		},
		{
			name: "CEL variable not found",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Check(`something == false`),
					s.Outcome("B"),
				),
			},
			wantErr: true,
		},
		{
			name: "CEL not return boolean",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Check(`input`),
					s.Outcome("B"),
				),
			},
			wantErr: true,
		},
		{
			name: "with action node",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Action("approval", nil),
					s.Outcome("C"),
				),
			},
			dialect: &dialect.Dialect{
				Actions: func() map[string]any {
					return map[string]any{"approval": nil}
				},
			},
			want: []string{
				"[A] start: A -> [default.1] action: approval",
				"[default.1] action: approval -> [C] outcome: C",
			},
		},
		{
			name: "invalid statement count",
			give: Compiler{
				// must have > 1 statement
				Program: SimpleProgram(
					s.Start("A"),
				),
			},
			wantErr: true,
		},
		{
			name: "invalid max depth exceeded",
			give: Compiler{
				MaxDepth: 1,
				Program: SimpleProgram(
					s.Start("A"),
					s.Boolean(step.And, s.Boolean(step.And, s.Boolean(step.And, s.Check("true")))),
					s.Outcome("D"),
				),
			},
			wantErr: true,
		},
		{
			name: "invalid end before start",
			give: Compiler{
				// must have > 1 statement
				Program: SimpleProgram(
					s.Outcome("B"),
					s.Start("A"),
				),
			},
			wantErr: true,
		},
		{
			name: "invalid unknown node",
			give: Compiler{
				Program: SimpleProgram(
					s.Start("A"),
					s.Ref("B"),
					s.Outcome("C"),
				),
			},
			wantErr: true,
		},
		{
			name: "ok with multiple passes",
			give: Compiler{
				Program: NewProgram().Pass("first",
					s.Start("A"),
					s.Check("true"),
					s.Outcome("B"),
				).Pass("second",
					s.Start("A"),
					s.Check("false"),
					s.Outcome("B"),
				),
			},
			want: []string{
				"[A] start: A -> [first.1] if: true",
				"[A] start: A -> [second.1] if: false",
				"[first.1] if: true -> [B] outcome: B",
				"[second.1] if: false -> [B] outcome: B",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.give.Compile()
			if (err != nil) != tt.wantErr {
				t.Errorf("compile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			var result []string
			if got != nil {
				result = printAdjacencyMap(t, got.G)
			}

			assert.Equal(t, tt.want, result)
		})
	}
}

// printAdjacencyMap prints a string representation of the adjancency map.
//
// To visually show the links between nodes in the graph,
// the output of the test follows the pattern:
//
//	[FROM_ID] <from_node> -> [TO_ID] <to_node>
//
// Where FROM_ID is the ID of the from node, TO_ID is the ID of the
// node being linked to,
// and <from_node> and <to_node> are string descriptions of each node.
func printAdjacencyMap(t *testing.T, g graph.Graph[string, step.Step]) []string {
	adj, err := g.AdjacencyMap()
	if err != nil {
		t.Fatal(err)
	}

	var result []string

	// turn the adjancency map into a string representation
	// so it's easier to see if things are not what we expect.
	for _, v := range adj {
		for _, aa := range v {
			source, err := g.Vertex(aa.Source)
			if err != nil {
				t.Fatal(err)
			}
			target, err := g.Vertex(aa.Target)
			if err != nil {
				t.Fatal(err)
			}

			result = append(result, fmt.Sprintf("%s -> %s", source.Debug(), target.Debug()))
		}
	}

	sort.Strings(result)
	return result
}
