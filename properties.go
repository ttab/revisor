package revisor

import (
	"fmt"
	"strings"

	"github.com/navigacontentlab/navigadoc/doc"
)

// PropertyConstraintMap is used to declare named document properties.
type PropertyConstraintMap map[string]PropertyConstraint

// PropertyConstraint is a declaration for a document property.
type PropertyConstraint struct {
	Description string            `json:"description,omitempty"`
	Value       *StringConstraint `json:"value,omitempty"`
	Parameters  ConstraintMap     `json:"parameters,omitempty"`
	Count       *int              `json:"count"`
	MinCount    *int              `json:"minCount"`
	MaxCount    *int              `json:"maxCount"`
}

// DescribeCountConstraint returns a human readable (english) description of the
// count contstraint for the property constraint.
func (pc PropertyConstraint) DescribeCountConstraint(name string) string {
	var s strings.Builder

	s.WriteString("there must be ")

	prop := func(n int) string {
		if n == 1 {
			return fmt.Sprintf("%q property", name)
		}

		return fmt.Sprintf("%q properties", name)
	}

	switch {
	case pc.Count != nil:
		fmt.Fprintf(&s, "%d %s",
			*pc.Count, prop(*pc.Count))
	case pc.MinCount != nil && pc.MaxCount != nil:
		fmt.Fprintf(&s,
			"between %d and %d %s",
			*pc.MinCount, *pc.MaxCount,
			prop(*pc.MaxCount),
		)
	case pc.MaxCount != nil:
		fmt.Fprintf(&s,
			"less than %d %s",
			*pc.MaxCount, prop(*pc.MaxCount),
		)
	case pc.MinCount != nil:
		fmt.Fprintf(&s, "more than %d %s",
			*pc.MinCount, prop(*pc.MinCount),
		)
	}

	return s.String()
}

func checkProperty(
	prop doc.Property, constraintSets []PropertyConstraintMap,
	vCtx *ValidationContext, res []ValidationResult,
) ([]ValidationResult, bool) {
	var matched bool

	for _, pm := range constraintSets {
		pc, ok := pm[prop.Name]
		if !ok {
			continue
		}

		matched = true

		if pc.Value != nil {
			err := pc.Value.Validate(prop.Value, true, vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{{
						RefType: RefTypeProperty,
						Name:    prop.Name,
					}},
					Error: fmt.Sprintf("invalid value: %v",
						err),
				})
			}
		}

		for k, check := range pc.Parameters {
			var (
				v  string
				ok bool
			)

			if prop.Parameters != nil {
				v, ok = prop.Parameters[k]
			}

			err := check.Validate(v, ok, vCtx)
			if err != nil {
				res = append(res, ValidationResult{
					Entity: []EntityRef{
						{
							RefType: RefTypeParameter,
							Name:    k,
						},
						{
							RefType: RefTypeProperty,
							Name:    prop.Name,
						},
					},
					Error: err.Error(),
				})
			}
		}

		for k := range prop.Parameters {
			_, ok := pc.Parameters[k]
			if !ok {
				res = append(res, ValidationResult{
					Entity: []EntityRef{
						{
							RefType: RefTypeParameter,
							Name:    k,
						},
						{
							RefType: RefTypeProperty,
							Name:    prop.Name,
						},
					},
					Error: "undeclared parameter",
				})
			}
		}
	}

	if !matched {
		res = append(res, ValidationResult{
			Entity: []EntityRef{
				{
					RefType: RefTypeProperty,
					Name:    prop.Name,
				},
			},
			Error: "undeclared property",
		})
	}

	return res, matched
}
