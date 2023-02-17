// Package noderr contains an error definition
// for compile and lint errors.
package noderr

import (
	"errors"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
)

type NodeError struct {
	Node ast.Node
	Err  error
}

// PrettyPrint the error along with the YAML node.
func (ne NodeError) PrettyPrint(yml []byte) (string, error) {
	path, err := yaml.PathString(ne.Node.GetPath())
	if err != nil {
		return "", err
	}
	source, err := path.AnnotateSource([]byte(yml), true)
	if err != nil {
		return "", err
	}
	return string(source), nil
}

func (ne NodeError) Error() string {
	return ne.Err.Error()
}

func Wrap(err error, node ast.Node) error {
	var ne NodeError
	if errors.Is(err, &ne) {
		// the error was already wrapped in a child node.
		return err
	}
	return NodeError{Err: err, Node: node}
}
