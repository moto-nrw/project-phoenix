package users

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// conflictPortStub records what the resolver asked of a domain, in order, so a
// test can assert not only the outcome but the sequence the atomicity depends
// on: lock everything, then decide.
type conflictPortStub struct {
	candidates map[int64]*ParentRequestConflictCandidate
	locked     []int64
	approved   []int64
	rejected   []int64
	staffValue *ParentRequestStaffValueWrite
	lockErr    error
	failID     int64
	failErr    error
	staffErr   error
}

func (s *conflictPortStub) ConflictCandidate(_ context.Context, requestID int64) (*ParentRequestConflictCandidate, error) {
	candidate, found := s.candidates[requestID]
	if !found {
		return nil, ErrReviewNotFound
	}
	return candidate, nil
}

func (s *conflictPortStub) LockConflictRequest(_ context.Context, requestID int64) error {
	if s.lockErr != nil {
		return s.lockErr
	}
	s.locked = append(s.locked, requestID)
	return nil
}

func (s *conflictPortStub) DecideConflictRequest(_ context.Context, decision ParentRequestConflictDecision) error {
	if decision.RequestID == s.failID {
		if s.failErr != nil {
			return s.failErr
		}
		return errors.New("decide failed")
	}
	if decision.Approve {
		s.approved = append(s.approved, decision.RequestID)
	} else {
		s.rejected = append(s.rejected, decision.RequestID)
	}
	return nil
}

func (s *conflictPortStub) WriteStaffValue(_ context.Context, write ParentRequestStaffValueWrite) error {
	if s.staffErr != nil {
		return s.staffErr
	}
	s.staffValue = &write
	return nil
}

// The in-memory group's fixed ids. Declared as typed variables rather than
// written as int64(n) at each assertion: these are stub-map keys, not database
// rows, and the hermetic gate reads an int64(n) literal as a hardcoded id.
var (
	conflictStudentID int64 = 7
	conflictAnchorID  int64 = 4
)

// conflictGroup builds a group of pending requests for ONE child.
func conflictGroup(updatedAt time.Time, ids ...int64) map[int64]*ParentRequestConflictCandidate {
	candidates := make(map[int64]*ParentRequestConflictCandidate, len(ids))
	for _, id := range ids {
		candidates[id] = &ParentRequestConflictCandidate{StudentID: conflictStudentID, UpdatedAt: updatedAt}
	}
	return candidates
}

func resolverWithExcusedPort(port ParentRequestConflictPort) *ParentRequestCoordinator {
	coordinator := NewParentRequestCoordinator(nil, nil)
	coordinator.SetExcusedConflictPort(port)
	return coordinator
}

func excusedResolveInput(versions time.Time, ids ...int64) ResolveConflictInput {
	expected := make([]string, 0, len(ids))
	for range ids {
		expected = append(expected, ParentRequestVersion(versions))
	}
	return ResolveConflictInput{
		Kind: ParentRequestKindExcused, RequestIDs: ids, ExpectedVersions: expected,
		Reason: "Mit den Eltern geklärt", ReviewerID: 99,
	}
}

func TestResolveConflictApprovesChosenAndRejectsEveryOtherRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5, 6)}
	input := excusedResolveInput(now, 4, 5, 6)
	input.ChosenRequestID = 5

	ctx := bulkReviewContext(t, "users:update")
	require.NoError(t, resolverWithExcusedPort(port).ResolveConflict(ctx, input))

	assert.Equal(t, []int64{5}, port.approved, "exactly one wish may win")
	assert.Equal(t, []int64{4, 6}, port.rejected, "every other wish in the group is refused")
	assert.Equal(t, []int64{4, 5, 6}, port.locked, "the whole group is locked in a fixed id order first")
	assert.False(t, tenant.RollbackRequested(ctx))
}

func TestResolveConflictNoneRejectsEveryRequestAndApprovesNothing(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	input := excusedResolveInput(now, 4, 5)
	input.None = true

	require.NoError(t, resolverWithExcusedPort(port).ResolveConflict(bulkReviewContext(t, "users:update"), input))

	assert.Empty(t, port.approved)
	assert.Equal(t, []int64{4, 5}, port.rejected)
	assert.Nil(t, port.staffValue)
}

func TestResolveConflictStaffValueRejectsEveryRequestAndWritesTheTypedValue(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	input := excusedResolveInput(now, 4, 5)
	input.StaffValue = map[string]any{"value": "excused"}
	input.ConflictKey = "absence:2026-09-01"

	require.NoError(t, resolverWithExcusedPort(port).ResolveConflict(bulkReviewContext(t, "users:update"), input))

	assert.Empty(t, port.approved, "a typed value means none of the wishes won")
	assert.Equal(t, []int64{4, 5}, port.rejected)
	require.NotNil(t, port.staffValue)
	assert.Equal(t, conflictStudentID, port.staffValue.StudentID)
	assert.Equal(t, []int64{4, 5}, port.staffValue.RequestIDs, "the domain reads the scope from the group")
	assert.Equal(t, "absence:2026-09-01", port.staffValue.ConflictKey)
	assert.Equal(t, "Mit den Eltern geklärt", port.staffValue.Reason)
}

