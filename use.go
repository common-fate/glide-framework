package glide

import (
	"context"

	"github.com/common-fate/glide/pkg/dialect"
)

// Use a specified Glide dialect.
func Use(parent context.Context, d dialect.Dialect) context.Context {
	return dialect.Context(parent, d)
}
