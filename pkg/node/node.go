package node

type Type int

const (
	// Unknown is used only in testing and will result in a compile error.
	Unknown Type = iota
	Start
	Outcome
)

func (t Type) String() string {
	if t == Start {
		return "start"
	}
	return "outcome"
}

type Node struct {
	Type Type

	// ID is a unique string identifier for the node.
	// e.g. "request"
	ID string

	// Name is a friendly display name for the node.
	// e.g. "Request"
	Name string
	// Priority of the node.
	// Used in workflow execution to determine
	// the final workflow outcome if two end nodes
	// are reached at the same time.
	// The node with the higher priority is the overall
	// worfklow outcome.
	// Each end node must have a unique priority.
	Priority int
}
