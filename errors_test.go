package glide

import (
	"context"
	"testing"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/noderr"
	"github.com/goccy/go-yaml"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestUnmarshal_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		give        string
		wantErrPath string
		wantErr     string
	}{
		{
			name: "ok",
			give: `
workflow:
  default:
    steps:
      - helloworld
		`,
			wantErrPath: "$.workflow.default.steps[0]",
			wantErr:     "[1:1] string was used where mapping is expected\n>  1 | helloworld\n       ^\n",
		},
		{
			name: "unknown action",
			give: `
workflow:
  default:
    steps:
      - action: hi
		`,
			wantErrPath: "$.workflow.default.steps[0].action",
			wantErr:     "no actions are defined for this Glide dialect",
		},
		{
			name: "nested in boolean",
			give: `
workflow:
  default:
    steps:
      - and:
          - check: true
          - action: hi
`,
			wantErrPath: "$.workflow.default.steps[0].and[1].action",
			wantErr:     "no actions are defined for this Glide dialect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			d := dialect.New()
			var got Program

			ctx := Use(context.Background(), *d)

			err := yaml.UnmarshalContext(ctx, []byte(tt.give), &got)
			var ne noderr.NodeError
			if errors.As(err, &ne) {
				assert.Equal(t, tt.wantErrPath, ne.Node.GetPath())
			} else {
				t.Fatal("error was not noderr.NodeError")
			}
			assert.EqualError(t, err, tt.wantErr)
		})
	}
}
