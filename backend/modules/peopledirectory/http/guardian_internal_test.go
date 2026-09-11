package users

import (
	"strings"
	"testing"
)

// TestGuardianAccountStatus locks the per-child derivation of the staff-facing
// account status: an account only counts as "active" for a child when the
// link actually grants parent_portal.access, and an open invitation (a
// pending-approval role-upgrade request) outranks "active_no_access" (#2172).
func TestGuardianAccountStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name                                       string
		hasAccount, hasPortalAccess, invitePending bool
		want                                       string
	}{
		{"account with access", true, true, false, "active"},
		{"account without access on this child", true, false, false, "active_no_access"},
		{"account without access, pending upgrade approval", true, false, true, "pending"},
		{"no account, open invitation", false, false, true, "pending"},
		{"no account, no invitation", false, false, false, "none"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := guardianAccountStatus(tc.hasAccount, tc.hasPortalAccess, tc.invitePending)
			if got != tc.want {
				t.Fatalf("guardianAccountStatus(%v, %v, %v) = %q, want %q", tc.hasAccount, tc.hasPortalAccess, tc.invitePending, got, tc.want)
			}
		})
	}
}

// TestGuardianFullDeleteWarning locks the German wording of the full-delete
// blast-radius warning (#819): it must read as a warning about what the
// delete WOULD do, because the same text backs the preview and the admin 409.
func TestGuardianFullDeleteWarning(t *testing.T) {
	t.Parallel()

	t.Run("no links", func(t *testing.T) {
		if msg := guardianFullDeleteWarning(nil); !strings.Contains(msg, "keinem Kind") {
			t.Fatalf("expected a no-links message, got %q", msg)
		}
	})

	t.Run("single link does not list the child", func(t *testing.T) {
		msg := guardianFullDeleteWarning([]string{"Anna Müller"})
		if !strings.Contains(msg, "nur mit diesem Kind") {
			t.Fatalf("expected the single-link reassurance, got %q", msg)
		}
		if strings.Contains(msg, "Anna Müller") {
			t.Fatalf("the sole child must not be listed, got %q", msg)
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

func TestPaymentExportRequestBindNormalizesFormat(t *testing.T) {
	t.Parallel()

	for input, want := range map[string]string{"": "pdf", " XLSX ": "xlsx", "docx": "docx"} {
		request := &PaymentExportRequest{Format: input}
		if err := request.Bind(nil); err != nil {
			t.Fatalf("format %q: unexpected error %v", input, err)
		}
		if request.Format != want {
			t.Fatalf("format %q normalized to %q, want %q", input, request.Format, want)
		}
	}
	if err := (&PaymentExportRequest{Format: "csv"}).Bind(nil); err == nil {
		t.Fatal("csv must be refused before any row is loaded")
	}
}
