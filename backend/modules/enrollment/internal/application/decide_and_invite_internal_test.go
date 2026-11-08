package application

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The decision command of the admin routes and the manual approval (#6093):
// one unit holds the decision and the guardian invitation, and a standalone
// call retries once on a transient failure before the decision body ran.

type unitKey struct{}

// fakeUnits runs fn in a marked context, as a transaction runner would, and
// fails the commit when commitErr is set.
type fakeUnits struct {
	opened    int
	commitErr error
}

func (u *fakeUnits) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	if ctx.Value(unitKey{}) != nil {
		return fn(ctx)
	}
	u.opened++
	if err := fn(context.WithValue(ctx, unitKey{}, u.opened)); err != nil {
		return err
	}
	return u.commitErr
}

type recordedInvitations struct {
	requests []identityaccess.GuardianInvitationRequest
	units    []any
	err      error
}

func (r *recordedInvitations) CreateGuardianInvitation(ctx context.Context, request identityaccess.GuardianInvitationRequest) (identityaccess.GuardianInvitation, error) {
	r.requests = append(r.requests, request)
	r.units = append(r.units, ctx.Value(unitKey{}))
	return identityaccess.GuardianInvitation{}, r.err
}

type scriptedDecide struct {
	errs    []error
	outcome *enrollmentNative.DecideOutcome
	units   []any
}

func (s *scriptedDecide) decide(ctx context.Context, _ enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error) {
	s.units = append(s.units, ctx.Value(unitKey{}))
	call := len(s.units) - 1
	if call < len(s.errs) && s.errs[call] != nil {
		return nil, s.errs[call]
	}
	out := *s.outcome
	return &out, nil
}

func transientForTest(err error) bool { return errors.Is(err, driver.ErrBadConn) }

func decisionCommand(units *fakeUnits, invitations DecisionGuardianInvitations) *Decisions {
	return &Decisions{deps: DecisionDependencies{
		GuardianInvitations: invitations,
		IsTransient:         transientForTest,
		Runtime: Runtime{
			Transactions:  units,
			InTransaction: func(ctx context.Context) bool { return ctx.Value(unitKey{}) != nil },
		},
		Logger: slog.Default(),
	}}
}

func approvedOutcome(invite *enrollmentNative.PendingGuardianInvite) *enrollmentNative.DecideOutcome {
	return &enrollmentNative.DecideOutcome{
		Child:         &enrollmentNative.RequestChild{ID: 99, Status: enrollmentNative.ChildStatusApproved},
		PendingInvite: invite,
	}
}

var decideApproved = enrollmentNative.DecideInput{RequestID: 1234, ChildID: 99, Status: enrollmentNative.DecisionApproved, ReviewedBy: 7}

func TestDecideAndInvite_CreatesTheOwedInvitationInTheDecisionUnit(t *testing.T) {
	t.Parallel()
	units, invitations := &fakeUnits{}, &recordedInvitations{}
	decide := &scriptedDecide{outcome: approvedOutcome(&enrollmentNative.PendingGuardianInvite{GuardianProfileID: 55, CreatedBy: 7})}

	outcome, err := decisionCommand(units, invitations).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.NoError(t, err)
	assert.Equal(t, int64(99), outcome.Child.ID)
	assert.Nil(t, outcome.PendingInvite, "the command leaves no invitation pending for its caller")
	require.Equal(t, []identityaccess.GuardianInvitationRequest{{GuardianProfileID: 55, CreatedBy: 7}}, invitations.requests)
	assert.Equal(t, decide.units, invitations.units, "decision and invitation share one unit")
	assert.Equal(t, 1, units.opened)
}

func TestDecideAndInvite_SkipsTheInvitationWhenNoneIsOwed(t *testing.T) {
	t.Parallel()
	invitations := &recordedInvitations{}
	decide := &scriptedDecide{outcome: approvedOutcome(nil)}

	_, err := decisionCommand(&fakeUnits{}, invitations).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.NoError(t, err)
	assert.Empty(t, invitations.requests)
}

