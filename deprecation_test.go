package revisor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ttab/newsdoc"
	"github.com/ttab/revisor"
	"github.com/ttab/revisor/internal"
)

type deprecationEntry struct {
	Deprecation revisor.Deprecation
	Context     revisor.DeprecationContext
}

func TestDeprecation(t *testing.T) {
	regenerate := os.Getenv("REGENERATE") == "true"

	testConstraints := decodeConstraintSets(t,
		"testdata/constraints/geo.json",
	)

	testValidator, err := revisor.NewValidator(testConstraints...)
	must(t, err, "failed to create test validator")

	var document newsdoc.Document

	err = internal.UnmarshalFile("testdata/geo.json", &document)
	must(t, err, "unmarshal geo doc")

	var (
		got    []deprecationEntry
		counts = make(map[string]int)
	)

	dfn := func(
		_ context.Context, _ *newsdoc.Document,
		deprecation revisor.Deprecation,
		c revisor.DeprecationContext,
	) error {
		got = append(got, deprecationEntry{
			Deprecation: deprecation,
			Context:     c,
		})

		counts[deprecation.Label]++

		return fmt.Errorf("nope, can't have %q", deprecation.Label)
	}

	ctx := context.Background()

	res := testValidator.ValidateDocument(ctx, &document,
		revisor.WithDeprecationHandler(dfn))

	t.Run("FoundDeprecations", func(t *testing.T) {
		var (
			goldenPath = "testdata/results-deprecation/geo.json"
			want       []deprecationEntry
		)

		if regenerate {
			data, err := json.MarshalIndent(got, "", "  ")
			must(t, err, "marshal for golden reference file")

			err = os.WriteFile(goldenPath, data, 0o600)
			must(t, err, "write golden reference file")
		}

		err = internal.UnmarshalFile(goldenPath, &want)
		must(t, err, "unmarshal golden file")

		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("deprecation mismatch (-want +got):\n%s",
				diff)
		}
	})

	t.Run("EnforcedDeprecations", func(t *testing.T) {
		var (
			goldenPath = "testdata/results-deprecation/geo.enforced.json"
			got        []revisor.ValidationResult
			want       []revisor.ValidationResult
		)

		for _, r := range res {
			if !r.EnforcedDeprecation {
				continue
			}

			got = append(got, r)
		}

		if regenerate {
			data, err := json.MarshalIndent(got, "", "  ")
			must(t, err, "marshal for golden reference file")

			err = os.WriteFile(goldenPath, data, 0o600)
			must(t, err, "write golden reference file")
		}

		err = internal.UnmarshalFile(goldenPath, &want)
		must(t, err, "unmarshal golden file")

		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("enforcement mismatch (-want +got):\n%s",
				diff)
		}
	})
}
