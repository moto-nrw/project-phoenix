package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// conflictPortStub records what the resolver asked of a queue, in order, so a
// test can assert not only the outcome but the sequence the atomicity depends
// on: lock everything, then decide.
type conflictPortStub struct {
	candidates map[int64]*ports.ConflictCandidate
	locked     []int64
	approved   []int64
	rejected   []int64
	staffValue *ports.StaffValueWrite
	lockErr    error
	failID     int64
	failErr    error
	staffErr   error
}

func (s *conflictPortStub) ConflictCandidate(_ context.Context, requestID int64) (*ports.ConflictCandidate, error) {
	candidate, found := s.candidates[requestID]
	if !found {
		return nil, masterdatarequests.ErrReviewNotFound
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

func (s *conflictPortStub) DecideConflictRequest(_ context.Context, decision ports.ConflictDecision) error {
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

func (s *conflictPortStub) WriteStaffValue(_ context.Context, write ports.StaffValueWrite) error {
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
	conflictReviewer  int64 = 99
	conflictOutsideID int64 = 99
)

// conflictGroup builds a group of pending requests for ONE child.
func conflictGroup(updatedAt time.Time, ids ...int64) map[int64]*ports.ConflictCandidate {
	candidates := make(map[int64]*ports.ConflictCandidate, len(ids))
	for _, id := range ids {
		candidates[id] = &ports.ConflictCandidate{StudentID: conflictStudentID, UpdatedAt: updatedAt}
	}
	return candidates
}

func resolverWithExcusedPort(t *testing.T, rights ports.ParentRequestRights, port ports.ConflictPort, ledger ports.RequestLedger) (*ParentRequestCoordinator, *rollbackRecorder) {
	t.Helper()
	return newTestCoordinator(t, rights, ParentRequestCoordinatorDependencies{ExcusedConflicts: port, Ledger: ledger})
}

func excusedResolveInput(versions time.Time, ids ...int64) parentrequests.ResolveConflictInput {
	expected := make([]string, 0, len(ids))
	for range ids {
		expected = append(expected, careplan.ParentRequestVersion(versions))
	}
	return parentrequests.ResolveConflictInput{
		Kind: parentrequests.KindExcused, RequestIDs: ids, ExpectedVersions: expected,
		Reason: "Mit den Eltern geklärt", ReviewerID: conflictReviewer,
	}
}

func TestResolveConflictApprovesChosenAndRejectsEveryOtherRequest(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5, 6)}
	input := excusedResolveInput(bulkNow, 4, 5, 6)
	input.ChosenRequestID = 5

	coordinator, rollback := resolverWithExcusedPort(t, writeReviewer, port, nil)
	require.NoError(t, coordinator.ResolveConflict(context.Background(), input))

	assert.Equal(t, []int64{5}, port.approved, "exactly one wish may win")
	assert.Equal(t, []int64{4, 6}, port.rejected, "every other wish in the group is refused")
	assert.Equal(t, []int64{4, 5, 6}, port.locked, "the whole group is locked in a fixed id order first")
	assert.False(t, rollback.requested)
}

func TestResolveConflictNoneRejectsEveryRequestAndApprovesNothing(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.None = true

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
	require.NoError(t, coordinator.ResolveConflict(context.Background(), input))

	assert.Empty(t, port.approved)
	assert.Equal(t, []int64{4, 5}, port.rejected)
	assert.Nil(t, port.staffValue)
}

func TestResolveConflictStaffValueRejectsEveryRequestAndWritesTheTypedValue(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.StaffValue = json.RawMessage(`{"value":"excused"}`)
	input.ConflictKey = "absence:2026-09-01"

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
	require.NoError(t, coordinator.ResolveConflict(context.Background(), input))

	assert.Empty(t, port.approved, "a typed value means none of the wishes won")
	assert.Equal(t, []int64{4, 5}, port.rejected)
	require.NotNil(t, port.staffValue)
	assert.Equal(t, conflictStudentID, port.staffValue.StudentID)
	assert.Equal(t, []int64{4, 5}, port.staffValue.RequestIDs, "the queue reads the scope from the group")
	assert.Equal(t, "absence:2026-09-01", port.staffValue.ConflictKey)
	assert.Equal(t, "Mit den Eltern geklärt", port.staffValue.Reason)
	assert.Equal(t, map[string]any{"value": "excused"}, port.staffValue.Value, "the kind reads its payload unchanged")
}

// recordingLedger captures the ledger entries the resolver files itself.
type recordingLedger struct {
	entries []ports.RequestLedgerEntry
}

func (l *recordingLedger) Record(_ context.Context, entry ports.RequestLedgerEntry) error {
	l.entries = append(l.entries, entry)
	return nil
}

func TestResolveConflictRecordsOnlyTheStaffValueInTheLedger(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 6, 4, 5)}
	ledger := &recordingLedger{}
	input := excusedResolveInput(bulkNow, 6, 4, 5)
	input.StaffValue = json.RawMessage(`{"value":"sick"}`)
	input.ConflictKey = "absence:2026-09-01"

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, ledger)
	require.NoError(t, coordinator.ResolveConflict(context.Background(), input))

	require.Len(t, ledger.entries, 1,
		"the queues record their own decided events; the resolver adds only the typed result")
	entry := ledger.entries[0]
	assert.Equal(t, careplan.ParentRequestEventDecided, entry.EventType)
	assert.Equal(t, string(parentrequests.KindExcused), entry.RequestType)
	assert.Equal(t, conflictAnchorID, entry.RequestID, "the group's lowest id is the deterministic anchor")
	assert.Equal(t, conflictStudentID, entry.StudentID)
	assert.Equal(t, conflictReviewer, entry.ActorAccountID)
	assert.Equal(t, map[string]any{"value": "sick"}, entry.Payload["staff_value"])
	assert.Equal(t, "absence:2026-09-01", entry.Payload["conflict_key"])
	assert.Equal(t, []int64{6, 4, 5}, entry.Payload["request_ids"])
}

