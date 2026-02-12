package revisor

import (
	"fmt"
	"strings"

	"github.com/ttab/newsdoc"
)

// BlockKind describes the different kinds of blocks that are available.
type BlockKind string

// The different kinds of blocks that a block source can have.
const (
	BlockKindLink    BlockKind = "link"
	BlockKindMeta    BlockKind = "meta"
	BlockKindContent BlockKind = "content"
)

var blockKinds = []BlockKind{
	BlockKindLink, BlockKindMeta, BlockKindContent,
}

var kindNames = map[BlockKind][2]string{
	BlockKindLink:    {"link", "links"},
	BlockKindMeta:    {"meta block", "meta blocks"},
	BlockKindContent: {"content block", "content blocks"},
}

// Description returns the pluralised name of the block kind.
func (bk BlockKind) Description(n int) string {
	name, ok := kindNames[bk]
	if ok {
		if n == 1 {
			return name[0]
		}

		return name[1]
	}

	if n == 1 {
		return "block"
	}

	return "blocks"
}

// BlockSignature is the signature of a block declaration.
type BlockSignature struct {
	Type string `json:"type,omitempty"`
	Rel  string `json:"rel,omitempty"`
	Role string `json:"role,omitempty"`
}

func (bs BlockSignature) AsConstraint() ConstraintMap {
	m := make(map[string]StringConstraint)

	if bs.Type != "" {
		m[string(blockAttrType)] = StringConstraint{
			Const: strPtr(bs.Type),
		}
	}

	if bs.Rel != "" {
		m[string(blockAttrRel)] = StringConstraint{
			Const: strPtr(bs.Rel),
		}
	}

	if bs.Role != "" {
		m[string(blockAttrRole)] = StringConstraint{
			Const: strPtr(bs.Role),
		}
	}

	return MakeConstraintMap(m)
}

func strPtr(v string) *string {
	return &v
}

type BlockDefinition struct {
	ID    string          `json:"id"`
	Block BlockConstraint `json:"block"`
}

// BlockConstraint is a specification for a block.
type BlockConstraint struct {
	Ref         string             `json:"ref,omitempty"`
	Declares    *BlockSignature    `json:"declares,omitempty"`
	Name        string             `json:"name,omitempty"`
	Description string             `json:"description,omitempty"`
	Match       ConstraintMap      `json:"match,omitempty"`
	Count       *int               `json:"count,omitempty"`
	MaxCount    *int               `json:"maxCount,omitempty"`
	MinCount    *int               `json:"minCount,omitempty"`
	Links       []*BlockConstraint `json:"links,omitempty"`
	Meta        []*BlockConstraint `json:"meta,omitempty"`
	Content     []*BlockConstraint `json:"content,omitempty"`
	Attributes  ConstraintMap      `json:"attributes,omitempty"`
	Data        ConstraintMap      `json:"data,omitempty"`
	Deprecated  *Deprecation       `json:"deprecated,omitempty"`
}

// IsNoop returns true if the constraint doesn't affect anything.
func (bc BlockConstraint) IsNoop() bool {
	return bc.Ref == "" && bc.Declares == nil && bc.Count == nil &&
		bc.MaxCount == nil && bc.MinCount == nil &&
		len(bc.Links) == 0 && len(bc.Meta) == 0 && len(bc.Content) == 0 &&
		len(bc.Attributes.Keys) == 0 && len(bc.Data.Keys) == 0 &&
		bc.Deprecated == nil
}

func (bc BlockConstraint) Copy() *BlockConstraint {
	return &BlockConstraint{
		Ref:         bc.Ref,
		Declares:    bSigCopy(bc.Declares),
		Name:        bc.Name,
		Description: bc.Description,
		Match:       bc.Match.Copy(),
		Count:       intPtrCopy(bc.Count),
		MaxCount:    intPtrCopy(bc.MaxCount),
		MinCount:    intPtrCopy(bc.MinCount),
		Links:       bsListCopy(bc.Links),
		Meta:        bsListCopy(bc.Meta),
		Content:     bsListCopy(bc.Content),
		Attributes:  bc.Attributes.Copy(),
		Data:        bc.Data.Copy(),
		Deprecated:  deprCopy(bc.Deprecated),
	}
}

