package revisor_test

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ttab/newsdoc"
	"github.com/ttab/revisor"
	"github.com/ttab/revisor/internal"
	"github.com/ttab/revisorschemas"
)

func FuzzValidationDocuments(f *testing.F) {
	var (
		constraints      revisor.ConstraintSet
		extraConstraints revisor.ConstraintSet
	)

	sFS := revisorschemas.Files()

	err := internal.UnmarshalFileFS(sFS, "core.json", &constraints)
	if err != nil {
		f.Fatalf("failed to unmarshal base constraints: %v", err)
	}

	err = internal.UnmarshalFileFS(sFS, "tt.json", &extraConstraints)
	if err != nil {
		f.Fatalf("failed to unmarshal example constraints: %v", err)
	}

	validator, err := revisor.NewValidator(constraints, extraConstraints)
	if err != nil {
		f.Fatalf("failed to create validator: %v", err)
	}

	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		f.Fatalf("failed to glob for sample document files: %v", err)
	}

	for i := range paths {
		docData, err := os.ReadFile(paths[i])
		if err != nil {
			f.Fatalf("failed to read document data from %q: %v",
				paths[i], err)
		}

		f.Add(docData)
	}

	f.Fuzz(func(t *testing.T, documentData []byte) {
		var document newsdoc.Document

		if !decodeBytes(t, documentData, &document) {
			return
		}

		ctx := context.Background()

		_, _ = validator.ValidateDocument(ctx, &document)
	})
}

func FuzzValidationWide(f *testing.F) {
	sFS := revisorschemas.Files()

	baseConstraints, err := sFS.ReadFile("core.json")
	if err != nil {
		f.Fatalf("failed to read base constraints: %v", err)
	}

	exampleConstraints, err := sFS.ReadFile("tt.json")
	if err != nil {
		f.Fatalf("failed to read example constraints: %v", err)
	}

	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		f.Fatalf("failed to glob for result files: %v", err)
	}

	for i := range paths {
		docData, err := os.ReadFile(paths[i])
		if err != nil {
			f.Fatalf("failed to read document data from %q: %v",
				paths[i], err)
		}

		f.Add(baseConstraints, exampleConstraints, docData)
	}

	f.Fuzz(func(t *testing.T, constraintsA []byte, constraintsB []byte, documentData []byte) {
		var (
			document         newsdoc.Document
			constraints      revisor.ConstraintSet
			extraConstraints revisor.ConstraintSet
		)

		if !(decodeBytes(t, constraintsA, &constraints) &&
			decodeBytes(t, constraintsB, &extraConstraints) &&
			decodeBytes(t, documentData, &document)) {
			return
		}

		validator, err := revisor.NewValidator(constraints, extraConstraints)
		if err != nil {
			return
		}

		ctx := context.Background()

		_, _ = validator.ValidateDocument(ctx, &document)
	})
}

func FuzzValidationConstraints(f *testing.F) {
	sFS := revisorschemas.Files()

	constraintPaths, err := fs.Glob(sFS, filepath.Join("constraints", "*.json"))
	if err != nil {
		f.Fatalf("failed to glob for constraint files: %v", err)
	}

	for i := range constraintPaths {
		data, err := sFS.ReadFile(constraintPaths[i])
		if err != nil {
			f.Fatalf("failed to read constraints from %q: %v", constraintPaths[i], err)
		}

		f.Add(data)
	}

	paths, err := filepath.Glob(filepath.Join("testdata", "*.json"))
	if err != nil {
		f.Fatalf("failed to glob for result files: %v", err)
	}

	var documents []*newsdoc.Document

	for i := range paths {
		var d newsdoc.Document

		err := internal.UnmarshalFile(paths[i], &d)
		if err != nil {
			f.Fatalf("failed to decode document %s: %v",
				paths[i], err)
		}

		documents = append(documents, &d)
	}

	f.Fuzz(func(t *testing.T, constraintData []byte) {
		var constraints revisor.ConstraintSet

		if !(decodeBytes(t, constraintData, &constraints)) {
			return
		}

		validator, err := revisor.NewValidator(constraints)
		if err != nil {
			return
		}

		ctx := context.Background()

		for _, document := range documents {
			_, _ = validator.ValidateDocument(ctx, document)
		}
	})
}

type testHelper interface {
	Helper()
}

