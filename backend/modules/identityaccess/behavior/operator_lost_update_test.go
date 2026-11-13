package behavior_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// An operator self-service write that read the row without a lock must not
// write a stale copy back over a deactivation that committed in between: the
// deactivated operator would be active again (and the retention trigger would
// clear deactivated_at). Each test holds an uncommitted deactivation, starts
// the self-service write, waits until that write blocks on the held row lock,
// and only then commits the deactivation.

func TestOperatorChangePasswordKeepsConcurrentDeactivation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	operatorID, oldHash := createEmailChangeTestOperator(t, db, fmt.Sprintf("pw-race-%d@test.local", time.Now().UnixNano()))
	newPassword := testpkg.NewTestPassword()
	passwords := buildOperatorPasswordChange(t, db)

	err := raceOperatorDeactivation(t, db, operatorID, func(ctx context.Context) error {
		return passwords.ChangeOperatorPassword(ctx, operatorID, operatorTestPassword, newPassword)
	})

	require.NoError(t, err)
	assertOperatorDeactivated(t, db, operatorID)
	var hash string
	require.NoError(t, db.NewRaw(`SELECT password_hash FROM platform.operators WHERE id = ?`, operatorID).Scan(context.Background(), &hash))
	assert.NotEqual(t, oldHash, hash, "the password change itself still lands")
}

func TestOperatorUpdateProfileKeepsConcurrentDeactivation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	operatorID, _ := createEmailChangeTestOperator(t, db, fmt.Sprintf("profile-race-%d@test.local", time.Now().UnixNano()))
	module := buildServiceFactory(t, db)

	err := raceOperatorDeactivation(t, db, operatorID, func(ctx context.Context) error {
		_, updateErr := module.UpdateOperatorProfile(ctx, operatorID, "Renamed Operator")
		return updateErr
	})

	require.NoError(t, err)
	assertOperatorDeactivated(t, db, operatorID)
	var name string
	require.NoError(t, db.NewRaw(`SELECT display_name FROM platform.operators WHERE id = ?`, operatorID).Scan(context.Background(), &name))
	assert.Equal(t, "Renamed Operator", name)
}

func TestOperatorEmailChangeConfirmKeepsConcurrentDeactivation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	email := fmt.Sprintf("mail-race-%d@test.local", time.Now().UnixNano())
	operatorID, _ := createEmailChangeTestOperator(t, db, email)
	service := buildAuthService(t, db)
	newEmail := fmt.Sprintf("mail-race-new-%d@test.local", time.Now().UnixNano())
	clientIP := net.IPv4(127, 0, 0, 1)
	require.NoError(t, service.InitiateEmailChange(context.Background(), operatorID, newEmail, operatorTestPassword, clientIP))
	token := mailedLinkToken(t, db, newEmail)

	err := raceOperatorDeactivation(t, db, operatorID, func(ctx context.Context) error {
		_, confirmErr := service.ConfirmEmailChange(ctx, token, clientIP)
		return confirmErr
	})

	require.ErrorIs(t, err, identityaccess.ErrOperatorInactive, "the confirmation sees the committed deactivation")
	assertOperatorDeactivated(t, db, operatorID)
	var stored string
	require.NoError(t, db.NewRaw(`SELECT email FROM platform.operators WHERE id = ?`, operatorID).Scan(context.Background(), &stored))
	assert.Equal(t, email, stored, "a refused confirmation keeps the old address")
}

// raceOperatorDeactivation deactivates the operator in a transaction that
// commits only after write has blocked on that row, and returns write's error.
func raceOperatorDeactivation(t *testing.T, db *bun.DB, operatorID int64, write func(context.Context) error) error {
	t.Helper()
	return raceHeldTransition(t, db, write, `UPDATE platform.operators SET active = false WHERE id = ?`, operatorID)
}

// raceHeldTransition runs the transition statement in a transaction that
// commits only after write has blocked on the row it changed, then returns
// write's error. The write's unlocked reads therefore see the state before the
// transition, and its row write lands after the transition committed.
func raceHeldTransition(t *testing.T, db *bun.DB, write func(context.Context) error, transition string, args ...any) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	committed := false
	defer func() {
		if !committed {
			assert.NoError(t, holder.Rollback())
		}
	}()
	var holderPID int
	require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &holderPID))
	result, err := holder.NewRaw(transition, args...).Exec(ctx)
	require.NoError(t, err)
	changed, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 1, changed, "the held transition changes exactly the raced row")

	finished := make(chan error, 1)
	go func() { finished <- write(ctx) }()
	awaitBlockedBy(t, db, holderPID, finished)
	require.NoError(t, holder.Commit())
	committed = true
	select {
	case err := <-finished:
		return err
	case <-ctx.Done():
		t.Fatal("the write did not finish after the transition committed")
		return nil
	}
}

// awaitBlockedBy waits until another backend waits on a lock holderPID holds.
// A write that finishes first never touched the held row; that is a broken
// arrangement, not a passing test.
func awaitBlockedBy(t *testing.T, db *bun.DB, holderPID int, finished <-chan error) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-finished:
			t.Fatalf("the write finished without waiting on the held row lock: %v", err)
		default:
		}
		var waiting bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ?::int = ANY (pg_blocking_pids(pid)))`, holderPID).
			Scan(context.Background(), &waiting))
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no backend waited on the held row lock")
}

func assertOperatorDeactivated(t *testing.T, db *bun.DB, operatorID int64) {
	t.Helper()
	var state struct {
		Active        bool       `bun:"active"`
		DeactivatedAt *time.Time `bun:"deactivated_at"`
	}
	require.NoError(t, db.NewRaw(`SELECT active, deactivated_at FROM platform.operators WHERE id = ?`, operatorID).
		Scan(context.Background(), &state))
	assert.False(t, state.Active, "the committed deactivation stays in force")
	assert.NotNil(t, state.DeactivatedAt, "the retention clock keeps running")
}
