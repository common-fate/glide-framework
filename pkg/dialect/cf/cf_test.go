package cf

import (
	"encoding/json"
	"testing"
)

func TestApproval_Complete(t *testing.T) {
	type fields struct {
		Groups []string
	}
	tests := []struct {
		name    string
		fields  fields
		input   string
		want    bool
		wantErr bool
	}{
		{
			name: "ok",
			fields: fields{
				Groups: []string{"admins"},
			},
			input: `
{
	"approvals": [
		{
			"groups": ["admins"]
		}
	]
}
`,
			want: true,
		},
		{
			name: "not complete",
			fields: fields{
				Groups: []string{"other"},
			},
			input: `
{
	"approvals": [
		{
			"groups": ["admins"]
		}
	]
}
`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input map[string]any

			err := json.Unmarshal([]byte(tt.input), &input)
			if err != nil {
				t.Fatal(err)
			}

			a := &Approval{
				Groups: tt.fields.Groups,
			}
			got, err := a.Complete(input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Approval.Complete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Approval.Complete() = %v, want %v", got, tt.want)
			}
		})
	}
}
