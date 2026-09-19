package auth

import (
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupInternalAuthService composes the retained service over the real
// repositories with the session port doubled: the role and link flows under
// test persist their rows through the real transaction runtime, while the
// revocation they request is recorded by the stub instead of reaching the
// Identity & Access owner (#3251).
func setupInternalAuthService(t *testing.T, db *bun.DB) *Service {
	t.Helper()
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	cfg, err := NewServiceConfig(nil, email.Email{}, "http://localhost:3000", time.Hour)
	require.NoError(t, err)
	cfg.Audit = testpkg.NewAuthEventCommand(repoFactory.AuthEvent)
	sessions := newStubAccountSessions()
	sessions.repos = repoFactory
	sessions.sessions, err = repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	cfg.Sessions = sessions
	cfg.Lifecycle = sessions
	service, err := NewService(repoFactory, cfg, db, slog.Default())
	require.NoError(t, err)
	testpkg.SetTenantRuntime(t, service, db)
	return service
}