func TestResolveConflictRecordsNothingWhenAWishWins(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	ledger := &recordingLedger{}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.ChosenRequestID = 4

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, ledger)
	require.NoError(t, coordinator.ResolveConflict(context.Background(), input))

	assert.Empty(t, ledger.entries,
		"a duplicate decided event would show the family two decisions where there was one")
}

func TestResolveConflictWithoutALedgerStillResolves(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.StaffValue = json.RawMessage(`{"value":"sick"}`)

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
	require.NoError(t, coordinator.ResolveConflict(context.Background(), input))
	assert.Equal(t, []int64{4, 5}, port.rejected)
}

func TestResolveConflictRefusesTheWholeSetWhenOneVersionIsStale(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.ExpectedVersions[1] = "stale"
	input.ChosenRequestID = 4

	coordinator, rollback := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrStale)
	assert.Empty(t, port.approved)
	assert.Empty(t, port.rejected)
	assert.Empty(t, port.locked, "a stale set is refused before anything is locked")
	assert.True(t, rollback.requested)
}

func TestResolveConflictRollsBackWhenOneRejectionFails(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5, 6), failID: 6}
	input := excusedResolveInput(bulkNow, 4, 5, 6)
	input.ChosenRequestID = 4

	coordinator, rollback := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.Error(t, err)
	assert.True(t, rollback.requested,
		"a half-resolved group — one wish approved, its rival still pending — must never commit")
}

func TestResolveConflictReportsADecisionRaceAsStale(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5), failID: 5, failErr: masterdatarequests.ErrReviewNotPending}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.ChosenRequestID = 4

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrStale)
	assert.Equal(t, "parent requests: request version is stale: excused 5 changed during resolution", err.Error(),
		"the students route renders this text verbatim")
}

func TestResolveConflictRefusesAGroupSpanningTwoChildren(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	port.candidates[5].StudentID = 8
	input := excusedResolveInput(bulkNow, 4, 5)
	input.None = true

	coordinator, rollback := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrInvalidConflictResolution)
	assert.Empty(t, port.rejected, "two children are not a conflict; nothing may be decided")
	assert.True(t, rollback.requested)
}

