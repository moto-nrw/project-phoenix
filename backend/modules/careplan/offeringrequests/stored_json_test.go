package offeringrequests

import (
	"encoding/json"
	"testing"
)

func TestCheckStoredJSONRejectsDocumentsThatDoNotDecode(t *testing.T) {
	t.Parallel()
	for name, request := range map[string]Request{
		"empty": {}, "null": {Payload: json.RawMessage(`null`), DecisionSnapshot: json.RawMessage(`null`)},
		"objects": {Payload: json.RawMessage(`{"a":1}`), DecisionSnapshot: json.RawMessage(`{}`)},
	} {
		if err := request.CheckStoredJSON(); err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
	if err := (Request{Payload: json.RawMessage(`[1]`)}).CheckStoredJSON(); err == nil {
		t.Error("a payload that is not an object must be rejected")
	}
	if err := (Request{DecisionSnapshot: json.RawMessage(`{`)}).CheckStoredJSON(); err == nil {
		t.Error("a broken snapshot must be rejected")
	}
}