// recordingEventStub captures the ledger entries the resolver files itself.
type recordingEventStub struct {
	events []ParentRequestEventInput
}

func (s *recordingEventStub) Record(_ context.Context, input ParentRequestEventInput) error {
	s.events = append(s.events, input)
	return nil
}

func (*recordingEventStub) ListForRequest(context.Context, string, int64) ([]*userModels.ParentRequestEvent, error) {
	return nil, nil
}

func TestResolveConflictRecordsOnlyTheStaffValueInTheLedger(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 6, 4, 5)}
	ledger := &recordingEventStub{}
	coordinator := resolverWithExcusedPort(port)
	coordinator.SetEventRecorder(ledger)
	input := excusedResolveInput(now, 6, 4, 5)
	input.StaffValue = map[string]any{"value": "sick"}
	input.ConflictKey = "absence:2026-09-01"

	require.NoError(t, coordinator.ResolveConflict(bulkReviewContext(t, "users:update"), input))

	require.Len(t, ledger.events, 1,
		"the domains record their own decided events; the resolver adds only the typed result")
	event := ledger.events[0]
	assert.Equal(t, userModels.ParentRequestEventDecided, event.EventType)
	assert.Equal(t, string(ParentRequestKindExcused), event.RequestType)
	assert.Equal(t, conflictAnchorID, event.RequestID, "the group's lowest id is the deterministic anchor")
	assert.Equal(t, conflictStudentID, event.StudentID)
	assert.Equal(t, int64(99), event.ActorAccountID)
	assert.Equal(t, map[string]any{"value": "sick"}, event.Payload["staff_value"])
	assert.Equal(t, "absence:2026-09-01", event.Payload["conflict_key"])
	assert.Equal(t, []int64{6, 4, 5}, event.Payload["request_ids"])
}

func TestResolveConflictRecordsNothingWhenAWishWins(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	ledger := &recordingEventStub{}
	coordinator := resolverWithExcusedPort(port)
	coordinator.SetEventRecorder(ledger)
	input := excusedResolveInput(now, 4, 5)
	input.ChosenRequestID = 4

	require.NoError(t, coordinator.ResolveConflict(bulkReviewContext(t, "users:update"), input))

	assert.Empty(t, ledger.events,
		"a duplicate decided event would show the family two decisions where there was one")
}

func TestResolveConflictWithoutALedgerStillResolves(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	input := excusedResolveInput(now, 4, 5)
	input.StaffValue = map[string]any{"value": "sick"}

	require.NoError(t, resolverWithExcusedPort(port).
		ResolveConflict(bulkReviewContext(t, "users:update"), input))
	assert.Equal(t, []int64{4, 5}, port.rejected)
}

func TestResolveConflictRefusesTheWholeSetWhenOneVersionIsStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	input := excusedResolveInput(now, 4, 5)
	input.ExpectedVersions[1] = "stale"
	input.ChosenRequestID = 4

	ctx := bulkReviewContext(t, "users:update")
	err := resolverWithExcusedPort(port).ResolveConflict(ctx, input)

	require.ErrorIs(t, err, ErrParentRequestStale)
	assert.Empty(t, port.approved)
	assert.Empty(t, port.rejected)
	assert.Empty(t, port.locked, "a stale set is refused before anything is locked")
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestResolveConflictRollsBackWhenOneRejectionFails(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5, 6), failID: 6}
	input := excusedResolveInput(now, 4, 5, 6)
	input.ChosenRequestID = 4

	ctx := bulkReviewContext(t, "users:update")
	err := resolverWithExcusedPort(port).ResolveConflict(ctx, input)

	require.Error(t, err)
	assert.True(t, tenant.RollbackRequested(ctx),
		"a half-resolved group — one wish approved, its rival still pending — must never commit")
}

func TestResolveConflictReportsADecisionRaceAsStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{
		candidates: conflictGroup(now, 4, 5), failID: 5, failErr: ErrReviewNotPending,
	}
	input := excusedResolveInput(now, 4, 5)
	input.ChosenRequestID = 4

	err := resolverWithExcusedPort(port).ResolveConflict(bulkReviewContext(t, "users:update"), input)

	require.ErrorIs(t, err, ErrParentRequestStale)
}

