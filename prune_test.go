package revisor_test

import (
	"context"
	"testing"

	"github.com/ttab/newsdoc"
	"github.com/ttab/revisor"
)

func intPtr(n int) *int {
	return &n
}

func newTestValidator(t *testing.T, sets ...revisor.ConstraintSet) *revisor.Validator {
	t.Helper()

	v, err := revisor.NewValidator(sets...)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	return v
}

func simpleConstraints() revisor.ConstraintSet {
	return revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Attributes: revisor.MakeConstraintMap(
					map[string]revisor.StringConstraint{
						"title": {AllowEmpty: true},
					},
				),
				Links: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/link",
							Rel:  "link",
						},
						Attributes: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"uri": {},
							},
						),
					},
				},
				Meta: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/meta",
						},
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"key": {},
							},
						),
					},
					{
						Declares: &revisor.BlockSignature{
							Type: "test/optional-data",
						},
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"opt": {Optional: true},
								"ae":  {AllowEmpty: true},
							},
						),
					},
				},
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/text",
						},
						Attributes: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"role": {
									Optional: true,
									Enum:     []string{"heading", "body"},
								},
							},
						),
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"text": {},
							},
						),
					},
				},
			},
		},
	}
}

func validDocument() *newsdoc.Document {
	return &newsdoc.Document{
		UUID:     "00000000-0000-0000-0000-000000000001",
		Type:     "test/article",
		Title:    "Test Article",
		Language: "en",
		Content: []newsdoc.Block{
			{
				Type: "test/text",
				Data: map[string]string{
					"text": "Hello world",
				},
			},
		},
		Meta: []newsdoc.Block{
			{
				Type: "test/meta",
				Data: map[string]string{
					"key": "value",
				},
			},
		},
		Links: []newsdoc.Block{
			{
				Type: "test/link",
				Rel:  "link",
				URI:  "http://example.com",
			},
		},
	}
}

func TestPruneValidDocument(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors for valid document, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// Document should be unchanged.
	if len(doc.Content) != 1 {
		t.Errorf("expected 1 content block, got %d", len(doc.Content))
	}

	if len(doc.Meta) != 1 {
		t.Errorf("expected 1 meta block, got %d", len(doc.Meta))
	}

	if len(doc.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(doc.Links))
	}
}

func TestPruneUnknownDataKeysRemoved(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()
	doc.Meta[0].Data["unknown"] = "should be removed"
	doc.Meta[0].Data["another"] = "also removed"

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if _, ok := doc.Meta[0].Data["unknown"]; ok {
		t.Error("expected unknown data key to be removed")
	}

	if _, ok := doc.Meta[0].Data["another"]; ok {
		t.Error("expected another data key to be removed")
	}

	if doc.Meta[0].Data["key"] != "value" {
		t.Errorf("expected known data key to be preserved, got %q",
			doc.Meta[0].Data["key"])
	}
}

func TestPruneUndeclaredBlockAttributeCleared(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()

	// Set an undeclared attribute on a content block.
	doc.Content[0].Title = "unexpected title"
	doc.Content[0].Sensitivity = "high"

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if doc.Content[0].Title != "" {
		t.Errorf("expected title to be cleared, got %q",
			doc.Content[0].Title)
	}

	if doc.Content[0].Sensitivity != "" {
		t.Errorf("expected sensitivity to be cleared, got %q",
			doc.Content[0].Sensitivity)
	}
}

func TestPruneInvalidAttributeWithAllowEmpty(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()

	// Role has enum constraint with Optional=true, so invalid value should
	// be cleared.
	doc.Content[0].Role = "invalid-role"

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if doc.Content[0].Role != "" {
		t.Errorf("expected role to be cleared, got %q",
			doc.Content[0].Role)
	}
}

