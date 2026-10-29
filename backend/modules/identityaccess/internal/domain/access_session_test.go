package domain

import (
	"testing"
	"time"
)

func TestAccessSessionLive(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
	head := AccountSession{AccountID: 7, FamilyID: "family-a", PortalScope: PortalScopeTenant, Expiry: now.Add(time.Hour)}
	token := AccessSession{AccountID: 7, Scope: ScopeTenant, FamilyID: "family-a"}

	cases := []struct {
		name  string
		head  func(AccountSession) AccountSession
		token func(AccessSession) AccessSession
		want  bool
	}{
		{name: "live family backs the token", want: true},
		{name: "expired family", head: func(h AccountSession) AccountSession { h.Expiry = now; return h }},
		{name: "family retired by a switch after its grace", head: func(h AccountSession) AccountSession { h.Expiry = now.Add(-time.Second); return h }},
		{name: "family of another account", head: func(h AccountSession) AccountSession { h.AccountID = 8; return h }},
		{name: "another family", head: func(h AccountSession) AccountSession { h.FamilyID = "family-b"; return h }},
		{name: "token without family", token: func(a AccessSession) AccessSession { a.FamilyID = ""; return a }},
		{name: "token of another portal", token: func(a AccessSession) AccessSession { a.Scope = ScopeParent; return a }},
		{name: "legacy row without portal scope", head: func(h AccountSession) AccountSession { h.PortalScope = PortalScopeUnknown; return h }, want: true},
		{
			name:  "preview rests on the acting admin's family",
			token: func(a AccessSession) AccessSession { a.AccountID = 99; a.ActingAdminID = 7; return a },
			want:  true,
		},
		{
			name:  "preview whose admin does not own the family",
			token: func(a AccessSession) AccessSession { a.ActingAdminID = 8; return a },
		},
		{
			name:  "school portal family",
			head:  func(h AccountSession) AccountSession { h.PortalScope = PortalScopeSchool; return h },
			token: func(a AccessSession) AccessSession { a.Scope = PortalScopeSchool; return a },
			want:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, a := head, token
			if tc.head != nil {
				h = tc.head(h)
			}
			if tc.token != nil {
				a = tc.token(a)
			}
			if got := AccessSessionLive(h, a, now); got != tc.want {
				t.Fatalf("AccessSessionLive() = %v, want %v", got, tc.want)
			}
		})
	}
}
