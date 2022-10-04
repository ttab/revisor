package constraints

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"log"

	"github.com/navigacontentlab/revisor"
)

//go:embed naviga.json
var defaultSpec []byte

func Naviga() revisor.ConstraintSet {
	var spec revisor.ConstraintSet

	dec := json.NewDecoder(bytes.NewReader(defaultSpec))

	dec.DisallowUnknownFields()

	err := dec.Decode(&spec)
	if err != nil {
		log.Fatalf("failed to unmarshal Naviga constraints: %v", err)
	}

	return spec
}
