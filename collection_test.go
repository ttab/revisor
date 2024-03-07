package revisor_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/ttab/newsdoc"
	"github.com/ttab/revisor"
	"github.com/ttab/revisor/internal"
)

func TestCollection(t *testing.T) {
	regenerate := os.Getenv("REGENERATE") == "true"

	testConstraints := decodeConstraintSets(t,
		"testdata/constraints/geo.json",
		"testdata/constraints/labels-hints.json",
	)

	testValidator, err := revisor.NewValidator(testConstraints...)
	must(t, err, "failed to create test validator")

	tests := []validatorTest{
		{
			Name:      "TestConf",
			Prefix:    "test-",
			Validator: testValidator,
		},
	}

	paths, err := filepath.Glob(filepath.Join("testdata", "results-collection", "*.json"))
	must(t, err, "failed to glob for collection result files")

	for j := range tests {
		testCase := tests[j]

		t.Run(testCase.Name, func(t *testing.T) {
			for i := range paths {
				goldenPath := paths[i]

				if !strings.HasPrefix(
					filepath.Base(goldenPath),
					testCase.Prefix) {
					continue
				}

				testCollectionAgainstGolden(t,
					goldenPath, testCase, regenerate)
			}
		})
	}
}

func testCollectionAgainstGolden(
	t *testing.T,
	goldenPath string, testCase validatorTest,
	regenerate bool,
) {
	t.Helper()

	sourceDocPath := filepath.Join(
		"testdata",
		strings.TrimPrefix(filepath.Base(goldenPath), testCase.Prefix),
	)

	t.Run(sourceDocPath, func(t *testing.T) {
		var document newsdoc.Document // want     []revisor.ValidationResult

		err := internal.UnmarshalFile(sourceDocPath, &document)
		must(t, err, "failed to load document")

		collector := revisor.NewValueCollector()

		ctx := context.Background()

		_, err = testCase.Validator.ValidateDocument(ctx, &document,
			revisor.WithValueCollector(collector))
		must(t, err, "validate document")

		collected := make(map[string]collectedValues)

		for _, a := range collector.Values() {
			key := entityRefsToPath(&document, a.Ref)
			cv := collected[key]

			if cv.Format == "" {
				cv.Format = string(a.Constraint.Format)
			}

			if a.Constraint.Hints != nil && cv.Hints == nil {
				cv.Hints = make(map[string][]string)
			}

			for k, v := range a.Constraint.Hints {
				cv.Hints[k] = v
			}

			for _, l := range a.Constraint.Labels {
				if !slices.Contains(cv.Labels, l) {
					cv.Labels = append(cv.Labels, l)
				}
			}

			if !slices.Contains(cv.Values, a.Value) {
				cv.Values = append(cv.Values, a.Value)
			}

			collected[key] = cv
		}

		if regenerate {
			data, err := json.MarshalIndent(collected, "", "  ")
			must(t, err, "marshal for golden reference file")

			err = os.WriteFile(goldenPath, data, 0o600)
			must(t, err, "write golden reference file")
		}

		var want map[string]collectedValues

		err = internal.UnmarshalFile(goldenPath, &want)
		must(t, err, "failed to load expected result")

		if diff := cmp.Diff(want, collected); diff != "" {
			t.Fatalf("collection mismatch (-want +got):\n%s",
				diff)
		}
	})
}

//nolint:unparam
func must(t *testing.T, err error, format string, a ...any) {
	t.Helper()

	if err != nil {
		t.Fatalf("failed: %s: %v", fmt.Sprintf(format, a...), err)
	}

	if testing.Verbose() {
		t.Logf("success: "+format, a...)
	}
}

type collectedValues struct {
	Format string              `json:"format,omitempty"`
	Values []string            `json:"values,omitempty"`
	Hints  map[string][]string `json:"hints,omitempty"`
	Labels []string            `json:"labels,omitempty"`
}

var nonAlphaNum = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)

func entityRefsToPath(doc *newsdoc.Document, refs []revisor.EntityRef) string {
	r := make([]string, len(refs))

	var source revisor.BlockSource = revisor.NewDocumentBlocks(doc)

	for i, v := range refs {
		switch v.RefType {
		case revisor.RefTypeData:
			r[i] = "data." + v.Name
		case revisor.RefTypeAttribute:
			r[i] = v.Name
		case revisor.RefTypeBlock:
			blocks := source.GetBlocks(v.BlockKind)
			block := blocks[v.Index]

			switch v.BlockKind {
			case revisor.BlockKindLink:
				key := nonAlphaNum.ReplaceAllString(block.Rel, "_")
				r[i] = "rel." + key
			case revisor.BlockKindMeta:
				key := nonAlphaNum.ReplaceAllString(block.Type, "_")
				r[i] = "meta." + key
			case revisor.BlockKindContent:
				key := nonAlphaNum.ReplaceAllString(block.Type, "_")
				r[i] = "content." + key
			}

			source = revisor.NewNestedBlocks(&block)
		}
	}

	return strings.Join(r, ".")
}
