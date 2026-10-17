package parentstore

import (
	"bytes"
	"encoding/json"
)

func portalRecordPointer[A, B any](v *A, convert func(A) B) *B {
	if v == nil {
		return nil
	}
	out := convert(*v)
	return &out
}
func portalRecordSlice[A, B any](v []A, convert func(A) B) []B {
	if v == nil {
		return nil
	}
	out := make([]B, len(v))
	for i, row := range v {
		out[i] = convert(row)
	}
	return out
}
func portalRecordMap[K comparable, A, B any](v map[K]A, convert func(A) B) map[K]B {
	if v == nil {
		return nil
	}
	out := make(map[K]B, len(v))
	for k, row := range v {
		out[k] = convert(row)
	}
	return out
}

// Typed JSON objects retain map presence and omitempty, without float conversion
// on the native write path. Read values preserve the original decoded maps.
func portalPayloadFromRetained(v map[string]any) (map[string]json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]json.RawMessage
	err = json.Unmarshal(raw, &out)
	return out, err
}
func portalPayloadToRetained(v map[string]json.RawMessage) (map[string]any, error) {
	if v == nil {
		return nil, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	err = decoder.Decode(&out)
	return out, err
}
