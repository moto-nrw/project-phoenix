package domain

import (
	"bytes"
	"encoding/json"
)

// Typed JSON objects retain map presence and omitempty, without float conversion
// on the native write path. Read values preserve the original decoded maps.
func ParentPayloadFromFacts(v map[string]any) (map[string]json.RawMessage, error) {
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
func ParentPayloadToFacts(v map[string]json.RawMessage) (map[string]any, error) {
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
