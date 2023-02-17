package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/common-fate/clio"
	"github.com/common-fate/glide"
	"github.com/common-fate/glide/pkg/dialect/cf"
	"github.com/common-fate/glide/pkg/jsoncel"
	"github.com/dominikbraun/graph/draw"
	"github.com/goccy/go-graphviz"
)

func main() {
	err := run()
	if err != nil {
		log.Fatal(err)
	}
}

func run() error {
	exampleFolder := "docs/examples"
	outputFolder := "docs/img"

	folders, err := os.ReadDir(exampleFolder)
	if err != nil {
		return err
	}

	for _, folder := range folders {
		if !folder.IsDir() {
			clio.Infof("skipping %s: not a folder", folder.Name())
			continue
		}

		workflowfile := filepath.Join(exampleFolder, folder.Name(), "workflow.yml")

		workflow, err := os.ReadFile(workflowfile)
		if err != nil {
			return err
		}

		prog, err := glide.Unmarshal(workflow, cf.Dialect)
		if err != nil {
			return err
		}

		schemaFile := filepath.Join(exampleFolder, folder.Name(), "schema.json")

		schemaBytes, err := os.ReadFile(schemaFile)
		if err != nil {
			return err
		}

		var schema jsoncel.Schema
		err = json.Unmarshal(schemaBytes, &schema)
		if err != nil {
			return err
		}

		var run bool // only set if we have an input to run with
		var input map[string]any

		// might or might not have this
		inputFile := filepath.Join(exampleFolder, folder.Name(), "input.json")

		inputBytes, err := os.ReadFile(inputFile)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err == nil {
			run = true

			// try and unmarshal the input
			err = json.Unmarshal(inputBytes, &input)
			if err != nil {
				return err
			}
		}

		compiler := glide.Compiler{
			Program:     prog,
			InputSchema: &schema,
		}

		g, err := compiler.Compile()
		if err != nil {
			return err
		}

		// if we have input.json, run the actual workflow too
		if run {
			res, err := g.Execute("request", input)
			if err != nil {
				return err
			}

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
		}

		var buf bytes.Buffer

		err = draw.DOT(g.G, &buf)
		if err != nil {
			return err
		}

		graph, err := graphviz.ParseBytes(buf.Bytes())
		if err != nil {
			return err
		}
		gv := graphviz.New()

		outfile := filepath.Join(outputFolder, folder.Name()+".svg")
		err = gv.RenderFilename(graph, graphviz.SVG, outfile)
		if err != nil {
			log.Fatal(err)
		}
		clio.Successf("rendered %s", outfile)
	}
	return nil
}
