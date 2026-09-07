package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

func main() {
	app := cli.Command{
		Name: "serve-wasm",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "addr",
				Value: ":8080",
				Usage: "The address to listen to",
			},
			&cli.StringFlag{
				Name:  "dir",
				Value: "public_html",
			},
		},
		Action: func(_ context.Context, c *cli.Command) error {
			var (
				addr = c.String("addr")
				dir  = c.String("dir")
			)

			fs := http.FileServer(http.Dir(dir))

			server := http.Server{
				Addr:              addr,
				ReadHeaderTimeout: 1 * time.Second,
				Handler: http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
					if strings.HasSuffix(req.URL.Path, ".wasm") {
						resp.Header().Set("content-type", "application/wasm")
					}

					fs.ServeHTTP(resp, req)
				}),
			}

			err := server.ListenAndServe()
			if err != nil {
				return fmt.Errorf("listen and serve: %w", err)
			}

			return nil
		},
	}

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run: %v", err)
		os.Exit(1)
	}
}
