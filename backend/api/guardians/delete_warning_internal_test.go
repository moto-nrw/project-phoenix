package guardians

import (
	"strings"
	"testing"
)

// TestGuardianFullDeleteWarning locks the German wording of the full-delete
// blast-radius warning (#819). It must read as a warning about what the delete
// WOULD do — never as if the deletion already happened — because the same text
// backs both the pre-confirmation preview and the admin 409 body.
func TestGuardianFullDeleteWarning(t *testing.T) {
	t.Parallel()

	t.Run("no links", func(t *testing.T) {
		msg := guardianFullDeleteWarning(nil)
		if !strings.Contains(msg, "keinem Kind") {
			t.Fatalf("expected a no-links message, got %q", msg)
		}
	})

	t.Run("single link does not list the child", func(t *testing.T) {
		msg := guardianFullDeleteWarning([]string{"Anna Müller"})
		if !strings.Contains(msg, "nur mit diesem Kind") {
			t.Fatalf("expected the single-link reassurance, got %q", msg)
		}
		if strings.Contains(msg, "Anna Müller") {
			t.Fatalf("the sole child (the one the caller came from) must not be listed, got %q", msg)
		}
	})

	t.Run("multiple links names every affected child", func(t *testing.T) {
		msg := guardianFullDeleteWarning([]string{"Anna Müller", "Ben Müller"})
		for _, name := range []string{"Anna Müller", "Ben Müller"} {
			if !strings.Contains(msg, name) {
				t.Fatalf("expected %q in the multi-link warning, got %q", name, msg)
			}
		}
		if !strings.Contains(msg, "2 Kindern") {
			t.Fatalf("expected the affected count in the multi-link warning, got %q", msg)
		}
	})
}
