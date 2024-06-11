package revisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type StringFormat string

const (
	StringFormatNone    StringFormat = ""
	StringFormatRFC3339 StringFormat = "RFC3339"
	StringFormatInt     StringFormat = "int"
	StringFormatFloat   StringFormat = "float"
	StringFormatBoolean StringFormat = "bool"
	StringFormatHTML    StringFormat = "html"
	StringFormatUUID    StringFormat = "uuid"
	StringFormatWKT     StringFormat = "wkt"
)

func (f StringFormat) Describe() string {
	switch f {
	case StringFormatRFC3339:
		return "a RFC3339 timestamp"
	case StringFormatInt:
		return "a integer value"
	case StringFormatFloat:
		return "a float value"
	case StringFormatBoolean:
		return "a boolean"
	case StringFormatHTML:
		return "a html string"
	case StringFormatUUID:
		return "a uuid"
	case StringFormatWKT:
		return "a WKT geometry"
	case StringFormatNone:
		return ""
	}

	return ""
}

type ConstraintMap struct {
	Keys        []string
	Constraints map[string]StringConstraint
}

func (cm ConstraintMap) Requirements() string {
	var requirements []string

	for _, k := range cm.Keys {
		requirements = append(requirements,
			fmt.Sprintf("%s %s", k, cm.Constraints[k].Requirement()),
		)
	}

	return strings.Join(requirements, "; and ")
}

func (cm *ConstraintMap) UnmarshalJSON(data []byte) error {
	clear(cm.Constraints)

	err := json.Unmarshal(data, &cm.Constraints)
	if err != nil {
		return fmt.Errorf("unmarshal map: %w", err)
	}

	keys := make([]string, 0, len(cm.Constraints))

	for k := range cm.Constraints {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	cm.Keys = keys

	return nil
}

func (cm *ConstraintMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(cm.Constraints)
}

type StringConstraint struct {
	Name        string       `json:"name,omitempty"`
	Description string       `json:"description,omitempty"`
	Optional    bool         `json:"optional,omitempty"`
	AllowEmpty  bool         `json:"allowEmpty,omitempty"`
	Const       *string      `json:"const,omitempty"`
	Enum        []string     `json:"enum,omitempty"`
	EnumRef     string       `json:"enumReference,omitempty"`
	Pattern     *Regexp      `json:"pattern,omitempty"`
	Glob        GlobList     `json:"glob,omitempty"`
	Format      StringFormat `json:"format,omitempty"`
	Time        string       `json:"time,omitempty"`
	Geometry    string       `json:"geometry,omitempty"`
	HTMLPolicy  string       `json:"htmlPolicy,omitempty"`
	Deprecated  *Deprecation `json:"deprecated,omitempty"`

	// Labels (and hints) are not constraints per se, but should be seen as
	// labels on the value that can be used by systems that process data
	// with the help of revisor schemas.
	Labels []string            `json:"labels,omitempty"`
	Hints  map[string][]string `json:"hints,omitempty"`
}

func (sc StringConstraint) Requirement() string {
	var reqs []string

	if sc.Const != nil {
		reqs = append(reqs, fmt.Sprintf("is %q", *sc.Const))
	}

	if len(sc.Enum) > 0 {
		reqs = append(reqs, fmt.Sprintf("is one of: %s",
			strings.Join(sc.Enum, ", "),
		))
	}

	if sc.Pattern != nil {
		reqs = append(reqs, fmt.Sprintf("matches regexp: %s",
			sc.Pattern.String()),
		)
	}

	if len(sc.Glob) > 0 {
		reqs = append(reqs, fmt.Sprintf("matches one of the glob patterns: %s",
			sc.Glob.String(),
		))
	}

	if sc.Time != "" {
		reqs = append(reqs, fmt.Sprintf("is a timestamp in the format: %s",
			sc.Time),
		)
	}

	if sc.Format != StringFormatNone {
		reqs = append(reqs, fmt.Sprintf("is a %s", sc.Format.Describe()))
	}

	return strings.Join(reqs, " and ")
}

type ValidationContext struct {
	coll ValueCollector
	depr DeprecationHandlerFunc

	ValidateHTML func(policyName, value string) error
	ValidateEnum func(enum string, value string) (*Deprecation, error)
}

func (sc *StringConstraint) Validate(
	value string, exists bool, vCtx *ValidationContext,
) (*Deprecation, error) {
	if !exists {
		if sc.Optional {
			return nil, nil //nolint: nilnil
		}

		return nil, errors.New("required value")
	}

	if sc.AllowEmpty && value == "" {
		return nil, nil //nolint: nilnil
	}

	if sc.Const != nil && value != *sc.Const {
		return nil, fmt.Errorf("must be %q", *sc.Const)
	}

	if len(sc.Enum) > 0 {
		var match bool

		for i := range sc.Enum {
			match = match || sc.Enum[i] == value
		}

		if !match {
			return nil, fmt.Errorf("must be one of: %s",
				strings.Join(sc.Enum, ", "))
		}
	}

	var deprecation *Deprecation

	if sc.EnumRef != "" {
		depr, err := vCtx.ValidateEnum(sc.EnumRef, value)
		if err != nil {
			return nil, err
		}

		deprecation = depr
	}

	if !sc.Glob.MatchOrEmpty(value) {
		return nil, errors.New(sc.Glob.String())
	}

	if sc.Pattern != nil && !sc.Pattern.Match(value) {
		return nil, fmt.Errorf("%q must match %q", value, sc.Pattern.String())
	}

	if sc.Time != "" {
		_, err := time.Parse(sc.Time, value)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %w", err)
		}
	}

	switch sc.Format {
	case StringFormatNone:
	case StringFormatRFC3339:
		_, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, fmt.Errorf("invalid RFC3339 value: %w", err)
		}
	case StringFormatInt:
		_, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.New("invalid integer value")
		}
	case StringFormatFloat:
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, errors.New("invalid float value")
		}
	case StringFormatBoolean:
		_, err := strconv.ParseBool(value)
		if err != nil {
			return nil, errors.New("invalid boolean value")
		}
	case StringFormatHTML:
		if vCtx == nil || vCtx.ValidateHTML == nil {
			return nil, errors.New("html validation is not available in this context")
		}

		return nil, vCtx.ValidateHTML(sc.HTMLPolicy, value)
	case StringFormatUUID:
		_, err := uuid.Parse(value)
		if err != nil {
			return nil, errors.New("invalid uuid value")
		}
	case StringFormatWKT:
		err := validateWKT(sc.Geometry, value)
		if err != nil {
			return nil, fmt.Errorf("WKT validation: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown string format %q", sc.Format)
	}

	if !sc.AllowEmpty && value == "" {
		return nil, fmt.Errorf("cannot be empty")
	}

	return deprecation, nil
}