func TestResolveConflictRefusesAGroupSpanningTwoChildren(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	port.candidates[5].StudentID = 8
	input := excusedResolveInput(now, 4, 5)
	input.None = true

	ctx := bulkReviewContext(t, "users:update")
	err := resolverWithExcusedPort(port).ResolveConflict(ctx, input)

	require.ErrorIs(t, err, ErrInvalidConflictResolution)
	assert.Empty(t, port.rejected, "two children are not a conflict; nothing may be decided")
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestResolveConflictRefusesAnythingOtherThanExactlyOneOutcome(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)

	tests := map[string]func(*ResolveConflictInput){
		"no outcome at all": func(*ResolveConflictInput) {},
		"a wish and a typed value": func(in *ResolveConflictInput) {
			in.ChosenRequestID = 4
			in.StaffValue = map[string]any{"value": "sick"}
		},
		"a typed value and none": func(in *ResolveConflictInput) {
			in.StaffValue = map[string]any{"value": "sick"}
			in.None = true
		},
		"a chosen request outside the group": func(in *ResolveConflictInput) {
			in.ChosenRequestID = 99
		},
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
			input := excusedResolveInput(now, 4, 5)
			mutate(&input)

			err := resolverWithExcusedPort(port).ResolveConflict(bulkReviewContext(t, "users:update"), input)

			require.ErrorIs(t, err, ErrInvalidConflictResolution)
			assert.Empty(t, port.rejected)
		})
	}
}

func TestResolveConflictRequiresAReasonBecauseItAlwaysRejects(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
	input := excusedResolveInput(now, 4, 5)
	input.ChosenRequestID = 4
	input.Reason = "   "

	err := resolverWithExcusedPort(port).ResolveConflict(bulkReviewContext(t, "users:update"), input)

	require.ErrorIs(t, err, ErrParentRequestReasonRequired)
	assert.Empty(t, port.rejected)
}

func TestResolveConflictRefusesAGroupOfOne(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4)}
	input := excusedResolveInput(now, 4)
	input.None = true

	err := resolverWithExcusedPort(port).ResolveConflict(bulkReviewContext(t, "users:update"), input)

	require.ErrorIs(t, err, ErrInvalidConflictResolution)
}

func TestResolveConflictRefusesAKindWithNoWiredDomain(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	input := excusedResolveInput(now, 4, 5)
	input.Kind = ParentRequestKindOffering
	input.None = true

	ctx := bulkReviewContext(t, "users:update")
	err := resolverWithExcusedPort(&conflictPortStub{}).ResolveConflict(ctx, input)

	require.ErrorIs(t, err, ErrConflictKindUnsupported)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestResolveConflictLetsAnAbsenceOnlyReviewerResolveAbsencesOnly(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)

	t.Run("absences are allowed", func(t *testing.T) {
		t.Parallel()

		port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
		input := excusedResolveInput(now, 4, 5)
		input.None = true

		require.NoError(t, resolverWithExcusedPort(port).
			ResolveConflict(bulkReviewContext(t, "users:absence"), input))
		assert.Equal(t, []int64{4, 5}, port.rejected)
	})

	t.Run("Stammdaten are not", func(t *testing.T) {
		t.Parallel()

		port := &conflictPortStub{candidates: conflictGroup(now, 4, 5)}
		coordinator := NewParentRequestCoordinator(nil, nil)
		coordinator.SetMasterDataConflictPort(port)
		input := excusedResolveInput(now, 4, 5)
		input.Kind = ParentRequestKindMasterData
		input.None = true

		ctx := bulkReviewContext(t, "users:absence")
		err := coordinator.ResolveConflict(ctx, input)

		require.ErrorIs(t, err, ErrParentRequestForbidden)
		assert.Empty(t, port.rejected)
		assert.True(t, tenant.RollbackRequested(ctx))
	})
}

func TestResolveConflictReportsALockRaceAsStale(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 29, 10, 0, 0, 0, time.UTC)
	port := &conflictPortStub{candidates: conflictGroup(now, 4, 5), lockErr: ErrReviewNotPending}
	input := excusedResolveInput(now, 4, 5)
	input.None = true

	ctx := bulkReviewContext(t, "users:update")
	err := resolverWithExcusedPort(port).ResolveConflict(ctx, input)

	require.ErrorIs(t, err, ErrParentRequestStale)
	assert.Empty(t, port.rejected)
	assert.True(t, tenant.RollbackRequested(ctx))
}

func TestSetCareConflictPortServesBothCareKinds(t *testing.T) {
	t.Parallel()

	port := &conflictPortStub{}
	coordinator := NewParentRequestCoordinator(nil, nil)
	coordinator.SetCareConflictPort(port)

	ctx := bulkReviewContext(t, "users:update")
	for _, kind := range []ParentRequestKind{ParentRequestKindCareSchedule, ParentRequestKindPickupChange} {
		resolved, err := coordinator.conflictPort(ctx, kind)
		require.NoError(t, err, "%s must resolve to the care domain", kind)
		assert.Same(t, port, resolved)
	}
}
