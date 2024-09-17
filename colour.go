package revisor

import (
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type ColourFormat string

const (
	ColourUnknown ColourFormat = ""
	ColourHex     ColourFormat = "hex"
	ColourRGB     ColourFormat = "rgb"
	ColourRGBA    ColourFormat = "rgba"
)

type cFormatSpec struct {
	Format   ColourFormat
	Prefix   string
	Validate func(spec cFormatSpec, code string) error
}

var (
	defaultColourFormats = []ColourFormat{ColourRGB, ColourRGBA}
	colourComponents     = []string{"r", "g", "b", "alpha"}
	colourFormats        = []cFormatSpec{
		{
			Format:   ColourHex,
			Prefix:   "#",
			Validate: parseHex,
		},
		{
			Format:   ColourRGBA,
			Prefix:   "rgba",
			Validate: parseRGBA,
		},
		{
			Format:   ColourRGB,
			Prefix:   "rgb",
			Validate: parseRGBA,
		},
	}
)

func validateColour(value string, formats []ColourFormat) error {
	var (
		spec cFormatSpec
		code string
	)

	for _, s := range colourFormats {
		after, ok := strings.CutPrefix(value, s.Prefix)
		if !ok {
			continue
		}

		spec = s
		code = after

		break
	}

	if len(formats) == 0 {
		formats = defaultColourFormats
	}

	if spec.Format == ColourUnknown || !slices.Contains(formats, spec.Format) {
		if len(formats) == 1 {
			return fmt.Errorf("expected a colour in the format %q",
				formats[0])
		}

		return fmt.Errorf("expected a colour in one of the formats %s",
			quotedSlice(formats))
	}

	return spec.Validate(spec, code)
}

const hexColourLength = 6

func parseHex(_ cFormatSpec, code string) error {
	if len(code) != hexColourLength {
		return fmt.Errorf("code length: expected %d characters, got %d",
			hexColourLength, len(code))
	}

	_, err := hex.DecodeString(code)
	if err != nil {
		return fmt.Errorf("invalid hex code: %w", err)
	}

	return nil
}

func parseRGBA(spec cFormatSpec, code string) error {
	rest, ok := strings.CutPrefix(code, "(")
	if !ok {
		return errors.New("missing starting '('")
	}

	rest, ok = strings.CutSuffix(rest, ")")
	if !ok {
		return errors.New("missing closing ')'")
	}

	numberStrings := strings.Split(rest, ",")
	components := len(numberStrings)

	//nolint: exhaustive
	switch spec.Format {
	case ColourRGB:
		if components != 3 {
			return fmt.Errorf("expected three components in a rgb() value, got %d", components)
		}
	case ColourRGBA:
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
	default:
		return fmt.Errorf(
			"configuration error: cannot parse %q with parseRGBA()",
			spec.Format,
		)
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

	return nil
}

func quotedSlice[T any](s []T) string {
	ss := make([]string, len(s))

	for i, v := range s {
		ss[i] = strconv.Quote(fmt.Sprintf("%v", v))
	}

	return strings.Join(ss, ", ")
}
