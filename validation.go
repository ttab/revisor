package revisor

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/ttab/newsdoc"
)

type Validator struct {
	constraints []ConstraintSet

	documents            []*DocumentConstraint
	blockConstraints     []BlockConstraintSet
	attributeConstraints []ConstraintMap
	htmlPolicies         map[string]*HTMLPolicy
	enums                *enumSet
}

func NewValidator(
	constraints ...ConstraintSet,
) (*Validator, error) {
	v := Validator{
		constraints:  constraints,
		htmlPolicies: make(map[string]*HTMLPolicy),
		enums:        newEnumSet(),
	}

	docDeclared := make(map[string]bool)
	policySet := NewHTMLPolicySet()

	for _, constraint := range constraints {
		err := constraint.Validate()
		if err != nil {
			return nil, fmt.Errorf("constraint set %q is not valid: %w",
				constraint.Name, err)
		}

		v.blockConstraints = append(v.blockConstraints, constraint)
		v.attributeConstraints = append(v.attributeConstraints, constraint.Attributes)

		for j := range constraint.Documents {
			doc := constraint.Documents[j]

			v.documents = append(v.documents, &doc)

			if doc.Declares == "" {
				continue
			}

			if docDeclared[doc.Declares] {
				return nil, fmt.Errorf("document type %q redeclared in %q",
					doc.Declares, constraint.Name)
			}

			docDeclared[doc.Declares] = true
		}

		err = policySet.Add(constraint.Name, constraint.HTMLPolicies...)
		if err != nil {
			return nil, fmt.Errorf("failed to add HTML policies for %q: %w",
				constraint.Name, err)
		}

		for _, e := range constraint.Enums {
			err := v.enums.Register(e)
			if err != nil {
				return nil, fmt.Errorf("failed to add enum for %q: %w",
					constraint.Name, err)
			}
		}
	}

	err := v.enums.Resolve()
	if err != nil {
		return nil, fmt.Errorf("invalid enums: %w", err)
	}

	htmlPolicies, err := policySet.Resolve()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HTML policies: %w", err)
	}

	v.htmlPolicies = htmlPolicies

	return &v, nil
}

// WithConstraints returns a new Validator that uses an additional set of
// constraints.
func (v *Validator) WithConstraints(
	constraints ...ConstraintSet,
) (*Validator, error) {
	c := slices.Clone(v.constraints)

	c = append(c, constraints...)

	return NewValidator(c...)
}

type ValidationResult struct {
	Entity              []EntityRef `json:"entity,omitempty"`
	Error               string      `json:"error,omitempty"`
	EnforcedDeprecation bool        `json:"enforcedDeprecation,omitempty"`
}

func (vr ValidationResult) String() string {
	if len(vr.Entity) > 0 {
		return entityRefsToString(vr.Entity) + ": " + vr.Error
	}

	return vr.Error
}

func entityRefsToString(refs []EntityRef) string {
	l := len(refs)
	r := make([]string, l)

	for i, v := range refs {
		r[i] = v.String()
	}

	return strings.Join(r, " of ")
}

type RefType string

const (
	RefTypeBlock     RefType = "block"
	RefTypeAttribute RefType = "attribute"
	RefTypeData      RefType = "data attribute"
)

func (rt RefType) String() string {
	return string(rt)
}

type EntityRef struct {
	RefType   RefType   `json:"refType"`
	BlockKind BlockKind `json:"kind,omitempty"`
	Index     int       `json:"index,omitempty"`
	Name      string    `json:"name,omitempty"`
	Type      string    `json:"type,omitempty"`
	Rel       string    `json:"rel,omitempty"`
}

type ValueAnnotation struct {
	Ref        []EntityRef      `json:"ref"`
	Constraint StringConstraint `json:"constraint"`
	Value      string           `json:"value"`
}

type ValueCollector interface {
	CollectValue(a ValueAnnotation)
	With(ref EntityRef) ValueCollector
}

type ValueDiscarder struct{}

// CollectValue implements ValueCollector.
func (ValueDiscarder) CollectValue(_ ValueAnnotation) {
}

// With implements ValueCollector.
func (ValueDiscarder) With(_ EntityRef) ValueCollector {
	return ValueDiscarder{}
}

func (er EntityRef) String() string {
	if er.RefType == RefTypeBlock {
		return fmt.Sprintf("%s %d %s",
			er.BlockKind.Description(1),
			er.Index+1,
			er.typeDesc(),
		)
	}

	return fmt.Sprintf("%s %q", er.RefType.String(), er.Name)
}

