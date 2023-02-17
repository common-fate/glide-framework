package glide

import (
	"context"
	"testing"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/node"
	"github.com/common-fate/glide/pkg/step"
	"github.com/common-fate/glide/pkg/step/s"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name string
		give string
		// only is a small helper flag allowing one test to be isolated
		// without needing to comment out tests (which breaks YAML indent)
		only    bool
		want    *Program
		dialect *dialect.Dialect
		wantErr bool
	}{
		{
			name: "ok",
			give: `
workflow:
  default:
    steps:
      - start: A
      - outcome: B
`,
			want: NewProgram().Pass("default",
				s.Start("A"),
				s.Outcome("B"),
			),
		},
		{
			name: "with nested elements",
			give: `
workflow:
  default:
    steps:
      - start: A
      - and:
        - check: B
        - check: C
      - outcome: D
`,
			want: NewProgram().Pass("default",
				s.Start("A"),
				s.Boolean(step.And,
					s.Check("B"),
					s.Check("C"),
				),
				s.Outcome("D"),
			),
		},
		{
			name: "with if statement",
			give: `
workflow:
  default:
    steps:
      - check: A
					`,
			want: NewProgram().Pass("default", s.Check("A")),
		},
		{
			name: "with actions",
			give: `
workflow:
  default:
    steps:
      - action: my_action
        with:
          property: hello
      - action: my_action
        with:
          property: something_else
`,
			want: NewProgram().Pass("default",
				s.Action("my_action", &testAction{Property: "hello"}),
				s.Action("my_action", &testAction{Property: "something_else"}),
			),
			dialect: &dialect.Dialect{
				Actions: func() map[string]any {
					return map[string]any{
						"my_action": &testAction{},
					}
				},
			},
		},
		{
			name: "with boolean and actions",
			give: `
workflow:
  default:
    steps:
      - or:
        - action: my_action
          with:
            property: hello
        - action: my_action
          with:
            property: something_else
`,
			want: NewProgram().Pass("default",
				s.Boolean(step.Or,
					s.Action("my_action", &testAction{Property: "hello"}),
					s.Action("my_action", &testAction{Property: "something_else"}),
				),
			),
			dialect: &dialect.Dialect{
				Actions: func() map[string]any {
					return map[string]any{
						"my_action": &testAction{},
					}
				},
			},
		},
		{
			name:    "with start node",
			dialect: dialect.New().Start("A"),
			give: `
workflow:
  default:
    steps:
      - start: A
`,
			want: NewProgram().Pass("default", s.Start("A")),
		},
		{
			name: "invalid dialect with multiple end nodes same priority",
			dialect: &dialect.Dialect{
				Nodes: map[string]node.Node{
					"end1": {Type: node.Outcome, Priority: 1},
					"end2": {Type: node.Outcome, Priority: 1},
				},
			},
			give: `
workflow:
  default:
    steps:
      - outcome: end1
`,
			wantErr: true,
		},
		{
			name: "invalid dialect with zero priority",
			dialect: &dialect.Dialect{
				Nodes: map[string]node.Node{
					"end1": {Type: node.Outcome, Priority: 0},
				},
			},
			give: `
workflow:
  default:
    steps:
      - outcome: end1
`,
			wantErr: true,
		},
		{
			name: "invalid no workflow",
			give: `
- something
`,
			wantErr: true,
		},
		{
			name: "invalid wrong workflow structure",
			give: `
workflow:
  - something
`,
			wantErr: true,
		},
		{
			name: "end node priority should be applied",
			dialect: &dialect.Dialect{
				Nodes: map[string]node.Node{
					"end1": {Type: node.Outcome, Priority: 100},
				},
			},
			give: `
workflow:
  default:
    steps:
      - outcome: end1
`,
			want: NewProgram().Pass("default",
				step.Step{
					Pass: "default",
					Body: step.Ref{
						Node: node.Node{
							Type:     node.Outcome,
							ID:       "end1",
							Priority: 100,
						},
					},
				},
			),
		},
		{
			name: "named steps",
			give: `
workflow:
  default:
    steps:
      - start: A

      - name: My check
        check: true

      - outcome: B
`,
			dialect: &dialect.Dialect{
				Nodes: map[string]node.Node{
					"A": {Type: node.Start, Name: "Start Node Name"},
					"B": {Type: node.Outcome, Name: "End Node Name", Priority: 1},
				},
			},
			want: NewProgram().Pass("default",
				s.Named("Start Node Name").Start("A"),
				s.Named("My check").Check("true"),
				s.Named("End Node Name").Priority(1).Outcome("B"),
			),
			only: true,
		},
	}

	var hasOnly bool
	for _, tt := range tests {
		if tt.only == true {
			hasOnly = true
			break
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if hasOnly && !tt.only {
				t.Skip("only flag is set")
			}
			if tt.dialect == nil {
				tt.dialect = dialect.New()
			}

			var got Program

			ctx := Use(context.Background(), *tt.dialect)

			err := yaml.UnmarshalContext(ctx, []byte(tt.give), &got)
			if (err != nil) != tt.wantErr {
				t.Errorf("compile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				statementsEqual(t, tt.want, &got)
			}
		})
	}
}

// statementsEqual compares statements without checking the AST node
// to make testing easier.
//
// The AST nodes are only used for pretty-printing errors.
func statementsEqual(t *testing.T, want *Program, got *Program) {
	cleanedWorkflow := &Program{
		Workflow: map[string]Path{},
	}

	for passID, pass := range got.Workflow {
		var cleaned []step.Step
		for _, s := range pass.Steps {
			// remove the YAML AST nodes
			s = cleanAst(s)

			cleaned = append(cleaned, s)
		}
		p := got.Workflow[passID]
		p.Steps = cleaned
		cleanedWorkflow.Workflow[passID] = p
	}

	assert.Equal(t, want, cleanedWorkflow)
}

func cleanAst(s step.Step) step.Step {
	s.Node = nil

	for i, child := range s.Children {
		s.Children[i] = cleanAst(child)
	}
	return s
}

func TestUnmarshalNoContext(t *testing.T) {
	// unmarshalling without calling glide.Use()
	// should return an error.
	ctx := context.Background()
	var got Program
	err := yaml.UnmarshalContext(ctx, []byte("workflow:"), &got)

	want := "glide dialect must be defined in context using glide.Use()"

	assert.EqualError(t, err, want)
}

// Check that the unmarshalling wrapper function
// glide.Unmarshal() works as expected.
func TestUnmarshalWrapper(t *testing.T) {
	type args struct {
		data    []byte
		dialect dialect.Dialect
	}
	tests := []struct {
		name    string
		args    args
		want    *Program
		wantErr bool
	}{
		{
			name: "ok",
			args: args{
				data: []byte(`
workflow:
  test:
    steps:
      - start: A
`),
			},
			want: NewProgram().Pass("test", s.Start("A")),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Unmarshal(tt.args.data, tt.args.dialect)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalA() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			statementsEqual(t, tt.want, got)
		})
	}
}
