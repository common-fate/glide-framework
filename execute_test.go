package glide

import (
	"testing"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/jsoncel"
	"github.com/common-fate/glide/pkg/step"
	"github.com/common-fate/glide/pkg/step/s"
	"github.com/stretchr/testify/assert"
)

func TestExecute(t *testing.T) {
	tests := []struct {
		name      string
		compiler  Compiler
		start     string
		input     map[string]any
		dialect   dialect.Dialect
		wantState map[string]State
		wantErr   bool
	}{
		{
			name:  "ok",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Boolean(step.And,
						s.Check("false"),
						s.Check("false"),
					),
					s.Outcome("approved"),
				),
			},
			dialect: testDialect,
			wantState: map[string]State{
				"request":     Complete,
				"default.1":   Inactive,
				"default.1.0": Inactive,
				"default.1.1": Inactive,
				"approved":    Inactive,
			},
		},
		{
			name:  "with if evaluation",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Boolean(step.And,
						s.Check("true"),
						s.Check("false"),
					),
					s.Outcome("approved"),
				),
			},
			dialect: testDialect,
			wantState: map[string]State{
				"request":     Complete,
				"default.1":   Inactive,
				"default.1.0": Complete,
				"default.1.1": Inactive,
				"approved":    Inactive,
			},
		},
		{
			name:  "boolean step.And",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Boolean(step.And,
						s.Check("true"),
						s.Check("true"),
					),
					s.Outcome("approved"),
				),
			},
			dialect: testDialect,
			wantState: map[string]State{
				"request":     Complete,
				"default.1":   Complete,
				"default.1.0": Complete,
				"default.1.1": Complete,
				"approved":    Complete,
			},
		},
		{
			name:  "boolean OR",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Boolean(step.Or,
						s.Check("true"),
						s.Check("false"),
					),
					s.Outcome("approved"),
				),
			},
			dialect: testDialect,
			wantState: map[string]State{
				"request":     Complete,
				"default.1":   Complete,
				"default.1.0": Complete,
				"default.1.1": Inactive,
				"approved":    Complete,
			},
		},
		{
			name:  "with CEL evaluation",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Check(`input.group == "test"`),
					s.Outcome("approved"),
				),
				InputSchema: &jsoncel.Schema{
					Properties: map[string]*jsoncel.Schema{
						"group": {
							Type: jsoncel.String,
						},
					},
				},
			},
			dialect: testDialect,
			input: map[string]any{
				"group": "test",
			},
			wantState: map[string]State{
				"request":   Complete,
				"default.1": Complete,
				"approved":  Complete,
			},
		},
		{
			name:  "with CEL evaluation on an object",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Check(`input.group != null`), // input.group itself is an object, not just a string
					s.Outcome("approved"),
				),
				InputSchema: &jsoncel.Schema{
					Type: jsoncel.Object,
					Properties: map[string]*jsoncel.Schema{
						"group": {
							Type: jsoncel.Object,
							Properties: map[string]*jsoncel.Schema{
								"id": {
									Type: jsoncel.String,
								},
							},
						},
					},
				},
			},
			dialect: testDialect,
			input: map[string]any{
				"group": map[string]any{
					"id": "test",
				},
			},
			wantState: map[string]State{
				"request":   Complete,
				"default.1": Complete,
				"approved":  Complete,
			},
		},
		{
			name:  "with action completion",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Action("my_action", &testAction{complete: true}),
					s.Outcome("approved"),
				),
			},
			dialect: testDialect,
			input: map[string]any{
				"group": map[string]any{
					"id": "test",
				},
			},
			wantState: map[string]State{
				"request":   Complete,
				"default.1": Complete,
				"approved":  Complete,
			},
		},
		{
			name:  "with multiple passes",
			start: "request",
			compiler: Compiler{
				Program: NewProgram().
					Pass("first", s.Start("request"), s.Check("true"), s.Outcome("approved")).
					Pass("second", s.Start("request"), s.Check("false"), s.Outcome("approved")),
			},
			dialect: testDialect,
			wantState: map[string]State{
				"request":  Complete,
				"first.1":  Complete,
				"second.1": Inactive,
				"approved": Complete,
			},
		},
		{
			name:  "with multiple actions in a row",
			start: "request",
			compiler: Compiler{
				Program: SimpleProgram(
					s.Start("request"),
					s.Action("my_action", &testAction{complete: false}),
					// this action should not be complete, as the
					// predecessor node wasn't complete.
					s.Action("my_action", &testAction{complete: true}),
					s.Outcome("approved"),
				),
			},
			dialect: testDialect,
			wantState: map[string]State{
				"request":   Complete,
				"default.1": Active,
				"default.2": Inactive,
				"approved":  Inactive,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := tt.compiler.Compile()
			if err != nil {
				t.Fatal(err)
			}

			got, err := g.Execute(tt.start, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}

			if got == nil {
				got = &Result{}
			}

			assert.Equal(t, tt.wantState, got.State)
		})
	}
}
