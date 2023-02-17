package command

import (
	"encoding/json"
	"os"

	"github.com/common-fate/glide"
	"github.com/common-fate/glide/pkg/dialect/cf"
	"github.com/common-fate/glide/pkg/jsoncel"
	"github.com/dominikbraun/graph/draw"
	"github.com/urfave/cli/v2"
)

var Compile = cli.Command{
	Name: "compile",
	Flags: []cli.Flag{
		&cli.PathFlag{Name: "file", Aliases: []string{"f"}, Usage: "the workflow file to compile", Required: true},
		&cli.PathFlag{Name: "schema", Aliases: []string{"s"}, Usage: "the input schema, in JSON schema format", Required: true},
	},
	Action: func(c *cli.Context) error {
		f := c.Path("file")
		schemaFile := c.Path("schema")

		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		prog, err := glide.Unmarshal(data, cf.Dialect)
		if err != nil {
			return err
		}

		schemaBytes, err := os.ReadFile(schemaFile)
		if err != nil {
			return err
		}

		var schema jsoncel.Schema
		err = json.Unmarshal(schemaBytes, &schema)
		if err != nil {
			return err
		}

		compiler := glide.Compiler{
			Program:     prog,
			InputSchema: &schema,
		}

		g, err := compiler.Compile()
		if err != nil {
			return err
		}
		err = draw.DOT(g.G, os.Stdout)
		if err != nil {
			return err
		}

		return nil
	},
}
