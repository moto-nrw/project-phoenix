package activities

import (
	"encoding/json"
	"testing"
)

func TestDateWireFormatAndOrdering(t *testing.T) {
	t.Parallel()

	date := Date("2026-03-28")
	if got := date.AddDays(2); got != Date("2026-03-30") {
		t.Fatalf("AddDays() = %q", got)
	}
	if !date.Before(Date("2026-03-29")) || !Date("2026-03-29").After(date) {
		t.Fatal("date ordering is wrong")
	}

	encoded, err := json.Marshal(date)
	if err != nil || string(encoded) != `"2026-03-28"` {
		t.Fatalf("Marshal() = %s, %v", encoded, err)
	}
	var decoded Date
	if err := json.Unmarshal(encoded, &decoded); err != nil || decoded != date {
		t.Fatalf("Unmarshal() = %q, %v", decoded, err)
	}
	if err := json.Unmarshal([]byte(`"2026-3-28"`), &decoded); err == nil {
		t.Fatal("Unmarshal() accepted a non-canonical date")
	}
}
