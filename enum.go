package revisor

import (
	"errors"
	"fmt"
	"strings"
)

type Enum struct {
	Declare     string                    `json:"declare,omitempty"`
	Match       string                    `json:"match,omitempty"`
	Name        string                    `json:"name,omitempty"`
	Description string                    `json:"description,omitempty"`
	Values      map[string]EnumConstraint `json:"values"`
}

type EnumConstraint struct {
	Forbidden  bool         `json:"forbidden,omitempty"`
	Deprecated *Deprecation `json:"deprecated,omitempty"`
}

type mergedEnum struct {
	Values  map[string][]EnumConstraint
	Allowed []string
}

func mergedEnumAllowedValues(m *mergedEnum) []string {
	var vals []string

	valueFn := func(v string, cs []EnumConstraint) (string, bool) {
		var deprecated bool

		for _, c := range cs {
			if c.Forbidden {
				return "", false
			}

			deprecated = deprecated || c.Deprecated != nil
		}

		if deprecated {
			return fmt.Sprintf("%q (deprecated)", v), true
		}

		return fmt.Sprintf("%q", v), true
	}

	for v, cs := range m.Values {
		s, ok := valueFn(v, cs)
		if ok {
			vals = append(vals, s)
		}
	}

	return vals
}

type enumSet struct {
	extensions []Enum
	enums      map[string]*mergedEnum
}

func newEnumSet() *enumSet {
	return &enumSet{
		enums: make(map[string]*mergedEnum),
	}
}

func (s *enumSet) Register(e Enum) error {
	if e.Declare != "" && e.Match != "" {
		return fmt.Errorf(
			"the enum %q cannot both declare and match an enum",
			e.Declare)
	}

	if e.Declare == "" && e.Match == "" {
		return errors.New("an enum must declare or match an existing enum")
	}

	if e.Match != "" {
		s.extensions = append(s.extensions, e)

		return nil
	}

	_, declared := s.enums[e.Declare]

	if declared {
		return errors.New("the enum %q has already been declared")
	}

	m := mergedEnum{
		Values: make(map[string][]EnumConstraint, len(e.Values)),
	}

	for k, c := range e.Values {
		m.Values[k] = []EnumConstraint{c}
	}

	s.enums[e.Declare] = &m

	return nil
}

func (s *enumSet) Resolve() error {
	for _, e := range s.extensions {
		m, declared := s.enums[e.Match]
		if !declared {
			return fmt.Errorf("the enum %q hasn't been declared and cannot be matched", e.Match)
		}

		for k, c := range e.Values {
			m.Values[k] = append(m.Values[k], c)
		}
	}

	for _, m := range s.enums {
		m.Allowed = mergedEnumAllowedValues(m)
	}

	return nil
}

func (s *enumSet) ValidValue(enum string, value string) (*Deprecation, error) {
	m, declared := s.enums[enum]
	if !declared {
		return nil, fmt.Errorf("unknown enum %q", enum)
	}

	constraints, hasValue := m.Values[value]
	if !hasValue {
		return nil, fmt.Errorf("must be one of: %s", strings.Join(m.Allowed, ", "))
	}

	var deprecation *Deprecation

	for _, c := range constraints {
		if c.Deprecated != nil && deprecation == nil {
			deprecation = c.Deprecated
		}

		if c.Forbidden {
			return nil, fmt.Errorf("%q is no longer allowed", value)
		}
	}

	return deprecation, nil
}