func TestPruneInvalidOptionalDataDeleted(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()

	doc.Meta = append(doc.Meta, newsdoc.Block{
		Type: "test/optional-data",
		Data: map[string]string{
			"opt": "", // Invalid: empty not allowed for non-AllowEmpty.
			"ae":  "", // Valid: AllowEmpty allows this.
		},
	})

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// "opt" key has Optional → should be deleted when invalid.
	if _, ok := doc.Meta[1].Data["opt"]; ok {
		t.Error("expected optional data key 'opt' to be deleted")
	}
}

func TestPruneUndeclaredBlockRemoved(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()

	doc.Content = append(doc.Content, newsdoc.Block{
		Type: "unknown/block",
		Data: map[string]string{
			"text": "this should be removed",
		},
	})

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if len(doc.Content) != 1 {
		t.Errorf("expected 1 content block after pruning, got %d",
			len(doc.Content))
	}

	if doc.Content[0].Type != "test/text" {
		t.Error("wrong block was removed")
	}
}

func TestPruneRequiredBlockNotRemoved(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Meta: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/required",
						},
						Count: intPtr(1),
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"value": {
									Format: revisor.StringFormatInt,
								},
							},
						),
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Meta: []newsdoc.Block{
			{
				Type: "test/required",
				Data: map[string]string{
					"value": "not-an-int",
				},
			},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should report error since block can't be removed (Count=1).
	if len(res) == 0 {
		t.Fatal("expected errors for required block with bad data")
	}

	// Block should still be present.
	if len(doc.Meta) != 1 {
		t.Errorf("expected meta block to remain, got %d blocks",
			len(doc.Meta))
	}
}

func TestPruneCascadeNestedUnfixableRemoved(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/wrapper",
						},
						Content: []*revisor.BlockConstraint{
							{
								Declares: &revisor.BlockSignature{
									Type: "test/inner",
								},
								Data: revisor.MakeConstraintMap(
									map[string]revisor.StringConstraint{
										"required": {},
									},
								),
							},
						},
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Content: []newsdoc.Block{
			{
				Type: "test/wrapper",
				Content: []newsdoc.Block{
					{
						Type: "test/inner",
						Data: map[string]string{
							"required": "", // Empty, but not AllowEmpty.
						},
					},
				},
			},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// The inner block should be removed (no count constraint), and the
	// wrapper stays since it is itself valid.
	if len(doc.Content) != 1 {
		t.Fatalf("expected wrapper to remain, got %d blocks",
			len(doc.Content))
	}

	if len(doc.Content[0].Content) != 0 {
		t.Errorf("expected inner blocks to be removed, got %d",
			len(doc.Content[0].Content))
	}
}

