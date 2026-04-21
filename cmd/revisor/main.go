package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/ttab/revisor"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "revisor",
		Commands: []*cli.Command{
			{
				Name:  "jsonschema",
				Usage: "generates a JSON schema for revisor specifications",
				Action: func(_ *cli.Context) error {
					schema := jsonschema.Reflect(&revisor.ConstraintSet{})

					enc := json.NewEncoder(os.Stdout)

					enc.SetIndent("", "  ")

					err := enc.Encode(schema)
					if err != nil {
						return fmt.Errorf("failed to encode schema: %w", err)
					}

					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())

		os.Exit(1)
	}
}
