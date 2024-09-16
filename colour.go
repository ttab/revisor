package revisor

import (
	"encoding/hex"
	"fmt"
)

const colourLength = 6

func validateColour(value string) error {
	if len(value) != colourLength {
		return fmt.Errorf("code length: expected %d characters, got %d",
			colourLength, len(value))
	}

	_, err := hex.DecodeString(value)
	if err != nil {
		return fmt.Errorf("invalid hex code: %w", err)
	}

	return nil
}