func (er EntityRef) typeDesc() string {
	if er.Type == "" && er.Rel == "" {
		return ""
	}

	if er.Type != "" && er.Rel != "" {
		return fmt.Sprintf("%s(%s)", er.Rel, er.Type)
	}

	if er.Type != "" {
		return fmt.Sprintf("(%s)", er.Type)
	}

	return er.Rel
}

func (v *Validator) validateHTML(policyName string, value string) error {
	if policyName == "" {
		policyName = "default"
	}

	policy, ok := v.htmlPolicies[policyName]
	if !ok {
		return fmt.Errorf("no %q HTML policy defined", policyName)
	}

	return policy.Check(value)
}

type ValidationOptionFunc func(vc *ValidationContext)

func WithValueCollector(
	collector ValueCollector,
) ValidationOptionFunc {
	return func(vc *ValidationContext) {
		vc.coll = collector
	}
}

type DeprecationContext struct {
	// Entity references the deprecated entity. Empty if this is a document
	// deprecation.
	Entity *EntityRef `json:"entity,omitempty"`
	// Block is provided unless this is a document or document attribute deprecation.
	Block *newsdoc.Block `json:"block,omitempty"`
	// Value is provided if this was a value deprecation.
	Value *string `json:"value,omitempty"`
}

// DeprecationDecision tells revisor how to handle the deprecation.
type DeprecationDecision struct {
	Enforce bool
	Message string
}

// DeprecationHandlerFunc can handle a deprecation, and should return an error
// if the deprecation should be enforced (treated as a validation error).
type DeprecationHandlerFunc func(
	ctx context.Context,
	doc *newsdoc.Document, deprecation Deprecation, c DeprecationContext,
) (DeprecationDecision, error)

func WithDeprecationHandler(
	fn DeprecationHandlerFunc,
) ValidationOptionFunc {
	return func(vc *ValidationContext) {
		vc.depr = fn
	}
}

func (v *Validator) ValidateDocument(
	ctx context.Context,
	document *newsdoc.Document, opts ...ValidationOptionFunc,
) ([]ValidationResult, error) {
	var res []ValidationResult

	blockConstraints := append([]BlockConstraintSet{}, v.blockConstraints...)
	attributeConstraints := append([]ConstraintMap{}, v.attributeConstraints...)

	var declared bool

	vCtx := ValidationContext{
		coll:         ValueDiscarder{},
		ValidateHTML: v.validateHTML,
		ValidateEnum: v.enums.ValidValue,
	}

	for i := range opts {
		opts[i](&vCtx)
	}

	_, err := uuid.Parse(document.UUID)
	if err != nil {
		res = append(res, ValidationResult{
			Entity: []EntityRef{
				{
					RefType: RefTypeAttribute,
					Name:    "uuid",
				},
			},
			Error: fmt.Sprintf("not a valid UUID: %v", err),
		})
	}

	for i := range v.documents {
		match := v.documents[i].Matches(document, &vCtx)
		if match == NoMatch {
			continue
		}

		if match == MatchDeclaration {
			declared = true
		}

		res, err = checkDeprecation(
			ctx, vCtx, res, document,
			DeprecationContext{},
			v.documents[i].Deprecated)
		if err != nil {
			return nil, err
		}

		blockConstraints = append(blockConstraints, v.documents[i])
		attributeConstraints = append(attributeConstraints, v.documents[i].Attributes)
	}

	if !declared {
		res = append(res, ValidationResult{
			Error: fmt.Sprintf("undeclared document type %q", document.Type),
		})
	}

	res, err = v.validateBlocks(
		ctx, document,
		NewDocumentBlocks(document),
		blockConstraints, res, vCtx,
	)
	if err != nil {
		return nil, err
	}

	res, err = validateDocumentAttributes(
		ctx, attributeConstraints, document, res, vCtx)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func checkDeprecation(
	ctx context.Context,
	vCtx ValidationContext,
	res []ValidationResult,
	doc *newsdoc.Document,
	dCtx DeprecationContext,
	deprecations ...*Deprecation,
) ([]ValidationResult, error) {
	if len(deprecations) == 0 || vCtx.depr == nil {
		return res, nil
	}

	for _, depr := range deprecations {
		if depr == nil {
			continue
		}

		d, err := vCtx.depr(
			ctx, doc,
			*depr,
			dCtx,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"deprecation handler failure: %w", err)
		}

		if d.Enforce {
			msg := d.Message
			if msg == "" {
				msg = depr.Doc
			}

			var entity []EntityRef

			if dCtx.Entity != nil {
				entity = append(entity, *dCtx.Entity)
			}

			res = append(res, ValidationResult{
				Entity: entity,
				Error: fmt.Sprintf(
					"enforced deprecation %q: %s",
					depr.Label, msg),
				EnforcedDeprecation: true,
			})
		}
	}

	return res, nil
}

