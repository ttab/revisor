package revisor

import (
	"slices"
	"strings"

	"github.com/ttab/newsdoc"
)

// DocumentConstraint describes a set of constraints for a document. Either by
// declaring a document type, or matching against a document that has been
// declared somewhere else.
type DocumentConstraint struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	// Declares is used to declare a document type.
	Declares string `json:"declares,omitempty"`
	// Match is used to extend other document declarations.
	Match      ConstraintMap      `json:"match,omitempty"`
	Links      []*BlockConstraint `json:"links,omitempty"`
	Meta       []*BlockConstraint `json:"meta,omitempty"`
	Content    []*BlockConstraint `json:"content,omitempty"`
	Attributes ConstraintMap      `json:"attributes,omitempty"`
	Deprecated *Deprecation       `json:"deprecated,omitempty"`
}

// BlockConstraints implements the BlockConstraintsSet interface.
func (dc DocumentConstraint) BlockConstraints(kind BlockKind) []*BlockConstraint {
	switch kind {
	case BlockKindLink:
		return dc.Links
	case BlockKindMeta:
		return dc.Meta
	case BlockKindContent:
		return dc.Content
	}

	return nil
}

// SetBlockConstraints implements the BlockConstraintsSet interface.
func (dc *DocumentConstraint) SetBlockConstraints(kind BlockKind, blocks []*BlockConstraint) {
	switch kind {
	case BlockKindLink:
		dc.Links = blocks
	case BlockKindMeta:
		dc.Meta = blocks
	case BlockKindContent:
		dc.Content = blocks
	}
}

// Matches checks if the given document matches the constraint.
func (dc DocumentConstraint) Matches(
	d *newsdoc.Document, vCtx *ValidationContext,
) Match {
	if dc.Declares != "" {
		if d.Type == dc.Declares || resolveVariant(d.Type, vCtx.variants) == dc.Declares {
			return MatchDeclaration
		}

		return NoMatch
	}

	for _, k := range dc.Match.Keys {
		value, ok := documentMatchAttribute(d, k, vCtx.variants)
		if !ok {
			return NoMatch
		}

		check := dc.Match.Constraints[k]

		_, err := check.Validate(value, ok, vCtx)
		if err != nil {
			return NoMatch
		}

		vCtx.coll.CollectValue(ValueAnnotation{
			Ref: []EntityRef{{
				RefType: RefTypeAttribute,
				Name:    k,
			}},
			Value: value,
		})
	}

	return Matches
}

type documentAttributeKey string

const (
	docAttrType     documentAttributeKey = "type"
	docAttrLanguage documentAttributeKey = "language"
	docAttrTitle    documentAttributeKey = "title"
	docAttrUUID     documentAttributeKey = "uuid"
	docAttrURI      documentAttributeKey = "uri"
	docAttrURL      documentAttributeKey = "url"
)

func documentMatchAttribute(d *newsdoc.Document, name string, variants []Variant) (string, bool) {
	if documentAttributeKey(name) == docAttrType {
		return resolveVariant(d.Type, variants), true
	}

	return "", false
}

func setDocumentAttribute(d *newsdoc.Document, name string, value string) bool {
	switch documentAttributeKey(name) {
	case docAttrUUID:
		d.UUID = value
	case docAttrType:
		d.Type = value
	case docAttrURI:
		d.URI = value
	case docAttrURL:
		d.URL = value
	case docAttrTitle:
		d.Title = value
	case docAttrLanguage:
		d.Language = value
	default:
		return false
	}

	return true
}

func documentAttribute(d *newsdoc.Document, name string) (string, bool) {
	switch documentAttributeKey(name) {
	case docAttrUUID:
		return d.UUID, true
	case docAttrType:
		return d.Type, true
	case docAttrURI:
		return d.URI, true
	case docAttrURL:
		return d.URL, true
	case docAttrTitle:
		return d.Title, true
	case docAttrLanguage:
		return d.Language, true
	}

	return "", false
}

// Variant defines a document type variant suffix. When a document type
// contains a "+" separator (e.g. "core/article+template"), the part after the
// last "+" is matched against configured variants. If Types is empty, the
// variant applies to all declared document types.
type Variant struct {
	Name  string   `json:"name"`
	Types []string `json:"types,omitempty"`
}

// resolveVariant returns the base document type if the suffix after the last
// "+" matches a configured variant (and the base type is allowed for that
// variant). Returns docType unchanged if no variant matches.
func resolveVariant(docType string, variants []Variant) string {
	idx := strings.LastIndex(docType, "+")
	if idx == -1 {
		return docType
	}

	base := docType[:idx]
	suffix := docType[idx+1:]

	for i := range variants {
		if variants[i].Name != suffix {
			continue
		}

		if len(variants[i].Types) == 0 {
			return base
		}

		if slices.Contains(variants[i].Types, base) {
			return base
		}
	}

	return docType
}
