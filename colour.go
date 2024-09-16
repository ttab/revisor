package revisor

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const colourLength = 6

var (
	colourComponents = []string{"r", "g", "b", "alpha"}
	colourKeyword    = map[StringFormat]string{
		StringFormatColourRGB:  "rgb",
		StringFormatColourRGBA: "rgba",
	}
)

func validateColour(value string, format StringFormat) error {
	switch format {
	case StringFormatColour:
		if len(value) != colourLength {
			return fmt.Errorf("code length: expected %d characters, got %d",
				colourLength, len(value))
		}

		_, err := hex.DecodeString(value)
		if err != nil {
			return fmt.Errorf("invalid hex code: %w", err)
		}
	case StringFormatColourRGB, StringFormatColourRGBA:
		keyword := colourKeyword[format]

		f, rest, ok := strings.Cut(value, "(")
		if !ok || f != keyword {
			return fmt.Errorf("expected %q colour", keyword)
		}

		numberStrings := strings.Split(strings.Trim(rest, " )"), ",")
		components := len(numberStrings)

		switch f {
		case "rgb":
			if components != 3 {
				return fmt.Errorf("expected three components in a rgb() value, got %d", components)
			}
		case "rgba":
			if components != 4 {
				return fmt.Errorf("expected four components in a rgba() value, got %d", components)
			}

			n, err := strconv.ParseFloat(strings.TrimSpace(numberStrings[3]), 64)
			if err != nil {
				return fmt.Errorf("invalid alpha value: %w", err)
			}

			if n < 0 || n > 1 {
				return fmt.Errorf("%q out of range", colourComponents[3])
			}
		}

		for i, ns := range numberStrings[:3] {
			n, err := strconv.Atoi(strings.TrimSpace(ns))
			if err != nil {
				return fmt.Errorf("invalid %q value: %w",
					colourComponents[i], err)
			}

			if n < 0 || n > 255 {
				return fmt.Errorf("%q out of range", colourComponents[i])
			}
		}
	}

	return nil
}
