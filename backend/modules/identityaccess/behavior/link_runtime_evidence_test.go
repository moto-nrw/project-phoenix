package behavior_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// TestLinkRuntimeEvidence records the runtime evidence for the one-time link
// cutover of #2722 — school invitations, password resets and guardian
// invitations — on an isolated clone: statement counts, latency, affected
// rows, pool waits, unit-of-work outcomes, deadlocks and the replay refusals
// per operation. It drives the flows through the composed root the way the
// routes do; the raw JSON is logged for the migration ticket.
func TestLinkRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	repoFactory := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	factory, err := services.NewFactoryForTests(repoFactory, db, slog.Default())
	require.NoError(t, err)
	require.NoError(t, factory.SetTenantRuntime(testpkg.TenantRuntime(t, db)))

	inviter := testpkg.CreateTestAccount(t, db, "link-evidence-inviter")
	role := testpkg.CreateTestRole(t, db, "link-evidence")

	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)))
	// The public accept and reset routes carry no tenant; they are measured
	// the way they run, on a counted but tenant-less context.
	publicCtx := counter.Context(testpkg.WithTenantRuntime(t, context.Background(), db))
	var version string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &version))
	deadlocks := func() int64 {
		var count int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &count))
		return count
	}
	beforeDeadlocks := deadlocks()

	samples := map[string][]testpkg.RuntimeCheckpointSample{}
	var failures []string
	replays := map[string]int{}
	const warmup = 3
	const iterations = 13
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := db.Stats()
		if iteration < warmup {
			require.NoError(t, err, "warmup %s", operation)
			return
		}
		sample := testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			PoolWaitCount: after.WaitCount - before.WaitCount,
			PoolWaitMS:    float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
		}
		if err != nil {
			sample.Status = 1
			sample.ErrorBody = err.Error()
			failures = append(failures, operation+": "+err.Error())
		} else {
			writes := counter.WriteRows()
			rows, statements := counter.Rows()
			sample.WriteRowsAffected = &writes
			sample.RowsAffected = rows
			sample.StatementsWithRows = statements
		}
		samples[operation] = append(samples[operation], sample)
	}

	for iteration := range iterations {
		// --- school invitation: issue, preview, redeem once, resend, revoke.
		staffAddress := inviteeAddress(fmt.Sprintf("link-evidence-staff-%d", iteration))
		var staffInvitationID int64
		var staffToken string
		measure("school_invitation_create", iteration, func() error {
			created, createErr := factory.Invitation.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
				Email: staffAddress, RoleID: role.ID, CreatedBy: inviter.ID,
				FirstName: testpkg.StrPtr("Evi"), LastName: testpkg.StrPtr("Dence"),
				ActorPermissions: []string{usersManagePermission},
			})
			if createErr != nil {
				return createErr
			}
			staffInvitationID, staffToken = created.ID, created.Token
			return nil
		})
		measure("school_invitation_validate", iteration, func() error {
			_, validateErr := factory.Invitation.ValidateSchoolInvitation(publicCtx, staffToken)
			return validateErr
		})
		measure("school_invitation_resend", iteration, func() error {
			return factory.Invitation.ResendSchoolInvitation(ctx, staffInvitationID, inviter.ID)
		})
		measure("school_invitation_accept", iteration, func() error {
			account, acceptErr := factory.Invitation.AcceptSchoolInvitation(publicCtx, staffToken, identityaccess.InvitationRegistration{
				Password: invitationPassword, ConfirmPassword: invitationPassword,
			})
			if acceptErr == nil {
				testpkg.OwnTestAccount(t, db, account.ID)
			}
			return acceptErr
		})
		// The spent link is refused on replay.
		if _, replayErr := factory.Invitation.AcceptSchoolInvitation(publicCtx, staffToken, identityaccess.InvitationRegistration{
			Password: invitationPassword, ConfirmPassword: invitationPassword,
		}); errors.Is(replayErr, identityaccess.ErrInvitationUsed) {
			replays["school_invitation_replay_refused"]++
		}

		// --- password reset: issue, redeem once.
		resetAddress := inviteeAddress(fmt.Sprintf("link-evidence-reset-%d", iteration))
		testpkg.CreateTestAccountWithPassword(t, db, resetAddress, invitationPassword)
		var resetToken string
		measure("password_reset_initiate", iteration, func() error {
			link, resetErr := factory.AccountAuthentication().InitiatePasswordReset(ctx, resetAddress, identityaccess.PasswordResetScopeStaff)
			if resetErr != nil {
				return resetErr
			}
			if link == nil {
				return fmt.Errorf("no reset link issued for %s", resetAddress)
			}
			resetToken = link.Token
			return nil
		})
		measure("password_reset_complete", iteration, func() error {
			return factory.AccountAuthentication().ResetPassword(publicCtx, resetToken, "R3set!Evidence")
		})
		if replayErr := factory.AccountAuthentication().ResetPassword(publicCtx, resetToken, "R3set!Again"); replayErr != nil {
			replays["password_reset_replay_refused"]++
		}
		// --- guardian invitation: issue, preview, redeem once, resend.
		profile := testpkg.CreateTestGuardianProfile(t, db, fmt.Sprintf("link-evidence-guardian-%d", iteration))
		var guardianToken string
		var guardianInvitationID int64
		measure("guardian_invitation_create", iteration, func() error {
			created, createErr := factory.GuardianInvitation.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
				GuardianProfileID: profile.ID, CreatedBy: inviter.ID,
			})
			if createErr != nil {
				return createErr
			}
			guardianToken, guardianInvitationID = created.Token, created.ID
			return nil
		})
		measure("guardian_invitation_validate", iteration, func() error {
			_, validateErr := factory.GuardianInvitation.ValidateGuardianInvitation(publicCtx, guardianToken)
			return validateErr
		})
		measure("guardian_invitation_resend", iteration, func() error {
			return factory.GuardianInvitation.ResendGuardianInvitation(ctx, guardianInvitationID, inviter.ID)
		})
		measure("guardian_invitation_accept", iteration, func() error {
			account, acceptErr := factory.GuardianInvitation.AcceptGuardianInvitation(publicCtx, guardianToken, identityaccess.GuardianRegistration{
				Password: invitationPassword, ConfirmPassword: invitationPassword,
			})
			if acceptErr == nil {
				testpkg.OwnTestAccount(t, db, account.ID)
			}
			return acceptErr
		})
		if _, replayErr := factory.GuardianInvitation.AcceptGuardianInvitation(publicCtx, guardianToken, identityaccess.GuardianRegistration{
			Password: invitationPassword, ConfirmPassword: invitationPassword,
		}); errors.Is(replayErr, identityaccess.ErrInvitationUsed) {
			replays["guardian_invitation_replay_refused"]++
		}

		measure("invitation_cleanup", iteration, func() error {
			_, cleanupErr := factory.Invitation.DeleteExpiredSchoolInvitations(ctx)
			return cleanupErr
		})
	}

	raw, err := json.Marshal(map[string]any{
		"postgres": version, "warmup": warmup, "samples_per_operation": iterations - warmup, "concurrency": 1,
		"samples": samples, "unit_of_work_events_including_warmup": events(),
		"deadlocks": deadlocks() - beforeDeadlocks, "replay_refusals_including_warmup": replays,
	})
	require.NoError(t, err)
	t.Logf("link-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
	require.Equal(t, iterations, replays["school_invitation_replay_refused"], "every replayed school acceptance must be refused")
	require.Equal(t, iterations, replays["password_reset_replay_refused"], "every replayed reset must be refused")
	require.Equal(t, iterations, replays["guardian_invitation_replay_refused"], "every replayed guardian acceptance must be refused")
}
