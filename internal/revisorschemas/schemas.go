package revisorschemas

import (
	"embed"
)

//go:embed *.json
var specifications embed.FS

func Files() embed.FS {
	return specifications
}