func TestDecideAndInvite_InvitationFailureFailsTheDecisionWithoutReplay(t *testing.T) {
	t.Parallel()
	invitations := &recordedInvitations{err: fmt.Errorf("enqueue invitation mail: %w", driver.ErrBadConn)}
	decide := &scriptedDecide{outcome: approvedOutcome(&enrollmentNative.PendingGuardianInvite{GuardianProfileID: 55, CreatedBy: 7})}

	_, err := decisionCommand(&fakeUnits{}, invitations).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.ErrorIs(t, err, driver.ErrBadConn)
	assert.Len(t, decide.units, 1, "a failure after the decision body ran must not replay the approval")
}

func TestDecideAndInvite_RetriesOnceAfterTransientFailureBeforeTheDecisionRan(t *testing.T) {
	t.Parallel()
	units := &fakeUnits{}
	decide := &scriptedDecide{
		errs:    []error{fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn)},
		outcome: approvedOutcome(nil),
	}

	outcome, err := decisionCommand(units, &recordedInvitations{}).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.NoError(t, err)
	assert.Equal(t, enrollmentNative.ChildStatusApproved, outcome.Child.Status)
	assert.Len(t, decide.units, 2)
	assert.Equal(t, 2, units.opened, "the retry runs in a fresh unit")
}

func TestDecideAndInvite_GivesUpAfterTheSecondTransientFailure(t *testing.T) {
	t.Parallel()
	transient := fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn)
	decide := &scriptedDecide{errs: []error{transient, transient, nil}, outcome: approvedOutcome(nil)}

	_, err := decisionCommand(&fakeUnits{}, &recordedInvitations{}).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.ErrorIs(t, err, driver.ErrBadConn)
	assert.Len(t, decide.units, 2)
}

func TestDecideAndInvite_DoesNotRetryAfterTheDecisionBodySucceeded(t *testing.T) {
	t.Parallel()
	units := &fakeUnits{commitErr: fmt.Errorf("commit failed: %w", driver.ErrBadConn)}
	decide := &scriptedDecide{outcome: approvedOutcome(nil)}

	_, err := decisionCommand(units, &recordedInvitations{}).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.ErrorIs(t, err, driver.ErrBadConn)
	assert.Len(t, decide.units, 1, "commit-boundary errors after a successful decision body must not replay the approval")
}

func TestDecideAndInvite_DoesNotRetryNonTransientFailures(t *testing.T) {
	t.Parallel()
	decide := &scriptedDecide{errs: []error{enrollmentNative.ErrDecisionAlreadyTerminal}, outcome: approvedOutcome(nil)}

	_, err := decisionCommand(&fakeUnits{}, &recordedInvitations{}).decideAndInvite(context.Background(), decideApproved, decide.decide)

	require.ErrorIs(t, err, enrollmentNative.ErrDecisionAlreadyTerminal)
	assert.Len(t, decide.units, 1)
}

func TestDecideAndInvite_DoesNotRetryAnEndedCallerContext(t *testing.T) {
	t.Parallel()
	for name, end := range map[string]func(context.Context) (context.Context, context.CancelFunc){
		"canceled": func(parent context.Context) (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(parent)
			cancel()
			return ctx, func() {}
		},
		"deadline exceeded": func(parent context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(parent, -1)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := end(context.Background())
			defer cancel()
			decide := &scriptedDecide{errs: []error{fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn)}, outcome: approvedOutcome(nil)}

			_, err := decisionCommand(&fakeUnits{}, &recordedInvitations{}).decideAndInvite(ctx, decideApproved, decide.decide)

			require.ErrorIs(t, err, ctx.Err())
			assert.Len(t, decide.units, 1)
		})
	}
}

func TestDecideAndInvite_JoinsTheCallerUnitWithoutOwningARetry(t *testing.T) {
	t.Parallel()
	units, invitations := &fakeUnits{}, &recordedInvitations{}
	decide := &scriptedDecide{
		errs:    []error{fmt.Errorf("decision: list child offerings: %w", driver.ErrBadConn)},
		outcome: approvedOutcome(&enrollmentNative.PendingGuardianInvite{GuardianProfileID: 55, CreatedBy: 7}),
	}
	callerUnit := context.WithValue(context.Background(), unitKey{}, "caller")

	_, err := decisionCommand(units, invitations).decideAndInvite(callerUnit, decideApproved, decide.decide)

	require.ErrorIs(t, err, driver.ErrBadConn, "only the owner of the unit may replay it")
	assert.Equal(t, []any{"caller"}, decide.units)
	assert.Zero(t, units.opened)
}
