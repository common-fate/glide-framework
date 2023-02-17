package main

import (
	"log"
	"os"

	"github.com/common-fate/glide/cmd/command"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:  "glide",
		Usage: "https://commonfate.io",
		Commands: []*cli.Command{
			&command.Compile,
			&command.Run,
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
