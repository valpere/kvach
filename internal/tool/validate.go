package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Validatable is an optional interface that tools can implement to provide
// richer validation beyond basic JSON Schema checks. The dispatcher calls
// ValidateOutput after a successful Call to verify the tool's output is
// well-formed. If validation fails, the error is fed back to the LLM as a
// retry hint.
type Validatable interface {
	// ValidateOutput checks the tool result content after Call returns. A
	// non-nil error triggers "LLM Theater" recovery — the agent tells the
	// model what went wrong and asks it to try again.
	ValidateOutput(result *Result) error
}

// FieldRule is a single constraint for a named field in a JSON object.
type FieldRule struct {
	// Field is the dot-separated path to the JSON field (e.g. "type" or
	// "options.count").
	Field string
	// Required means the field must be present and non-null.
	Required bool
	// Enum restricts the field to one of the listed string values.
	Enum []string
	// MinInt and MaxInt bound numeric fields (inclusive). Both zero means
	// unconstrained.
	MinInt int64
	MaxInt int64
	// MaxLen bounds string length. Zero means unconstrained.
	MaxLen int
}

// FieldValidator validates decoded JSON tool arguments against a list of
// field rules. This catches "LLM Theater" — the model returns structurally
// valid JSON with semantically invalid values.
type FieldValidator struct {
	Rules []FieldRule
}

// Validate checks the raw JSON against all rules. Returns the first
// validation failure encountered.
func (v *FieldValidator) Validate(raw json.RawMessage) error {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return fmt.Errorf("expected JSON object: %w", err)
	}

	for _, rule := range v.Rules {
		if err := validateField(obj, rule); err != nil {
			return err
		}
	}
	return nil
}

func validateField(obj map[string]json.RawMessage, rule FieldRule) error {
	val, exists := resolveField(obj, rule.Field)

	// Required check.
	if rule.Required && (!exists || isJSONNull(val)) {
		return fmt.Errorf("field %q is required", rule.Field)
	}
	if !exists || isJSONNull(val) {
		return nil // optional and absent, skip further checks
	}

	// Enum check.
	if len(rule.Enum) > 0 {
		var s string
		if err := json.Unmarshal(val, &s); err != nil {
			return fmt.Errorf("field %q: expected string for enum check: %w", rule.Field, err)
		}
		found := false
		for _, allowed := range rule.Enum {
			if s == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("field %q: value %q not in allowed values %v", rule.Field, s, rule.Enum)
		}
	}

	// Integer range check.
	if rule.MinInt != 0 || rule.MaxInt != 0 {
		var n int64
		if err := json.Unmarshal(val, &n); err != nil {
			var f float64
			if err2 := json.Unmarshal(val, &f); err2 != nil {
				return fmt.Errorf("field %q: expected number: %w", rule.Field, err)
			}
			n = int64(f)
		}
		if rule.MinInt != 0 && n < rule.MinInt {
			return fmt.Errorf("field %q: value %d below minimum %d", rule.Field, n, rule.MinInt)
		}
		if rule.MaxInt != 0 && n > rule.MaxInt {
			return fmt.Errorf("field %q: value %d above maximum %d", rule.Field, n, rule.MaxInt)
		}
	}

	// String length check.
	if rule.MaxLen > 0 {
		var s string
		if err := json.Unmarshal(val, &s); err == nil && len(s) > rule.MaxLen {
			return fmt.Errorf("field %q: length %d exceeds maximum %d", rule.Field, len(s), rule.MaxLen)
		}
	}

	return nil
}

// resolveField gets a field value from a flat JSON object by a dot-separated
// path. Only supports one level of nesting for simplicity.
func resolveField(obj map[string]json.RawMessage, field string) (json.RawMessage, bool) {
	parts := strings.SplitN(field, ".", 2)
	val, ok := obj[parts[0]]
	if !ok || len(parts) == 1 {
		return val, ok
	}

	// One level of nesting.
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(val, &nested); err != nil {
		return nil, false
	}
	val, ok = nested[parts[1]]
	return val, ok
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}
