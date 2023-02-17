// Package cf contains the Common Fate dialect of Glide.
// This dialect is for access request approval workflows.
package cf

import (
	"fmt"
	"strings"

	"github.com/mitchellh/mapstructure"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/common-fate/glide/pkg/node"
)

var Dialect = dialect.Dialect{
	Actions: actions,
	Nodes: map[string]node.Node{
		"request":  {Type: node.Start, Name: "Request"},
		"approved": {Type: node.Outcome, Priority: 1, Name: "Approved"},
	},
}

func actions() map[string]any {
	return map[string]any{
		"approval": &Approval{},
	}
}

type Approval struct {
	Groups []string `yaml:"groups"`
}

type Input struct {
	Approvals []ApprovalInput `mapstructure:"approvals"`
}

type ApprovalInput struct {
	User   string   `mapstructure:"user"`
	Groups []string `mapstructure:"groups"`
}

// Complete returns true if an Approval step in a workflow is complete.
func (a *Approval) Complete(input any) (bool, error) {
	var i Input
	err := mapstructure.Decode(input, &i)
	if err != nil {
		return false, err
	}

	for _, approval := range i.Approvals {
		for _, g := range approval.Groups {
			for _, requiredGroups := range a.Groups {
				if g == requiredGroups {
					// someone from a required group has approved
					return true, nil
				}
			}
		}
	}

	// not complete yet
	return false, nil
}

func (a *Approval) PrintAction() string {
	groups := strings.Join(a.Groups, ", ")
	return fmt.Sprintf("notifying %s for access approval", groups)
}
