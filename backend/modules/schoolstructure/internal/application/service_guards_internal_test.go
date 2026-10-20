package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingStore counts every persistence call, so the guard tests can prove
// that invalid input never reaches persistence. It echoes ids; they are not
// database rows.
type recordingStore struct {
	findID  int64
	listIDs []int64
	calls   int
}

func newRecordingService(store *recordingStore) *Service {
	return New(store, NewTransitionRetention(store), func(ports.Observation) {})
}

func (s *recordingStore) List(context.Context, int64, int) ([]domain.Group, domain.OperationStats, error) {
	s.calls++
	return nil, domain.OperationStats{}, nil
}

func (s *recordingStore) FindByID(_ context.Context, id int64) (domain.Group, bool, domain.OperationStats, error) {
	s.calls++
	s.findID = id
	return domain.Group{ID: id, Name: "Igel"}, true, domain.OperationStats{}, nil
}

func (s *recordingStore) ListByIDs(_ context.Context, ids []int64) ([]domain.Group, domain.OperationStats, error) {
	s.calls++
	s.listIDs = ids
	result := make([]domain.Group, 0, len(ids))
	for _, id := range ids {
		result = append(result, domain.Group{ID: id, Name: "Igel"})
	}
	return result, domain.OperationStats{}, nil
}

func (s *recordingStore) CountStudentTransitionHistory(context.Context, int64, int64) (int, domain.OperationStats, error) {
	s.calls++
	return 0, domain.OperationStats{}, nil
}

func (s *recordingStore) AnonymizeStudentTransitionHistory(context.Context, int64, int64) (int64, domain.OperationStats, error) {
	s.calls++
	return 0, domain.OperationStats{}, nil
}

func (s *recordingStore) FindTransition(_ context.Context, _, id int64, _ string) (schoolstructure.Transition, bool, domain.OperationStats, error) {
	s.calls++
	return schoolstructure.Transition{ID: id, Status: schoolstructure.TransitionStatusDraft}, true, domain.OperationStats{}, nil
}

func (s *recordingStore) ListTransitions(context.Context, int64, schoolstructure.TransitionFilter) ([]schoolstructure.Transition, int, domain.OperationStats, error) {
	s.calls++
	return nil, 0, domain.OperationStats{}, nil
}

func (s *recordingStore) InsertTransition(_ context.Context, _ int64, draft schoolstructure.TransitionDraft) (schoolstructure.Transition, domain.OperationStats, error) {
	s.calls++
	return schoolstructure.Transition{AcademicYear: draft.AcademicYear, CreatedBy: draft.CreatedBy}, domain.OperationStats{}, nil
}

func (s *recordingStore) UpdateTransitionFields(context.Context, int64, int64, string, *string) (int64, domain.OperationStats, error) {
	s.calls++
	return 1, domain.OperationStats{}, nil
}

func (s *recordingStore) ReplaceMappings(context.Context, int64, int64, []schoolstructure.TransitionMappingInput) ([]schoolstructure.TransitionMapping, domain.OperationStats, error) {
	s.calls++
	return nil, domain.OperationStats{}, nil
}

func (s *recordingStore) DeleteTransition(context.Context, int64, int64) (int64, domain.OperationStats, error) {
	s.calls++
	return 1, domain.OperationStats{}, nil
}

func (s *recordingStore) DeleteTransitionsEndedBefore(context.Context, int64, time.Time) (int64, domain.OperationStats, error) {
	s.calls++
	return 0, domain.OperationStats{}, nil
}

func (s *recordingStore) LockTransitions(context.Context, int64) (domain.OperationStats, error) {
	s.calls++
	return domain.OperationStats{}, nil
}

func (s *recordingStore) LockLatestApplied(context.Context, int64) (schoolstructure.Transition, bool, domain.OperationStats, error) {
	s.calls++
	return schoolstructure.Transition{}, false, domain.OperationStats{}, nil
}

func (s *recordingStore) MarkApplied(context.Context, int64, int64, int64, time.Time, *int64) (int64, domain.OperationStats, error) {
	s.calls++
	return 1, domain.OperationStats{}, nil
}

func (s *recordingStore) MarkReverted(context.Context, int64, int64, int64, time.Time) (int64, domain.OperationStats, error) {
	s.calls++
	return 1, domain.OperationStats{}, nil
}

func (s *recordingStore) InsertHistory(context.Context, int64, []schoolstructure.TransitionHistoryEntry) (domain.OperationStats, error) {
	s.calls++
	return domain.OperationStats{}, nil
}

