package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The Identity & Access compositions of the command roots: the cleanup CLI's
// maintenance and the dev seeder's guardian access (#4006). None of them
// signs or verifies a session.

// NewInvitationMaintenance composes the school invitation maintenance the
// cleanup CLI runs. The invitation flows themselves stay unavailable: the CLI
// only deletes expired links (#2722).
func NewInvitationMaintenance(db *bun.DB) (identityaccess.SchoolInvitations, error) {
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	if err != nil {
		return nil, fmt.Errorf("invitation maintenance: %w", err)
	}
	return module, nil
}

// NewGuardianSeedAccess binds the dev seeder's guardian access command
// without composing sessions or token signers. The caller supplies its
// transaction and tenant through the context.
func NewGuardianSeedAccess(db *bun.DB) (func(ctx context.Context, email, passwordHash string) (int64, bool, error), error) {
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	if err != nil {
		return nil, err
	}
	return module.SeedGuardianAccount, nil
}

// SessionMaintenanceDependencies are the seams the expired session sweep
// and its revocation follow-ups need: the schools and persons a wipe names
// in its audit evidence, the audit ledger and the push subscriptions a
// revocation orphans. The sweep never signs or verifies a session, so it
// takes no signing secret.
type SessionMaintenanceDependencies struct {
	DB      *bun.DB
	Runtime tenant.UnitOfWork
	Schools identityPorts.SchoolDirectory
	Persons identityPorts.PersonDirectory
	Audit   identityPorts.AuthAudit
	Push    identityPorts.PushSubscriptionCleanup
	// ResetExpiry configures the reset flows; the sweep only deletes spent
	// links and stale rate-limit windows and never issues one.
	ResetExpiry time.Duration
	Logger      *slog.Logger
}

// SessionMaintenance is what the cleanup CLI and the Worker run: the expired
// session sweep with its revocation follow-ups (#3251) and the password
// reset maintenance (#3332).
type SessionMaintenance struct {
	identityaccess.AccountSessionMaintenance
	identityaccess.PasswordResets
}

// NewSessionMaintenance composes the maintenance without a session signer:
// every flow that would sign, verify or check a password reports itself
// unavailable.
func NewSessionMaintenance(deps SessionMaintenanceDependencies) (*SessionMaintenance, error) {
	if deps.Schools == nil || deps.Persons == nil || deps.Audit == nil || deps.Push == nil {
		return nil, errors.New("identity access compose: the session maintenance needs the schools, persons, audit and push seams")
	}
	module, err := New(Dependencies{
		DB: deps.DB, Observe: func(Observation) {}, TenantRuntime: deps.Runtime,
		Sessions: &identityPorts.SessionDependencies{
			Schools: deps.Schools, Persons: deps.Persons, Passwords: application.UnusablePasswords{},
			Codec: application.UnsignedSessions{}, MFALock: application.UnlockedMFAPolicy{}, Audit: deps.Audit, Push: deps.Push,
			Logger: deps.Logger,
		},
		Resets: &PasswordResetDependencies{
			DeliveryFor: func(identityaccess.PasswordResetDeliveryRecorder) identityPorts.PasswordResetDelivery {
				return application.UnsentPasswordResets{}
			},
			Passwords: application.UnusablePasswords{}, Expiry: deps.ResetExpiry, Logger: deps.Logger,
		},
	})
	if err != nil {
		return nil, err
	}
	return &SessionMaintenance{AccountSessionMaintenance: module, PasswordResets: module}, nil
}

func AccountExists(profiles identityaccess.AccountProfiles) func(context.Context, int64) (bool, error) {
	return application.AccountExists(profiles)
}
