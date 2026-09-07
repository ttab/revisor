package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/invopop/jsonschema"
	"github.com/ttab/revisor"
	"github.com/urfave/cli/v3"
)

func main() {
	app := cli.Command{
		Name: "revisor",
		Commands: []*cli.Command{
			{
				Name:  "jsonschema",
				Usage: "generates a JSON schema for revisor specifications",
				Action: func(_ context.Context, _ *cli.Command) error {
					schema := jsonschema.Reflect(&revisor.ConstraintSet{})

					enc := json.NewEncoder(os.Stdout)

					enc.SetIndent("", "  ")

					err := enc.Encode(schema)
					if err != nil {
						return fmt.Errorf("encode schema: %w", err)
					}

					return nil
				},
			},
		},
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())

		os.Exit(1)
	}
}