func (s *recordingStore) ListHistory(context.Context, int64, int64) ([]schoolstructure.TransitionHistoryEntry, domain.OperationStats, error) {
	s.calls++
	return nil, domain.OperationStats{}, nil
}

func (s *recordingStore) InsertClassTeacherLedger(context.Context, int64, []schoolstructure.TransitionClassTeacherEntry) (domain.OperationStats, error) {
	s.calls++
	return domain.OperationStats{}, nil
}

func (s *recordingStore) ListClassTeacherLedger(context.Context, int64, int64) ([]schoolstructure.TransitionClassTeacherEntry, domain.OperationStats, error) {
	s.calls++
	return nil, domain.OperationStats{}, nil
}

func (s *recordingStore) InsertClassListLedger(context.Context, int64, []schoolstructure.TransitionClassListEntry) (domain.OperationStats, error) {
	s.calls++
	return domain.OperationStats{}, nil
}

func (s *recordingStore) ListClassListLedger(context.Context, int64, int64) ([]schoolstructure.TransitionClassListEntry, domain.OperationStats, error) {
	s.calls++
	return nil, domain.OperationStats{}, nil
}

func TestServiceRejectsInvalidListingLimitsBeforeReachingTheStore(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	service := newRecordingService(store)
	for _, limit := range []int{-1, 0, 1001} {
		_, err := service.ListGroups(t.Context(), limit)
		require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)
	}
	assert.Zero(t, store.calls)
}

func TestServiceRejectsInvalidIdentifiersBeforeReachingTheStore(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	service := newRecordingService(store)
	ctx := context.Background()

	_, err := service.FindGroup(ctx, 0)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)

	_, err = service.ListGroupsByID(ctx, []int64{7, -1})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidGroup)

	assert.Zero(t, store.calls, "invalid input must never reach persistence")
}

func TestServiceAnswersEmptyListWithoutPersistence(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	service := newRecordingService(store)

	groups, err := service.ListGroupsByID(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, groups)
	assert.Zero(t, store.calls)
}

func TestServiceForwardsValidReadsToTheStore(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	service := newRecordingService(store)
	ctx := context.Background()

	group, err := service.FindGroup(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, int64(42), store.findID)
	assert.Equal(t, "Igel", group.Name)

	groups, err := service.ListGroupsByID(ctx, []int64{3, 5})
	require.NoError(t, err)
	assert.Equal(t, []int64{3, 5}, store.listIDs)
	assert.Len(t, groups, 2)
}

func TestNewPanicsWithoutDependencies(t *testing.T) {
	t.Parallel()
	assert.Panics(t, func() { New(nil, nil, nil) })
}

func TestTransitionServiceRejectsInvalidInputBeforeReachingTheStore(t *testing.T) {
	t.Parallel()
	store := &recordingStore{}
	service := newRecordingService(store)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	_, err := service.FindTransition(ctx, 0)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	_, _, err = service.ListTransitions(ctx, schoolstructure.TransitionFilter{Limit: -1})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	negativeAfterID := int64(-1)
	_, _, err = service.ListTransitions(ctx, schoolstructure.TransitionFilter{AfterID: &negativeAfterID})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	_, err = service.ListTransitionHistory(ctx, -3)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	_, err = service.CreateTransition(ctx, schoolstructure.TransitionDraft{AcademicYear: "2026-2027"})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition, "created_by is required")
	_, err = service.UpdateTransition(ctx, schoolstructure.TransitionUpdate{})
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	require.ErrorIs(t, service.DeleteTransition(ctx, 0), schoolstructure.ErrInvalidTransition)
	_, err = service.LockTransitionForMutation(ctx, 0)
	require.ErrorIs(t, err, schoolstructure.ErrInvalidTransition)
	require.ErrorIs(t, service.MarkTransitionApplied(ctx, 1, 0, now, nil), schoolstructure.ErrInvalidTransition)
	require.ErrorIs(t, service.MarkTransitionReverted(ctx, 1, 7, time.Time{}), schoolstructure.ErrInvalidTransition)
	assert.Zero(t, store.calls, "invalid input must never reach persistence")

	require.NoError(t, service.AppendTransitionHistory(ctx, nil))
	require.NoError(t, service.AppendTransitionClassTeacherLedger(ctx, nil))
	require.NoError(t, service.AppendTransitionClassListLedger(ctx, nil))
	assert.Zero(t, store.calls, "an empty append is answered without persistence")
}