func validateDocumentAttributes(
	ctx context.Context,
	constraints []ConstraintMap, d *newsdoc.Document,
	res []ValidationResult, vCtx ValidationContext,
) ([]ValidationResult, error) {
	for i := range constraints {
		for _, k := range constraints[i].Keys {
			value, ok := documentAttribute(d, k)

			ref := EntityRef{
				RefType: RefTypeAttribute,
				Name:    k,
			}

			check := constraints[i].Constraints[k]

			depr, err := check.Validate(value, ok, &vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{ref},
					Error:  err.Error(),
				})
			}

			res, err = checkDeprecation(
				ctx, vCtx, res, d,
				DeprecationContext{
					Entity: &ref,
					Value:  &value,
				}, depr, check.Deprecated)
			if err != nil {
				return nil, err
			}

			if value != "" {
				vCtx.coll.CollectValue(ValueAnnotation{
					Ref:   []EntityRef{ref},
					Value: value,
				})
			}
		}
	}

	return res, nil
}

func (v *Validator) validateBlocks(
	ctx context.Context, doc *newsdoc.Document,
	blocks BlockSource,
	constraints []BlockConstraintSet, res []ValidationResult,
	vCtx ValidationContext,
) ([]ValidationResult, error) {
	c := vCtx

	c.ValidateHTML = v.validateHTML

	var err error

	for i := range blockKinds {
		res, err = v.validateBlockSlice(
			ctx, doc,
			blocks.GetBlocks(blockKinds[i]), vCtx,
			constraints, blockKinds[i],
			res,
		)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (v *Validator) validateBlockSlice(
	ctx context.Context, doc *newsdoc.Document,
	blocks []newsdoc.Block, vCtx ValidationContext,
	constraints []BlockConstraintSet, kind BlockKind,
	res []ValidationResult,
) ([]ValidationResult, error) {
	matches := make(map[*BlockConstraint]int)

	for i := range blocks {
		entity := EntityRef{
			RefType:   RefTypeBlock,
			Index:     i,
			BlockKind: kind,
			Type:      blocks[i].Type,
			Rel:       blocks[i].Rel,
		}

		childCtx := vCtx

		childCtx.coll = vCtx.coll.With(entity)

		r, err := v.validateBlock(
			ctx, doc,
			&blocks[i], childCtx, constraints, entity, matches, nil,
		)
		if err != nil {
			return nil, err
		}

		for j := range r {
			r[j].Entity = append(r[j].Entity, entity)
		}

		res = append(res, r...)
	}

	for i := range constraints {
		for _, constraint := range constraints[i].BlockConstraints(kind) {
			count := matches[constraint]

			valid := nilOrEqual(constraint.Count, count) &&
				nilOrGTE(constraint.MinCount, count) &&
				nilOrLTE(constraint.MaxCount, count)
			if !valid {
				res = append(res, ValidationResult{
					Error: constraint.DescribeCountConstraint(kind),
				})
			}
		}
	}

	return res, nil
}

func nilOrEqual(t *int, n int) bool {
	if t == nil {
		return true
	}

	return *t == n
}

func nilOrLTE(t *int, n int) bool {
	if t == nil {
		return true
	}

	return n <= *t
}

func nilOrGTE(t *int, n int) bool {
	if t == nil {
		return true
	}

	return n >= *t
}

func (v *Validator) validateBlock(
	ctx context.Context, doc *newsdoc.Document,
	b *newsdoc.Block, vCtx ValidationContext,
	constraintSets []BlockConstraintSet, entity EntityRef,
	matches map[*BlockConstraint]int, res []ValidationResult,
) ([]ValidationResult, error) {
	var (
		defined                     bool
		matchedConstraints          []BlockConstraintSet
		matchedDataConstraints      []ConstraintMap
		matchedAttributeConstraints []ConstraintMap
	)

	if b.UUID != "" {
		_, err := uuid.Parse(b.UUID)
		if err != nil {
			res = append(res, ValidationResult{
				Entity: []EntityRef{
					{
						RefType: RefTypeAttribute,
						Name:    "uuid",
					},
				},
				Error: fmt.Sprintf("not a valid UUID: %v", err),
			})
		}
	}

	declaredAttributes := make(map[blockAttributeKey]bool)

	var declaredKeys []blockAttributeKey

	for _, set := range constraintSets {
		constraints := set.BlockConstraints(entity.BlockKind)

		for _, constraint := range constraints {
			match, attributes := constraint.Matches(b)
			if match == NoMatch {
				continue
			}

			if match == MatchDeclaration {
				defined = true
			}

			r, err := checkDeprecation(
				ctx, vCtx, res, doc,
				DeprecationContext{
					Entity: &entity,
					Block:  b,
				}, constraint.Deprecated)
			if err != nil {
				return nil, err
			}

			res = r

			for i := range attributes {
				k := blockAttributeKey(attributes[i])

				if !declaredAttributes[k] {
					declaredAttributes[k] = true

					declaredKeys = append(declaredKeys, k)
				}
			}

			matches[constraint]++

			matchedConstraints = append(
				matchedConstraints, constraint)

			if len(constraint.BlocksFrom) > 0 {
				matchedConstraints = append(
					matchedConstraints,
					v.borrowedBlockConstraints(constraint.BlocksFrom, vCtx)...,
				)
			}

			matchedDataConstraints = append(
				matchedDataConstraints, constraint.Data)

			matchedAttributeConstraints = append(
				matchedAttributeConstraints, constraint.Attributes)
		}
	}

	if !defined {
		res = append(res, ValidationResult{
			Error: "undeclared block type or rel",
		})
	}

	slices.Sort(declaredKeys)

	for _, k := range declaredKeys {
		value, _ := blockMatchAttribute(b, string(k))

		vCtx.coll.CollectValue(ValueAnnotation{
			Ref: []EntityRef{{
				RefType: RefTypeAttribute,
				Name:    string(k),
			}},
			Constraint: StringConstraint{
				Const: &value,
			},
			Value: value,
		})
	}

	var err error

	res, err = validateBlockAttributes(
		ctx, doc,
		declaredAttributes,
		matchedAttributeConstraints, b, vCtx, res)
	if err != nil {
		return nil, err
	}

	res, err = validateBlockData(
		ctx, doc, b.Data, vCtx, b, matchedDataConstraints, res)
	if err != nil {
		return nil, err
	}

	res, err = v.validateBlocks(
		ctx, doc,
		NewNestedBlocks(b),
		matchedConstraints, res, vCtx,
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (v *Validator) borrowedBlockConstraints(
	list []BlocksFrom, vCtx ValidationContext,
) []BlockConstraintSet {
	var match []BlockConstraintSet

	for _, borrow := range list {
		if borrow.Global {
			for _, c := range v.blockConstraints {
				match = append(
					match,
					BorrowedBlocks{
						Kind:   borrow.Kind,
						Source: c,
					},
				)
			}
		}

		if borrow.DocType != "" {
			dummyArt := newsdoc.Document{
				Type: borrow.DocType,
			}

			for _, d := range v.documents {
				if d.Matches(&dummyArt, &vCtx) == NoMatch {
					continue
				}

				match = append(
					match,
					BorrowedBlocks{
						Kind:   borrow.Kind,
						Source: d,
					},
				)
			}
		}
	}

	return match
}

func validateBlockAttributes(
	ctx context.Context, doc *newsdoc.Document,
	declaredAttributes map[blockAttributeKey]bool,
	constraints []ConstraintMap, b *newsdoc.Block, vCtx ValidationContext,
	res []ValidationResult,
) ([]ValidationResult, error) {
	if b.UUID != "" {
		_, err := uuid.Parse(b.UUID)
		if err != nil {
			res = append(res, ValidationResult{
				Entity: []EntityRef{{
					RefType: RefTypeAttribute,
					Name:    string(blockAttrUUID),
				}},
				Error: err.Error(),
			})
		}
	}

	for i := range constraints {
		for _, k := range constraints[i].Keys {
			value, ok := blockAttribute(b, k)

			ref := EntityRef{
				RefType: RefTypeAttribute,
				Name:    k,
			}

			check := constraints[i].Constraints[k]

			// Optional attributes are empty strings.
			check.AllowEmpty = check.AllowEmpty || check.Optional

			depr, err := check.Validate(value, ok, &vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{ref},
					Error:  err.Error(),
				})
			}

			res, err = checkDeprecation(
				ctx, vCtx, res, doc, DeprecationContext{
					Entity: &ref,
					Block:  b,
					Value:  &value,
				}, check.Deprecated, depr)
			if err != nil {
				return nil, err
			}

			if value != "" {
				vCtx.coll.CollectValue(ValueAnnotation{
					Ref:        []EntityRef{ref},
					Constraint: check,
					Value:      value,
				})
			}

			declaredAttributes[blockAttributeKey(k)] = true
		}
	}

	for i := range allBlockAttributes {
		if declaredAttributes[allBlockAttributes[i]] {
			continue
		}

		k := string(allBlockAttributes[i])

		value, ok := blockAttribute(b, k)
		if ok && value != "" {
			res = append(res, ValidationResult{
				Entity: []EntityRef{{
					RefType: RefTypeAttribute,
					Name:    k,
				}},
				Error: "undeclared block attribute",
			})
		}
	}

	return res, nil
}

func validateBlockData(
	ctx context.Context, doc *newsdoc.Document,
	data map[string]string, vCtx ValidationContext, b *newsdoc.Block,
	constraints []ConstraintMap, res []ValidationResult,
) ([]ValidationResult, error) {
	known := make(map[string]bool)

	for i := range constraints {
		for _, k := range constraints[i].Keys {
			var (
				v  string
				ok bool
			)

			check := constraints[i].Constraints[k]

			if data != nil {
				v, ok = data[k]
			}

			if ok && !known[k] {
				known[k] = true
			}

			ref := EntityRef{
				RefType: RefTypeData,
				Name:    k,
			}

			if !ok && !check.Optional {
				res = append(res, ValidationResult{
					Entity: []EntityRef{ref},
					Error:  "missing required attribute",
				})
			}

			if !ok {
				continue
			}

			depr, err := check.Validate(v, true, &vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{ref},
					Error:  err.Error(),
				})
			}

			r, err := checkDeprecation(
				ctx, vCtx, res, doc,
				DeprecationContext{
					Entity: &ref,
					Block:  b,
					Value:  &v,
				}, check.Deprecated, depr)
			if err != nil {
				return nil, err
			}

			res = r

			vCtx.coll.CollectValue(ValueAnnotation{
				Ref:        []EntityRef{ref},
				Constraint: check,
				Value:      v,
			})
		}
	}

	var unknownKeys []string

	for k := range data {
		if known[k] {
			continue
		}

		unknownKeys = append(unknownKeys, k)
	}

	slices.Sort(unknownKeys)

	for _, k := range unknownKeys {
		res = append(res, ValidationResult{
			Entity: []EntityRef{{
				RefType: RefTypeData,
				Name:    k,
			}},
			Error: "unknown attribute",
		})
	}

	return res, nil
}

