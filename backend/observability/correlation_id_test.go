package observability

import "testing"

func TestCorrelationIDFromRequest_PreservesProvidedValue(t *testing.T) {
	t.Parallel()

	const requestValue = "8dc3a9ca-8ac7-4b8e-9bfa-3c17760d92c0"
	id, err := CorrelationIDFromRequest(requestValue)
	if err != nil {
		t.Fatalf("CorrelationIDFromRequest() error = %v", err)
	}
	if got := id.String(); got != requestValue {
		t.Fatalf("CorrelationIDFromRequest() = %q, want %q", got, requestValue)
	}
}

func TestCorrelationIDFromRequest_ReplacesUntrustedValue(t *testing.T) {
	t.Parallel()

	id, err := CorrelationIDFromRequest("student-Erika-Mustermann")
	if err != nil {
		t.Fatalf("CorrelationIDFromRequest() error = %v", err)
	}
	if id.String() == "" || id.String() == "student-Erika-Mustermann" {
		t.Fatalf("CorrelationIDFromRequest() = %q, want generated UUID", id.String())
	}
}

func TestCorrelationIDFromRequest_GeneratesMissingValue(t *testing.T) {
	t.Parallel()

	first, err := CorrelationIDFromRequest("")
	if err != nil {
		t.Fatalf("first CorrelationIDFromRequest() error = %v", err)
	}
	second, err := CorrelationIDFromRequest("")
	if err != nil {
		t.Fatalf("second CorrelationIDFromRequest() error = %v", err)
	}
	if first.String() == "" || second.String() == "" {
		t.Fatal("generated correlation IDs must not be empty")
	}
	if first == second {
		t.Fatalf("generated correlation IDs collided: %q", first)
	}
}