func bSigCopy(v *BlockSignature) *BlockSignature {
	if v == nil {
		return nil
	}

	s := *v

	return &s
}

func intPtrCopy(v *int) *int {
	if v == nil {
		return nil
	}

	n := *v

	return &n
}

func bsListCopy(b []*BlockConstraint) []*BlockConstraint {
	if len(b) == 0 {
		return nil
	}

	s := make([]*BlockConstraint, len(b))

	for i := range b {
		s[i] = b[i].Copy()
	}

	return s
}

func deprCopy(v *Deprecation) *Deprecation {
	if v == nil {
		return nil
	}

	d := *v

	return &d
}

// BlockConstraints implements the BlockConstraintsSet interface.
func (bc BlockConstraint) BlockConstraints(kind BlockKind) []*BlockConstraint {
	switch kind {
	case BlockKindLink:
		return bc.Links
	case BlockKindMeta:
		return bc.Meta
	case BlockKindContent:
		return bc.Content
	}

	return nil
}

// SetBlockConstraints implements the BlockConstraintsSet interface.
func (bc *BlockConstraint) SetBlockConstraints(kind BlockKind, blocks []*BlockConstraint) {
	switch kind {
	case BlockKindLink:
		bc.Links = blocks
	case BlockKindMeta:
		bc.Meta = blocks
	case BlockKindContent:
		bc.Content = blocks
	}
}

// Match describes if and how a block constraint matches a block.
type Match int

// Match constants for no match / match / matched declaration.
const (
	NoMatch Match = iota
	Matches
	MatchDeclaration
)

// Matches checks if the given block matches the constraint and returns the
// names of the attributes that matched.
func (bc BlockConstraint) Matches(b *newsdoc.Block) (Match, []string) {
	match, attributes := bc.declares(b)
	if match == NoMatch {
		return NoMatch, nil
	}

	for _, k := range bc.Match.Keys {
		value, ok := blockMatchAttribute(b, k)

		check := bc.Match.Constraints[k]

		// Optional attributes are empty strings.
		check.AllowEmpty = check.AllowEmpty || check.Optional

		_, err := check.Validate(value, ok, nil)
		if err != nil {
			return NoMatch, nil
		}

		attributes = append(attributes, k)
	}

	return match, attributes
}

func (bc BlockConstraint) declares(b *newsdoc.Block) (Match, []string) {
	var attributes []string

	if bc.Declares == nil {
		return Matches, nil
	}

	if bc.Declares.Type != "" {
		if b.Type != bc.Declares.Type {
			return NoMatch, nil
		}

		attributes = append(attributes, string(blockAttrType))
	}

	if bc.Declares.Rel != "" {
		if b.Rel != bc.Declares.Rel {
			return NoMatch, nil
		}

		attributes = append(attributes, string(blockAttrRel))
	}

	if bc.Declares.Role != "" {
		if b.Role != bc.Declares.Role {
			return NoMatch, nil
		}

		attributes = append(attributes, string(blockAttrRole))
	}

	return MatchDeclaration, attributes
}

type blockAttributeKey string

const (
	blockAttrID          blockAttributeKey = "id"
	blockAttrUUID        blockAttributeKey = "uuid"
	blockAttrType        blockAttributeKey = "type"
	blockAttrURI         blockAttributeKey = "uri"
	blockAttrURL         blockAttributeKey = "url"
	blockAttrTitle       blockAttributeKey = "title"
	blockAttrRel         blockAttributeKey = "rel"
	blockAttrName        blockAttributeKey = "name"
	blockAttrValue       blockAttributeKey = "value"
	blockAttrContentType blockAttributeKey = "contenttype"
	blockAttrRole        blockAttributeKey = "role"
	blockAttrSensitivity blockAttributeKey = "sensitivity"
)

