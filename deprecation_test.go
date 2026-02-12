package revisor_test

import (
	"context"
	"encoding/json"
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
	regenerate := regenerateGoldenFiles()

	testConstraints := decodeConstraintSets(t,
		"testdata/constraints/geo.json",
	)

	testValidator, err := revisor.NewValidator(testConstraints...)
	mustf(t, err, "failed to create test validator")

	var document newsdoc.Document

	err = internal.UnmarshalFile("testdata/geo.json", &document)
	mustf(t, err, "unmarshal geo doc")

	var (
		got    []deprecationEntry
		counts = make(map[string]int)
	)

	deprecationHandler := func(
		_ context.Context, _ *newsdoc.Document,
		deprecation revisor.Deprecation,
		c revisor.DeprecationContext,
	) (revisor.DeprecationDecision, error) {
		got = append(got, deprecationEntry{
			Deprecation: deprecation,
			Context:     c,
		})

		counts[deprecation.Label]++

		return revisor.DeprecationDecision{
			Enforce: true,
		}, nil
	}

	ctx := context.Background()

	res, err := testValidator.ValidateDocument(ctx, &document,
		revisor.WithDeprecationHandler(deprecationHandler))
	mustf(t, err, "validate document")

	t.Run("FoundDeprecations", func(t *testing.T) {
		var (
			goldenPath = "testdata/results-deprecation/geo.json"
			want       []deprecationEntry
		)

		if regenerate {
			data, err := json.MarshalIndent(got, "", "  ")
			mustf(t, err, "marshal for golden reference file")

			err = os.WriteFile(goldenPath, data, 0o600)
			mustf(t, err, "write golden reference file")
		}

		err = internal.UnmarshalFile(goldenPath, &want)
		mustf(t, err, "unmarshal golden file")

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
			mustf(t, err, "marshal for golden reference file")

			err = os.WriteFile(goldenPath, data, 0o600)
			mustf(t, err, "write golden reference file")
		}

		err = internal.UnmarshalFile(goldenPath, &want)
		mustf(t, err, "unmarshal golden file")

		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("enforcement mismatch (-want +got):\n%s",
				diff)
		}
	})
}
