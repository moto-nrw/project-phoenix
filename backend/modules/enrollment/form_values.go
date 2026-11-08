package enrollment

import (
	"bytes"
	"encoding/json"
	"maps"
)

// FormValues is one JSON object a family submits: the answers to the form
// (custom data of a request and of a child), the consent flags or the source
// metadata of a submission (ADR 0170). The keys follow the school's form
// schema, so the values have no fixed Go type; decoded from JSON they are
// float64, string, bool, nil, []any or map[string]any.
//
// The zero value is empty and ready to use. Empty values always encode as
// {}, never as null; null and an absent document decode as empty.
type FormValues struct {
	values map[string]any
}

// CollectFormValues builds values from key-value pairs (an
// iter.Seq2[string, any]), for example maps.All of a map the caller assembled.
func CollectFormValues(seq func(yield func(string, any) bool)) FormValues {
	var out FormValues
	for key, value := range seq {
		out.Set(key, value)
	}
	return out
}

// Lookup returns the value stored under key and whether the key is present.
func (v FormValues) Lookup(key string) (any, bool) {
	value, ok := v.values[key]
	return value, ok
}

// Len is the number of keys.
func (v FormValues) Len() int { return len(v.values) }

// IsZero reports whether no key is present. encoding/json's omitzero uses it.
func (v FormValues) IsZero() bool { return len(v.values) == 0 }

// All iterates the keys and values in unspecified order. It is an
// iter.Seq2[string, any].
func (v FormValues) All() func(yield func(string, any) bool) { return maps.All(v.values) }

// Set stores value under key.
func (v *FormValues) Set(key string, value any) {
	if v.values == nil {
		v.values = make(map[string]any)
	}
	v.values[key] = value
}

// Unset removes key.
func (v *FormValues) Unset(key string) { delete(v.values, key) }

// Clone returns a shallow copy: the keys are independent, nested values are
// shared, as with maps.Clone.
func (v FormValues) Clone() FormValues { return FormValues{values: maps.Clone(v.values)} }

// MarshalJSON encodes the values as a JSON object; empty values as {}.
func (v FormValues) MarshalJSON() ([]byte, error) {
	if len(v.values) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(v.values)
}

// UnmarshalJSON decodes a JSON object. null and empty input decode as empty
// values; any other JSON value is the *json.UnmarshalTypeError a map would
// report.
func (v *FormValues) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		v.values = nil
		return nil
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if len(decoded) == 0 {
		decoded = nil
	}
	v.values = decoded
	return nil
}
