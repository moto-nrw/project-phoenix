package enrollmenthttp_test

import (
	"encoding/json"
	"maps"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// testFormValues builds the owner's answers, consent flags or source
// metadata from a map literal (ADR 0170).
func testFormValues(values map[string]any) capability.FormValues {
	return capability.CollectFormValues(maps.All(values))
}

// testFormValue is the value stored under key, nil when absent.
func testFormValue(values capability.FormValues, key string) any {
	value, _ := values.Lookup(key)
	return value
}

// testJSONFormValues decodes a JSON object fixture into the owner's values.
func testJSONFormValues(raw []byte) capability.FormValues {
	var values capability.FormValues
	if err := json.Unmarshal(raw, &values); err != nil {
		panic(err)
	}
	return values
}