var allBlockAttributes = []blockAttributeKey{
	blockAttrUUID, blockAttrType, blockAttrURI,
	blockAttrURL, blockAttrTitle, blockAttrRel,
	blockAttrName, blockAttrValue, blockAttrContentType,
	blockAttrRole, blockAttrSensitivity,
}

func blockMatchAttribute(block *newsdoc.Block, name string) (string, bool) {
	//nolint:exhaustive
	switch blockAttributeKey(name) {
	case blockAttrType:
		return block.Type, true
	case blockAttrURI:
		return block.URI, true
	case blockAttrURL:
		return block.URL, true
	case blockAttrRel:
		return block.Rel, true
	case blockAttrName:
		return block.Name, true
	case blockAttrValue:
		return block.Value, true
	case blockAttrContentType:
		return block.Contenttype, true
	case blockAttrRole:
		return block.Role, true
	case blockAttrSensitivity:
		return block.Sensitivity, true
	}

	return "", false
}

func blockAttribute(block *newsdoc.Block, name string) (string, bool) {
	switch blockAttributeKey(name) {
	case blockAttrUUID:
		return block.UUID, true
	case blockAttrID:
		return block.ID, true
	case blockAttrType:
		return block.Type, true
	case blockAttrURI:
		return block.URI, true
	case blockAttrURL:
		return block.URL, true
	case blockAttrTitle:
		return block.Title, true
	case blockAttrRel:
		return block.Rel, true
	case blockAttrName:
		return block.Name, true
	case blockAttrValue:
		return block.Value, true
	case blockAttrContentType:
		return block.Contenttype, true
	case blockAttrRole:
		return block.Role, true
	case blockAttrSensitivity:
		return block.Sensitivity, true
	}

	return "", false
}

func setBlockAttribute(block *newsdoc.Block, name string, value string) bool {
	switch blockAttributeKey(name) {
	case blockAttrUUID:
		block.UUID = value
	case blockAttrID:
		block.ID = value
	case blockAttrType:
		block.Type = value
	case blockAttrURI:
		block.URI = value
	case blockAttrURL:
		block.URL = value
	case blockAttrTitle:
		block.Title = value
	case blockAttrRel:
		block.Rel = value
	case blockAttrName:
		block.Name = value
	case blockAttrValue:
		block.Value = value
	case blockAttrContentType:
		block.Contenttype = value
	case blockAttrRole:
		block.Role = value
	case blockAttrSensitivity:
		block.Sensitivity = value
	default:
		return false
	}

	return true
}

// DescribeCountConstraint returns a human readable (english) description of the
// count contstraint for the block constraint.
func (bc BlockConstraint) DescribeCountConstraint(kind BlockKind) string {
	var s strings.Builder

	s.WriteString("there must be ")

	switch {
	case bc.Count != nil:
		fmt.Fprintf(&s, "%d %s",
			*bc.Count, kind.Description(*bc.Count))
	case bc.MinCount != nil && bc.MaxCount != nil:
		fmt.Fprintf(&s,
			"between %d and %d %s",
			*bc.MinCount, *bc.MaxCount,
			kind.Description(*bc.MaxCount),
		)
	case bc.MaxCount != nil:
		fmt.Fprintf(&s,
			"less than %d %s",
			*bc.MaxCount, kind.Description(*bc.MaxCount),
		)
	case bc.MinCount != nil:
		fmt.Fprintf(&s, "%d or more %s",
			*bc.MinCount, kind.Description(2),
		)
	}

	if len(bc.Match.Keys) > 0 {
		s.WriteString(" where ")
		s.WriteString(bc.Match.Requirements())
	}

	if bc.Declares != nil {
		var parts []string

		if bc.Declares.Type != "" {
			parts = append(parts, fmt.Sprintf(
				"type is %q", bc.Declares.Type))
		}

		if bc.Declares.Rel != "" {
			parts = append(parts, fmt.Sprintf(
				"rel is %q", bc.Declares.Rel))
		}

		fmt.Fprintf(&s, " where %s", strings.Join(parts, " and "))
	}

	return s.String()
}
