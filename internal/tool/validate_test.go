package tool

import (
	"encoding/json"
	"testing"
)

func TestFieldValidatorRequired(t *testing.T) {
	v := &FieldValidator{
		Rules: []FieldRule{
			{Field: "name", Required: true},
		},
	}

	// Present.
	if err := v.Validate(json.RawMessage(`{"name": "test"}`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	// Missing.
	if err := v.Validate(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for missing required field")
	}

	// Null.
	if err := v.Validate(json.RawMessage(`{"name": null}`)); err == nil {
		t.Fatal("expected error for null required field")
	}
}

func TestFieldValidatorEnum(t *testing.T) {
	v := &FieldValidator{
		Rules: []FieldRule{
			{Field: "type", Enum: []string{"a", "b", "c"}},
		},
	}

	// Valid.
	if err := v.Validate(json.RawMessage(`{"type": "a"}`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	// Invalid.
	if err := v.Validate(json.RawMessage(`{"type": "z"}`)); err == nil {
		t.Fatal("expected error for invalid enum value")
	}

	// Absent (optional) — should pass.
	if err := v.Validate(json.RawMessage(`{}`)); err != nil {
		t.Fatalf("expected pass for absent optional enum, got %v", err)
	}
}

func TestFieldValidatorRange(t *testing.T) {
	v := &FieldValidator{
		Rules: []FieldRule{
			{Field: "count", MinInt: 1, MaxInt: 100},
		},
	}

	if err := v.Validate(json.RawMessage(`{"count": 50}`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := v.Validate(json.RawMessage(`{"count": 0}`)); err == nil {
		t.Fatal("expected error for below minimum")
	}
	if err := v.Validate(json.RawMessage(`{"count": 101}`)); err == nil {
		t.Fatal("expected error for above maximum")
	}
}

func TestFieldValidatorMaxLen(t *testing.T) {
	v := &FieldValidator{
		Rules: []FieldRule{
			{Field: "desc", MaxLen: 10},
		},
	}

	if err := v.Validate(json.RawMessage(`{"desc": "short"}`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := v.Validate(json.RawMessage(`{"desc": "this is way too long"}`)); err == nil {
		t.Fatal("expected error for string exceeding max length")
	}
}

func TestFieldValidatorNestedField(t *testing.T) {
	v := &FieldValidator{
		Rules: []FieldRule{
			{Field: "options.color", Required: true, Enum: []string{"red", "blue"}},
		},
	}

	if err := v.Validate(json.RawMessage(`{"options": {"color": "red"}}`)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}
	if err := v.Validate(json.RawMessage(`{"options": {"color": "green"}}`)); err == nil {
		t.Fatal("expected error for invalid nested enum")
	}
}

func TestFieldValidatorCombined(t *testing.T) {
	v := &FieldValidator{
		Rules: []FieldRule{
			{Field: "name", Required: true, MaxLen: 20},
			{Field: "type", Required: true, Enum: []string{"read", "write"}},
			{Field: "count", MinInt: 0, MaxInt: 1000},
		},
	}

	valid := `{"name": "test", "type": "read", "count": 5}`
	if err := v.Validate(json.RawMessage(valid)); err != nil {
		t.Fatalf("expected pass, got %v", err)
	}

	noName := `{"type": "read"}`
	if err := v.Validate(json.RawMessage(noName)); err == nil {
		t.Fatal("expected error for missing name")
	}

	badType := `{"name": "test", "type": "delete"}`
	if err := v.Validate(json.RawMessage(badType)); err == nil {
		t.Fatal("expected error for bad type")
	}
}
