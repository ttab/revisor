package revisor_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/ttab/newsdoc"
	"github.com/ttab/revisor"
	"github.com/ttab/revisor/internal"
)

func BenchmarkValidateDocument(b *testing.B) {
	var (
		document         newsdoc.Document
		constraints      revisor.ConstraintSet
		extraConstraints revisor.ConstraintSet
	)

	err := internal.UnmarshalFile("constraints/core.json", &constraints)
	if err != nil {
		panic(fmt.Errorf(
			"failed to load constraints: %w", err))
	}

	err = internal.UnmarshalFile("constraints/tt.json",
		&extraConstraints)
	if err != nil {
		panic(fmt.Errorf(
			"failed to load constraints: %w", err))
	}

	err = internal.UnmarshalFile("testdata/example-article.json", &document)
	if err != nil {
		panic(fmt.Errorf(
			"failed to load constraints: %w", err))
	}

	validator, err := revisor.NewValidator(constraints, extraConstraints)
	if err != nil {
		panic(fmt.Errorf("failed to create validator: %w", err))
	}

	ctx := context.Background()

	for n := 0; n < b.N; n++ {
		_, _ = validator.ValidateDocument(ctx, &document)
	}
}
