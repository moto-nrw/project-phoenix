package integration

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestOperatorTokenRuntimeEvidence records the runtime evidence for the
// operator invitation and e-mail change cutover (#2722): statement counts,
// latency, affected rows, pool waits, unit-of-work outcomes (including the
// rollback of an injected failure after the last write), deadlocks and the
// duplicate-prevention conflicts per operation on an isolated clone. It
// mirrors the sequences the retained operator service runs over the public
// capability; the raw JSON is logged for the migration ticket.
func TestOperatorTokenRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tokens, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: func(identityCompose.Observation) {}})
	require.NoError(t, err)
	inviter := testpkg.CreateTestOperator(t, db)
	operator := testpkg.CreateTestOperator(t, db)
	counter := testpkg.CaptureQueriesForContext(t, db)
	testpkg.AttachLockWaitEvidence(db)
	ctx, events := testpkg.CaptureUnitOfWorkEvidence(counter.Context(testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)))
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
	conflicts := map[string]int{}
	measure := func(operation string, iteration int, fn func() error) {
		counter.Reset()
		before := db.Stats()
		started := time.Now()
		err := fn()
		elapsed := time.Since(started)
		after := db.Stats()
		if iteration < 5 {
			require.NoError(t, err, "warmup %s", operation)
			return
		}
		sample := testpkg.RuntimeCheckpointSample{
			DurationMS: float64(elapsed) / float64(time.Millisecond), Queries: counter.Total(),
			PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
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
	withinAdmin := func(fn func(context.Context) error) error {
		return testpkg.WithAdminTx(t, ctx, db, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
	}
	invite := func(txCtx context.Context, invitation identityaccess.OperatorInvitation) error {
		if _, err := tokens.FindOperatorForUpdate(txCtx, inviter.ID); err != nil {
			return err
		}
		if _, err := tokens.CountOperatorInvitationsCreatedAfter(txCtx, inviter.ID, time.Now().Add(-time.Hour)); err != nil {
			return err
		}
		if _, err := tokens.RevokeOperatorInvitationsForEmail(txCtx, invitation.Email); err != nil {
			return err
		}
		_, err := tokens.CreateOperatorInvitation(txCtx, invitation)
		return err
	}
	initiate := func(txCtx context.Context, change identityaccess.OperatorEmailChange) error {
		if _, err := tokens.FindOperatorForUpdate(txCtx, operator.ID); err != nil {
			return err
		}
		if _, err := tokens.CountOperatorEmailChangesCreatedAfter(txCtx, operator.ID, time.Now().Add(-time.Hour)); err != nil {
			return err
		}
		if err := tokens.RevokeOperatorEmailChanges(txCtx, operator.ID); err != nil {
			return err
		}
		_, err := tokens.CreateOperatorEmailChange(txCtx, change)
		return err
	}

	for iteration := range 35 {
		invitation := newInvitation(inviter.ID, inviteeEmail())
		// Invite: inviter lock, rate-limit count, previous-link revocation and
		// insert in one administrative transaction; a failure after the last
		// write rolls everything back first.
		measure("invite_rolled_back", iteration, func() error {
			err := withinAdmin(func(txCtx context.Context) error {
				if err := invite(txCtx, invitation); err != nil {
					return err
				}
				return errInjected
			})
			if errors.Is(err, errInjected) {
				return nil
			}
			return err
		})
		measure("invite", iteration, func() error {
			return withinAdmin(func(txCtx context.Context) error { return invite(txCtx, invitation) })
		})
		var stored identityaccess.OperatorInvitation
		measure("record_invitation_delivery", iteration, func() error {
			var err error
			stored, err = tokens.FindRedeemableOperatorInvitation(ctx, invitation.Token)
			if err != nil {
				return err
			}
			sentAt := time.Now()
			return tokens.RecordOperatorInvitationDelivery(ctx, stored.ID, identityaccess.TokenDelivery{SentAt: &sentAt, RetryCount: 1})
		})
		measure("list_pending_invitations", iteration, func() error {
			_, err := tokens.ListRedeemableOperatorInvitations(ctx)
			return err
		})
		measure("resend_invitation", iteration, func() error {
			if _, err := tokens.FindOperatorInvitation(ctx, stored.ID); err != nil {
				return err
			}
			if err := tokens.ExtendOperatorInvitation(ctx, stored.ID, time.Now().Add(48*time.Hour)); err != nil {
				return err
			}
			return tokens.RecordOperatorInvitationDelivery(ctx, stored.ID, identityaccess.TokenDelivery{})
		})
		// Accept: redemption and operator creation commit together.
		measure("accept_invitation", iteration, func() error {
			return withinAdmin(func(txCtx context.Context) error {
				redeemed, err := tokens.RedeemOperatorInvitation(txCtx, invitation.Token)
				if err != nil {
					return err
				}
				created, err := tokens.CreateOperator(txCtx, identityaccess.Operator{
					Email: redeemed.Email, DisplayName: "Evidence", PasswordHash: "hash", Active: true,
				})
				if err == nil {
					testpkg.OwnTestOperator(t, db, created.ID)
				}
				return err
			})
		})
		// A replayed acceptance of the same link is refused.
		if _, err := tokens.RedeemOperatorInvitation(ctx, invitation.Token); errors.Is(err, identityaccess.ErrOperatorInvitationNotFound) {
			conflicts["replayed_invitation_redemption"]++
		}

		change := newEmailChange(operator.ID)
		measure("initiate_email_change", iteration, func() error {
			return withinAdmin(func(txCtx context.Context) error { return initiate(txCtx, change) })
		})
		// A second active link without the revocation hits the unique index.
		if _, err := tokens.CreateOperatorEmailChange(ctx, newEmailChange(operator.ID)); err != nil {
			conflicts["duplicate_active_email_change"]++
		}
		measure("confirm_email_change_rolled_back", iteration, func() error {
			err := withinAdmin(func(txCtx context.Context) error {
				if _, err := tokens.RedeemOperatorEmailChange(txCtx, change.Token); err != nil {
					return err
				}
				return errInjected
			})
			if errors.Is(err, errInjected) {
				return nil
			}
			return err
		})
		measure("confirm_email_change", iteration, func() error {
			return withinAdmin(func(txCtx context.Context) error {
				redeemed, err := tokens.RedeemOperatorEmailChange(txCtx, change.Token)
				if err != nil {
					return err
				}
				current, err := tokens.FindOperator(txCtx, redeemed.OperatorID)
				if err != nil {
					return err
				}
				current.Email = redeemed.NewEmail
				_, err = tokens.UpdateOperator(txCtx, current)
				return err
			})
		})
		if _, err := tokens.RedeemOperatorEmailChange(ctx, change.Token); errors.Is(err, identityaccess.ErrOperatorEmailChangeNotFound) {
			conflicts["replayed_email_change_redemption"]++
		}
		measure("cleanup", iteration, func() error {
			if _, err := tokens.DeleteExpiredOperatorInvitations(ctx); err != nil {
				return err
			}
			if _, err := tokens.RevokeExpiredOperatorEmailChanges(ctx); err != nil {
				return err
			}
			_, err := tokens.DeleteStaleOperatorEmailChanges(ctx)
			return err
		})
	}
	raw, err := json.Marshal(map[string]any{
		"postgres": version, "warmup": 5, "samples_per_operation": 30, "concurrency": 1, "samples": samples,
		"unit_of_work_events_including_warmup": events(), "deadlocks": deadlocks() - beforeDeadlocks,
		"duplicate_prevention_conflicts_including_warmup": conflicts,
	})
	require.NoError(t, err)
	t.Logf("operator-token-runtime %s", raw)
	require.Empty(t, failures, "every measured sample must succeed")
	require.Equal(t, 35, conflicts["replayed_invitation_redemption"], "every replayed acceptance must be refused")
	require.Equal(t, 35, conflicts["replayed_email_change_redemption"], "every replayed confirmation must be refused")
	require.Equal(t, 35, conflicts["duplicate_active_email_change"], "every second active link must be refused")
}
