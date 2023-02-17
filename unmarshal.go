package glide

import (
	"context"

	"github.com/common-fate/glide/pkg/dialect"
	"github.com/goccy/go-yaml"
)

// Unmarshal a glide workflow YAML file into a program which can be compiled.
func Unmarshal(data []byte, dialect dialect.Dialect) (*Program, error) {
	var p Program
	ctx := context.Background()
	ctx = Use(ctx, dialect)

	err := yaml.UnmarshalContext(ctx, data, &p)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