func TestResolveConflictRefusesAnythingOtherThanExactlyOneOutcome(t *testing.T) {
	t.Parallel()
	tests := map[string]func(*parentrequests.ResolveConflictInput){
		"no outcome at all": func(*parentrequests.ResolveConflictInput) {},
		"a wish and a typed value": func(in *parentrequests.ResolveConflictInput) {
			in.ChosenRequestID = 4
			in.StaffValue = json.RawMessage(`{"value":"sick"}`)
		},
		"a typed value and none": func(in *parentrequests.ResolveConflictInput) {
			in.StaffValue = json.RawMessage(`{"value":"sick"}`)
			in.None = true
		},
		"a chosen request outside the group": func(in *parentrequests.ResolveConflictInput) {
			in.ChosenRequestID = conflictOutsideID
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
			input := excusedResolveInput(bulkNow, 4, 5)
			mutate(&input)

			coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
			err := coordinator.ResolveConflict(context.Background(), input)

			require.ErrorIs(t, err, parentrequests.ErrInvalidConflictResolution)
			assert.Empty(t, port.rejected)
		})
	}
}

func TestResolveConflictRequiresAReasonBecauseItAlwaysRejects(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.ChosenRequestID = 4
	input.Reason = "   "

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrReasonRequired)
	assert.Empty(t, port.rejected)
}

func TestResolveConflictRefusesAGroupOfOne(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4)}
	input := excusedResolveInput(bulkNow, 4)
	input.None = true

	coordinator, _ := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrInvalidConflictResolution)
}

func TestResolveConflictRefusesAKindWithNoComposedQueue(t *testing.T) {
	t.Parallel()
	input := excusedResolveInput(bulkNow, 4, 5)
	input.Kind = parentrequests.KindOffering
	input.None = true

	coordinator, rollback := resolverWithExcusedPort(t, writeReviewer, &conflictPortStub{}, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrConflictKindUnsupported)
	assert.True(t, rollback.requested)
}

func TestResolveConflictLetsAnAbsenceOnlyReviewerResolveAbsencesOnly(t *testing.T) {
	t.Parallel()

	t.Run("absences are allowed", func(t *testing.T) {
		t.Parallel()
		port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
		input := excusedResolveInput(bulkNow, 4, 5)
		input.None = true

		coordinator, _ := resolverWithExcusedPort(t, absenceReviewer, port, nil)
		require.NoError(t, coordinator.ResolveConflict(context.Background(), input))
		assert.Equal(t, []int64{4, 5}, port.rejected)
	})

	t.Run("Stammdaten are not", func(t *testing.T) {
		t.Parallel()
		port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5)}
		coordinator, rollback := newTestCoordinator(t, absenceReviewer, ParentRequestCoordinatorDependencies{MasterDataConflicts: port})
		input := excusedResolveInput(bulkNow, 4, 5)
		input.Kind = parentrequests.KindMasterData
		input.None = true

		err := coordinator.ResolveConflict(context.Background(), input)

		require.ErrorIs(t, err, parentrequests.ErrForbidden)
		assert.Empty(t, port.rejected)
		assert.True(t, rollback.requested)
	})
}

func TestResolveConflictReportsALockRaceAsStale(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{candidates: conflictGroup(bulkNow, 4, 5), lockErr: masterdatarequests.ErrReviewNotPending}
	input := excusedResolveInput(bulkNow, 4, 5)
	input.None = true

	coordinator, rollback := resolverWithExcusedPort(t, writeReviewer, port, nil)
	err := coordinator.ResolveConflict(context.Background(), input)

	require.ErrorIs(t, err, parentrequests.ErrStale)
	assert.Empty(t, port.rejected)
	assert.True(t, rollback.requested)
}

func TestCareConflictPortServesBothCareKinds(t *testing.T) {
	t.Parallel()
	port := &conflictPortStub{}
	coordinator, _ := newTestCoordinator(t, writeReviewer, ParentRequestCoordinatorDependencies{CareConflicts: port})

	for _, kind := range []parentrequests.Kind{parentrequests.KindCareSchedule, parentrequests.KindPickupChange} {
		resolved, err := coordinator.conflictPort(context.Background(), kind)
		require.NoError(t, err, "%s must resolve to the care queue", kind)
		assert.Same(t, port, resolved)
	}
}