func decodeBytes(t testHelper, data []byte, o interface{}) bool {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	err := dec.Decode(o)

	return err == nil
}

type validatorTest struct {
	Name      string
	Prefix    string
	Validator *revisor.Validator
}

func decodeConstraintSets(
	t *testing.T, names ...string,
) []revisor.ConstraintSet {
	t.Helper()

	var constraints []revisor.ConstraintSet

	for _, n := range names {
		var c revisor.ConstraintSet

		err := internal.UnmarshalFile(n, &c)
		if err != nil {
			t.Fatalf("failed to load constraints from %q: %v",
				n, err)
		}

		constraints = append(constraints, c)
	}

	return constraints
}

func decodeConstraintSetsFS(
	t *testing.T, sFS embed.FS, names ...string,
) []revisor.ConstraintSet {
	t.Helper()

	var constraints []revisor.ConstraintSet

	for _, n := range names {
		data, err := sFS.ReadFile(n)
		if err != nil {
			t.Fatalf("failed to load constraints from %q: %v",
				n, err)
		}

		var c revisor.ConstraintSet

		decodeBytes(t, data, &c)

		constraints = append(constraints, c)
	}

	return constraints
}

func TestValidateDocument(t *testing.T) {
	sFS := revisorschemas.Files()

	core := decodeConstraintSetsFS(t, sFS,
		"core.json", "core-planning.json",
	)

	baseValidator, err := revisor.NewValidator(core...)
	if err != nil {
		t.Fatalf("failed to create base validator: %v", err)
	}

	extraConstraints := decodeConstraintSetsFS(t, sFS,
		"tt.json", "tt-planning.json",
	)

	orgValidator, err := baseValidator.WithConstraints(extraConstraints...)
	if err != nil {
		t.Fatalf("failed to create extended validator: %v", err)
	}

	testConstraints := decodeConstraintSets(t,
		"testdata/constraints/geo.json",
		"testdata/constraints/labels-hints.json",
	)

	testValidator, err := revisor.NewValidator(testConstraints...)
	if err != nil {
		t.Fatalf("failed to create test validator: %v", err)
	}

	tests := []validatorTest{
		{
			Name:      "Base",
			Prefix:    "base-",
			Validator: baseValidator,
		},
		{
			Name:      "ExtendedConf",
			Prefix:    "example-",
			Validator: orgValidator,
		},
		{
			Name:      "TestConf",
			Prefix:    "test-",
			Validator: testValidator,
		},
	}

	paths, err := filepath.Glob(filepath.Join("testdata", "results", "*.json"))
	if err != nil {
		t.Fatalf("failed to glob for result files: %v", err)
	}

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

				testAgainstGolden(t, goldenPath, testCase)
			}
		})
	}
}

func testAgainstGolden(t *testing.T, goldenPath string, testCase validatorTest) {
	t.Helper()

	sourceDocPath := filepath.Join(
		"testdata",
		strings.TrimPrefix(filepath.Base(goldenPath), testCase.Prefix),
	)

	t.Run(sourceDocPath, func(t *testing.T) {
		var (
			document newsdoc.Document
			want     []revisor.ValidationResult
		)

		err := internal.UnmarshalFile(sourceDocPath, &document)
		must(t, err, "failed to load document")

		err = internal.UnmarshalFile(goldenPath, &want)
		must(t, err, "failed to load expected result")

		ctx := context.Background()

		got, err := testCase.Validator.ValidateDocument(ctx, &document)
		must(t, err, "validate document")

		for i := range got {
			if !resultHas(want, got[i]) {
				t.Errorf("unexpected validation error: %v", got[i])
			}
		}

		for i := range want {
			if !resultHas(got, want[i]) {
				t.Errorf("missing validation error: %v", want[i])
			}
		}

		if len(got) != len(want) {
			t.Errorf("wanted %d errors, got %d",
				len(want), len(got))
		}
	})
}

func resultHas(list []revisor.ValidationResult, item revisor.ValidationResult) bool {
	for i := range list {
		if equalResult(list[i], item) {
			return true
		}
	}

	return false
}

func equalResult(a, b revisor.ValidationResult) bool {
	if a.Error != b.Error {
		return false
	}

	if len(a.Entity) != len(b.Entity) {
		return false
	}

	for i := range a.Entity {
		if a.Entity[i] != b.Entity[i] {
			return false
		}
	}

	return true
}