func TestPruneCascadeToRootReportsError(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/wrapper",
						},
						MinCount: intPtr(1),
						Content: []*revisor.BlockConstraint{
							{
								Declares: &revisor.BlockSignature{
									Type: "test/inner",
								},
								MinCount: intPtr(1),
								Data: revisor.MakeConstraintMap(
									map[string]revisor.StringConstraint{
										"required": {},
									},
								),
							},
						},
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Content: []newsdoc.Block{
			{
				Type: "test/wrapper",
				Content: []newsdoc.Block{
					{
						Type: "test/inner",
						Data: map[string]string{
							"required": "",
						},
					},
				},
			},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Errors should cascade all the way to root and be reported.
	if len(res) == 0 {
		t.Fatal("expected errors from cascade to root")
	}

	// Check that we get error with entity chain.
	found := false

	for _, r := range res {
		if len(r.Entity) >= 2 {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected error with entity chain of depth >= 2")

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// Both wrapper and inner should still be present.
	if len(doc.Content) != 1 {
		t.Errorf("expected wrapper to remain, got %d blocks",
			len(doc.Content))
	}

	if len(doc.Content[0].Content) != 1 {
		t.Errorf("expected inner to remain, got %d blocks",
			len(doc.Content[0].Content))
	}
}

func TestPruneMultipleBlocksRemovedSameSlice(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())
	doc := validDocument()

	// Add multiple undeclared blocks interspersed with valid ones.
	doc.Content = []newsdoc.Block{
		{Type: "unknown/a"},
		{
			Type: "test/text",
			Data: map[string]string{"text": "first"},
		},
		{Type: "unknown/b"},
		{
			Type: "test/text",
			Data: map[string]string{"text": "second"},
		},
		{Type: "unknown/c"},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if len(doc.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(doc.Content))
	}

	if doc.Content[0].Data["text"] != "first" {
		t.Errorf("expected first valid block, got %q",
			doc.Content[0].Data["text"])
	}

	if doc.Content[1].Data["text"] != "second" {
		t.Errorf("expected second valid block, got %q",
			doc.Content[1].Data["text"])
	}
}

func TestPruneCountConstraintRespectsMinCount(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/text",
						},
						MinCount: intPtr(2),
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"text": {},
							},
						),
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Content: []newsdoc.Block{
			{
				Type: "test/text",
				Data: map[string]string{"text": "valid"},
			},
			{
				Type: "test/text",
				Data: map[string]string{"text": ""},
			},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The second block has invalid data (empty required text), but removing
	// it would violate MinCount=2. So we should get an error and both
	// blocks should remain.
	if len(res) == 0 {
		t.Fatal("expected errors since removal would violate MinCount")
	}

	if len(doc.Content) != 2 {
		t.Errorf("expected both blocks to remain, got %d",
			len(doc.Content))
	}
}

func TestPruneDataNilAfterAllKeysRemoved(t *testing.T) {
	// Use a constraint where all data keys are optional, so the block
	// survives even after unknown keys are removed.
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Meta: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/allopt",
						},
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"opt1": {Optional: true},
								"opt2": {Optional: true},
							},
						),
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Meta: []newsdoc.Block{
			{
				Type: "test/allopt",
				Data: map[string]string{
					"unknown1": "val1",
					"unknown2": "val2",
				},
			},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if len(doc.Meta) != 1 {
		t.Fatalf("expected 1 meta block, got %d", len(doc.Meta))
	}

	// Data should be nil after all unknown keys are removed.
	if doc.Meta[0].Data != nil {
		t.Errorf("expected data to be nil, got %v", doc.Meta[0].Data)
	}
}

func TestPruneDocumentAttribute(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Attributes: revisor.MakeConstraintMap(
					map[string]revisor.StringConstraint{
						"language": {
							AllowEmpty: true,
							Enum:       []string{"en", "sv"},
						},
					},
				),
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID:     "00000000-0000-0000-0000-000000000001",
		Type:     "test/article",
		Language: "fr",
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// Language should be cleared since AllowEmpty is true.
	if doc.Language != "" {
		t.Errorf("expected language to be cleared, got %q",
			doc.Language)
	}
}

func TestPruneUndeclaredDocumentType(t *testing.T) {
	v := newTestValidator(t, simpleConstraints())

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "unknown/type",
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) == 0 {
		t.Fatal("expected error for undeclared document type")
	}

	found := false

	for _, r := range res {
		if r.Error == `undeclared document type "unknown/type"` {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected undeclared document type error, got: %v", res)
	}
}

func TestPruneExcessBlocksMaxCount(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/text",
						},
						MaxCount: intPtr(2),
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"text": {},
							},
						),
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Content: []newsdoc.Block{
			{Type: "test/text", Data: map[string]string{"text": "first"}},
			{Type: "test/text", Data: map[string]string{"text": "second"}},
			{Type: "test/text", Data: map[string]string{"text": "third"}},
			{Type: "test/text", Data: map[string]string{"text": "fourth"}},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// MaxCount=2, so only the first 2 should remain.
	if len(doc.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(doc.Content))
	}

	if doc.Content[0].Data["text"] != "first" {
		t.Errorf("expected first block, got %q", doc.Content[0].Data["text"])
	}

	if doc.Content[1].Data["text"] != "second" {
		t.Errorf("expected second block, got %q",
			doc.Content[1].Data["text"])
	}
}

