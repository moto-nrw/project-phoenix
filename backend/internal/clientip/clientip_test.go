package clientip

import "testing"

func TestParseIPString(t *testing.T) {
	t.Parallel()

	if ParseIPString("") != nil {
		t.Fatal("empty IP must stay nil")
	}
	if ParseIPString("not-an-ip") != nil {
		t.Fatal("invalid IP must stay nil")
	}
	if got := ParseIPString("203.0.113.10"); got == nil || got.String() != "203.0.113.10" {
		t.Fatalf("ParseIPString() = %v, want 203.0.113.10", got)
	}
}
