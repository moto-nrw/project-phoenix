package authtest

import (
	"context"
	"net"
	"testing"

	auth "github.com/moto-nrw/project-phoenix/services/auth"
)

func TestMFAServiceMockVerifyMFAChallengeForScope(t *testing.T) {
	t.Parallel()

	t.Run("delegates to configured function", func(t *testing.T) {
		t.Parallel()

		const (
			token = "school-challenge-token"
			code  = "123456"
			scope = "school"
		)
		expected := auth.VerifiedMFAChallenge{AccountID: 7, Scope: scope, TenantID: 3}

		mock := &MFAServiceMock{
			VerifyMFAChallengeForScopeFn: func(_ context.Context, actualToken, actualCode, actualScope string) (auth.VerifiedMFAChallenge, error) {
				if actualToken != token || actualCode != code || actualScope != scope {
					t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", actualToken, actualCode, actualScope, token, code, scope)
				}
				return expected, nil
			},
		}

		got, err := mock.VerifyMFAChallengeForScope(context.Background(), token, code, scope)
		if err != nil {
			t.Fatalf("VerifyMFAChallengeForScope() error = %v", err)
		}
		if got != expected {
			t.Fatalf("VerifyMFAChallengeForScope() = %v, want %v", got, expected)
		}
	})

	t.Run("returns zero values without configured function", func(t *testing.T) {
		t.Parallel()

		mock := &MFAServiceMock{}

		got, err := mock.VerifyMFAChallengeForScope(context.Background(), "token", "123456", "school")
		if got != (auth.VerifiedMFAChallenge{}) || err != nil {
			t.Fatalf("VerifyMFAChallengeForScope() = (%v, %v), want (zero, nil)", got, err)
		}
	})
}

func TestMFAServiceMockResendMFAChallengeForScope(t *testing.T) {
	t.Parallel()

	t.Run("delegates to configured function", func(t *testing.T) {
		t.Parallel()

		const (
			token   = "school-challenge-token"
			scope   = "school"
			renewed = "renewed-school-challenge-token"
		)
		ip := net.ParseIP("203.0.113.7")

		mock := &MFAServiceMock{
			ResendMFAChallengeForScopeFn: func(_ context.Context, actualToken string, actualIP net.IP, actualScope string) (string, error) {
				if actualToken != token || actualScope != scope || !actualIP.Equal(ip) {
					t.Errorf("got (%q, %v, %q), want (%q, %v, %q)", actualToken, actualIP, actualScope, token, ip, scope)
				}
				return renewed, nil
			},
		}

		got, err := mock.ResendMFAChallengeForScope(context.Background(), token, ip, scope)
		if err != nil {
			t.Fatalf("ResendMFAChallengeForScope() error = %v", err)
		}
		if got != renewed {
			t.Fatalf("ResendMFAChallengeForScope() = %q, want %q", got, renewed)
		}
	})

	t.Run("returns zero values without configured function", func(t *testing.T) {
		t.Parallel()

		mock := &MFAServiceMock{}

		got, err := mock.ResendMFAChallengeForScope(context.Background(), "token", net.ParseIP("203.0.113.7"), "school")
		if got != "" || err != nil {
			t.Fatalf("ResendMFAChallengeForScope() = (%q, %v), want (\"\", nil)", got, err)
		}
	})
}

func TestInvitationServiceMockGetTenantSubdomainForToken(t *testing.T) {
	t.Parallel()

	t.Run("delegates to configured function", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		const (
			token             = "invitation-token"
			expectedSubdomain = "rheinland"
		)

		mock := &InvitationServiceMock{
			GetTenantSubdomainForTokenFn: func(actualCtx context.Context, actualToken string) string {
				if actualCtx != ctx {
					t.Errorf("context = %v, want configured context", actualCtx)
				}
				if actualToken != token {
					t.Errorf("token = %q, want %q", actualToken, token)
				}
				return expectedSubdomain
			},
		}

		if got := mock.GetTenantSubdomainForToken(ctx, token); got != expectedSubdomain {
			t.Fatalf("GetTenantSubdomainForToken() = %q, want %q", got, expectedSubdomain)
		}
	})

	t.Run("returns empty string without configured function", func(t *testing.T) {
		t.Parallel()

		mock := &InvitationServiceMock{}

		if got := mock.GetTenantSubdomainForToken(context.Background(), "invitation-token"); got != "" {
			t.Fatalf("GetTenantSubdomainForToken() = %q, want empty string", got)
		}
	})
}