func TestPruneExcessBlocksCount(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Meta: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/meta",
						},
						Count: intPtr(1),
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"key": {},
							},
						),
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Meta: []newsdoc.Block{
			{Type: "test/meta", Data: map[string]string{"key": "a"}},
			{Type: "test/meta", Data: map[string]string{"key": "b"}},
			{Type: "test/meta", Data: map[string]string{"key": "c"}},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	// Count=1, so only the first should remain.
	if len(doc.Meta) != 1 {
		t.Fatalf("expected 1 meta block, got %d", len(doc.Meta))
	}

	if doc.Meta[0].Data["key"] != "a" {
		t.Errorf("expected first block, got %q", doc.Meta[0].Data["key"])
	}
}

func TestPruneExcessAfterInvalidRemoval(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/text",
						},
						MaxCount: intPtr(2),
						Data: revisor.MakeConstraintMap(
							map[string]revisor.StringConstraint{
								"text": {},
							},
						),
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	// 5 blocks: 1 invalid (empty text), 4 valid. After removing the
	// invalid one we have 4, but MaxCount=2, so excess removal trims to 2.
	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Content: []newsdoc.Block{
			{Type: "test/text", Data: map[string]string{"text": "first"}},
			{Type: "test/text", Data: map[string]string{"text": ""}},
			{Type: "test/text", Data: map[string]string{"text": "third"}},
			{Type: "test/text", Data: map[string]string{"text": "fourth"}},
			{Type: "test/text", Data: map[string]string{"text": "fifth"}},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if len(doc.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(doc.Content))
	}

	if doc.Content[0].Data["text"] != "first" {
		t.Errorf("expected first block, got %q",
			doc.Content[0].Data["text"])
	}

	if doc.Content[1].Data["text"] != "third" {
		t.Errorf("expected third block (second valid), got %q",
			doc.Content[1].Data["text"])
	}
}

func TestPruneExcessNestedBlocks(t *testing.T) {
	cs := revisor.ConstraintSet{
		Name: "test",
		Documents: []revisor.DocumentConstraint{
			{
				Declares: "test/article",
				Content: []*revisor.BlockConstraint{
					{
						Declares: &revisor.BlockSignature{
							Type: "test/wrapper",
						},
						Meta: []*revisor.BlockConstraint{
							{
								Declares: &revisor.BlockSignature{
									Type: "test/tag",
								},
								MaxCount: intPtr(1),
								Data: revisor.MakeConstraintMap(
									map[string]revisor.StringConstraint{
										"value": {},
									},
								),
							},
						},
					},
				},
			},
		},
	}

	v := newTestValidator(t, cs)

	doc := &newsdoc.Document{
		UUID: "00000000-0000-0000-0000-000000000001",
		Type: "test/article",
		Content: []newsdoc.Block{
			{
				Type: "test/wrapper",
				Meta: []newsdoc.Block{
					{Type: "test/tag", Data: map[string]string{"value": "keep"}},
					{Type: "test/tag", Data: map[string]string{"value": "remove1"}},
					{Type: "test/tag", Data: map[string]string{"value": "remove2"}},
				},
			},
		},
	}

	ctx := context.Background()

	res, err := v.Prune(ctx, doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(res) != 0 {
		t.Errorf("expected no errors, got %d:", len(res))

		for _, r := range res {
			t.Errorf("  %v", r)
		}
	}

	if len(doc.Content[0].Meta) != 1 {
		t.Fatalf("expected 1 nested meta, got %d",
			len(doc.Content[0].Meta))
	}

	if doc.Content[0].Meta[0].Data["value"] != "keep" {
		t.Errorf("expected first tag to be kept, got %q",
			doc.Content[0].Meta[0].Data["value"])
	}
}
