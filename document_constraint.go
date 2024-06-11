package revisor

import (
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

// Matches checks if the given document matches the constraint.
func (dc DocumentConstraint) Matches(
	d *newsdoc.Document, vCtx *ValidationContext,
) Match {
	if dc.Declares != "" {
		if d.Type == dc.Declares {
			return MatchDeclaration
		}

		return NoMatch
	}

	for _, k := range dc.Match.Keys {
		value, ok := documentMatchAttribute(d, k)
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

func documentMatchAttribute(d *newsdoc.Document, name string) (string, bool) {
	if documentAttributeKey(name) == docAttrType {
		return d.Type, true
	}

	return "", false
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
