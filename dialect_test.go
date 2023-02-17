package glide

import (
	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/node"
)

// testDialect is a Glide dialect used
// for internal tests.
//
// It contains 'request' and 'approved'
// nodes, and an action called 'my_action'
var testDialect = dialect.Dialect{
	Actions: func() map[string]any {
		return map[string]any{
			"my_action": &testAction{},
		}
	},
	Nodes: map[string]node.Node{
		"request":  {Type: node.Start},
		"approved": {Type: node.Outcome},
	},
}

type testAction struct {
	Property string `yaml:"property"`
	// complete will mark the action as complete
	// when the graph is evaluated.
	complete bool
}

func (t *testAction) Complete(input any) (bool, error) {
	return t.complete, nil
}
