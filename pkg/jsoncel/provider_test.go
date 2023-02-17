package jsoncel

import (
	"testing"

	"github.com/google/cel-go/cel"
)

func TestProvider(t *testing.T) {
	p := NewProvider("input", &Schema{
		Properties: map[string]*Schema{
			"name": {
				Type: String,
			},
			"group": {
				Type: Object,
				Properties: map[string]*Schema{
					"id": {
						Type: String,
					},
				},
			},
		},
	})
	env, err := cel.NewEnv(
		cel.CustomTypeProvider(p),
		cel.Variable("input", cel.ObjectType("input")),
	)
	if err != nil {
		t.Fatal(err)
	}

	_, issues := env.Compile(`input.group.id == "world"`)
	if issues != nil && issues.Err() != nil {
		t.Fatal(issues.Err())
	}
}
