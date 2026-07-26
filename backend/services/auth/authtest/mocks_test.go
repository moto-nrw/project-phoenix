package authtest

import (
	"context"
	"testing"
)

func TestInvitationServiceMockGetTenantSubdomainForToken(t *testing.T) {
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