type BlockConstraintSet interface {
	// BlockConstraints returns the constraints of the specified kind.
	BlockConstraints(kind BlockKind) []*BlockConstraint
}

type Deprecation struct {
	Label string `json:"label"`
	Doc   string `json:"doc"`
}

type ConstraintSet struct {
	Version      int                  `json:"version,omitempty"`
	Schema       string               `json:"$schema,omitempty"`
	Name         string               `json:"name"`
	Documents    []DocumentConstraint `json:"documents,omitempty"`
	Links        []*BlockConstraint   `json:"links,omitempty"`
	Meta         []*BlockConstraint   `json:"meta,omitempty"`
	Content      []*BlockConstraint   `json:"content,omitempty"`
	Attributes   ConstraintMap        `json:"attributes,omitempty"`
	Enums        []Enum               `json:"enums,omitempty"`
	HTMLPolicies []HTMLPolicy         `json:"htmlPolicies,omitempty"`
}

func (cs ConstraintSet) Validate() error {
	err := validateBlockConstraints(map[string][]*BlockConstraint{
		"link":    cs.Links,
		"meta":    cs.Meta,
		"content": cs.Content,
	})
	if err != nil {
		return err
	}

	for i, doc := range cs.Documents {
		err := validateBlockConstraints(map[string][]*BlockConstraint{
			"link":    doc.Links,
			"meta":    doc.Meta,
			"content": doc.Content,
		})
		if err != nil {
			return fmt.Errorf("document %d: %w", i+1, err)
		}
	}

	return nil
}

func validateBlockConstraints(c map[string][]*BlockConstraint) error {
	for k := range c {
		for i, block := range c[k] {
			if block == nil {
				return fmt.Errorf("%s block %d must not be nil/null", k, i+1)
			}

			err := validateBlockConstraints(map[string][]*BlockConstraint{
				"link":    block.Links,
				"meta":    block.Meta,
				"content": block.Content,
			})
			if err != nil {
				return fmt.Errorf("%s block %d: %w", k, i+1, err)
			}
		}
	}

	return nil
}

func (cs ConstraintSet) BlockConstraints(kind BlockKind) []*BlockConstraint {
	switch kind {
	case BlockKindLink:
		return cs.Links
	case BlockKindMeta:
		return cs.Meta
	case BlockKindContent:
		return cs.Content
	}

	return nil
}
