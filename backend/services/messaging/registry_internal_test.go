package messaging

import "testing"

// TestRequestRegistryComplete guards the single-source-of-truth contract of
// parentMessageRequestRegistry (see requestHandler): every registered request
// type must declare ALL of its behavior. A new type added with only an apply
// func — the historical failure mode — would silently no-op in validation,
// diff, audit and the parent-facing body; this turns that into a test failure.
func TestRequestRegistryComplete(t *testing.T) {
	if len(parentMessageRequestRegistry) == 0 {
		t.Fatal("parentMessageRequestRegistry is empty")
	}
	for reqType, h := range parentMessageRequestRegistry {
		if h.requestBody == "" {
			t.Errorf("request type %q: missing requestBody", reqType)
		}
		if h.confirmBody == "" {
			t.Errorf("request type %q: missing confirmBody", reqType)
		}
		if h.apply == nil {
			t.Errorf("request type %q: missing apply", reqType)
		}
		if h.validate == nil {
			t.Errorf("request type %q: missing validate", reqType)
		}
		if h.canonicalize == nil {
			t.Errorf("request type %q: missing canonicalize", reqType)
		}
		if h.diff == nil {
			t.Errorf("request type %q: missing diff", reqType)
		}
		if h.broadcastChanges == nil {
			t.Errorf("request type %q: missing broadcastChanges", reqType)
		}
		if h.auditSummary == nil {
			t.Errorf("request type %q: missing auditSummary", reqType)
		}
	}
}
