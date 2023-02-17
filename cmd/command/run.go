package command

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/common-fate/clio"
	"github.com/common-fate/glide"
	"github.com/common-fate/glide/pkg/dialect/cf"
	"github.com/common-fate/glide/pkg/jsoncel"
	"github.com/common-fate/glide/pkg/noderr"
	"github.com/dominikbraun/graph/draw"
	"github.com/urfave/cli/v2"
)

var Run = cli.Command{
	Name: "run",
	Flags: []cli.Flag{
		&cli.PathFlag{Name: "file", Aliases: []string{"f"}, Usage: "the workflow YAML file to compile", Required: true},
		&cli.PathFlag{Name: "schema", Aliases: []string{"s"}, Usage: "the input schema, in JSON schema format", Required: true},
		&cli.PathFlag{Name: "input", Aliases: []string{"i"}, Usage: "the input data for the workflow, in JSON format", Required: true},
	},
	Action: func(c *cli.Context) error {
		f := c.Path("file")
		schemaFile := c.Path("schema")
		inputFile := c.Path("input")

		data, err := os.ReadFile(f)
		if err != nil {
			return err
		}

		p, err := glide.Unmarshal(data, cf.Dialect)

		var ne noderr.NodeError
		if errors.As(err, &ne) {
			clio.Infof("node error at: %s", ne.Node.GetPath())
			source, printErr := ne.PrettyPrint(data)
			if printErr != nil {
				clio.Errorf("error pretty printing YAML path: %s", printErr)
			}
			fmt.Fprintf(os.Stderr, "%s\n", source)
		}

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

		inputBytes, err := os.ReadFile(inputFile)
		if err != nil {
			return err
		}

		var input map[string]any
		err = json.Unmarshal(inputBytes, &input)
		if err != nil {
			return err
		}

		compiler := glide.Compiler{
			Program:     p,
			InputSchema: &schema,
		}

		// compile the graph
		g, err := compiler.Compile()
		if errors.As(err, &ne) {
			clio.Infof("node error at: %s", ne.Node.GetPath())
			source, printErr := ne.PrettyPrint(data)
			if printErr != nil {
				clio.Errorf("error pretty printing YAML path: %s", printErr)
			}
			fmt.Fprintf(os.Stderr, "%s\n", source)
		}

		if err != nil {
			clio.Error("compile err")
			return err
		}

		// execute the graph
		res, err := g.Execute("request", input)
		if err != nil {
			return err
		}

		outcome := res.Outcome
		if outcome == "" {
			outcome = "<running>"
		}

		clio.Infof("workflow outcome: %s", outcome)

		// shade completed nodes
		for id, state := range res.State {
			_, props, err := g.G.VertexWithProperties(id)
			if err != nil {
				return err
			}
			props.Attributes["style"] = "filled"

			switch state {
			case glide.Complete:
				props.Attributes["fillcolor"] = "#00FF00"
			case glide.Active:
				props.Attributes["fillcolor"] = "#89CFF0"
			}
		}

		err = draw.DOT(g.G, os.Stdout)
		if err != nil {
			return err
		}

		return nil
	},
}
