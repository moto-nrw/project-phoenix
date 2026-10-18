package records

import "testing"

func TestProtocolRetentionTypesAreDataKinds(t *testing.T) {
	t.Parallel()
	types := ProtocolRetentionTypes()
	if len(types) != 10 {
		t.Fatalf("ProtocolRetentionTypes() has %d entries, want the ten protocols of #4044", len(types))
	}
	for _, kind := range types {
		if !IsDataKindRetention(kind) {
			t.Errorf("%s is not a data-kind retention", kind)
		}
	}
}
