package enrollment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// stubMatchLockRequestRepo backs guardMatchedStudentUnique: it records the
// phase-wide lock acquisition and answers the matched-student existence probe.
// The embedded interface panics on any other call, flagging an unintended
// dependency.
type stubMatchLockRequestRepo struct {
	ChangeRequestIntakeRequests
	lockErr       error
	lockedPhases  []int64
	has           bool
	hasErr        error
	hasStudentIDs []int64
	excludedIDs   []int64
}

func (s *stubMatchLockRequestRepo) AcquireExistingStudentMatchLock(_ context.Context, phaseID int64) error {
	s.lockedPhases = append(s.lockedPhases, phaseID)
	return s.lockErr
}

func (s *stubMatchLockRequestRepo) HasActiveRequestForMatchedStudent(_ context.Context, _, studentID, excludeRequestChildID int64) (bool, error) {
	s.hasStudentIDs = append(s.hasStudentIDs, studentID)
	s.excludedIDs = append(s.excludedIDs, excludeRequestChildID)
	return s.has, s.hasErr
}

// A nil pin is a fresh create — nothing to collide on, so the guard must not
// even acquire the lock or probe the repo.
func TestGuardMatchedStudentUnique_NilPinNoOps(t *testing.T) {
	t.Parallel()

	repo := &stubMatchLockRequestRepo{has: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Requests: repo}}

	err := svc.guardMatchedStudentUnique(context.Background(), 42, nil, 0, 0)
	require.NoError(t, err)
	assert.Empty(t, repo.lockedPhases, "no lock for a nil pin")
	assert.Empty(t, repo.hasStudentIDs, "no probe for a nil pin")
}

// A pin whose student is already targeted by another active request is
// rejected with the dedicated 409 sentinel, after taking the phase lock.
func TestGuardMatchedStudentUnique_CollisionRejected(t *testing.T) {
	t.Parallel()

	repo := &stubMatchLockRequestRepo{has: true}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Requests: repo}}

	pin := int64(555)
	err := svc.guardMatchedStudentUnique(context.Background(), 42, &pin, 0, 1)
	require.ErrorIs(t, err, ErrExistingStudentAlreadyRequested)
	assert.Equal(t, []int64{42}, repo.lockedPhases, "guard must lock the phase before probing")
	assert.Equal(t, []int64{555}, repo.hasStudentIDs)
}

// The common case: the matched student has no other active request, so the
// guard passes (and still serializes via the lock).
func TestGuardMatchedStudentUnique_NoCollisionPasses(t *testing.T) {
	t.Parallel()

	repo := &stubMatchLockRequestRepo{has: false}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Requests: repo}}

	pin := int64(777)
	err := svc.guardMatchedStudentUnique(context.Background(), 9, &pin, 0, 0)
	require.NoError(t, err)
	assert.Equal(t, []int64{9}, repo.lockedPhases)
}

// A lock failure aborts before the probe (fail closed rather than racing).
func TestGuardMatchedStudentUnique_LockErrorPropagates(t *testing.T) {
	t.Parallel()

	repo := &stubMatchLockRequestRepo{lockErr: errors.New("boom")}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Requests: repo}}

	pin := int64(1)
	err := svc.guardMatchedStudentUnique(context.Background(), 3, &pin, 0, 0)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrExistingStudentAlreadyRequested)
	assert.Empty(t, repo.hasStudentIDs, "probe must not run when the lock failed")
}

// A probe failure surfaces as a non-conflict error (do not mask an infra error
// as a business-level duplicate).
func TestGuardMatchedStudentUnique_ProbeErrorPropagates(t *testing.T) {
	t.Parallel()

	repo := &stubMatchLockRequestRepo{hasErr: errors.New("boom")}
	svc := &requestService{RequestServiceConfig: RequestServiceConfig{Requests: repo}}

	pin := int64(1)
	err := svc.guardMatchedStudentUnique(context.Background(), 3, &pin, 0, 0)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrExistingStudentAlreadyRequested)
}

// sameSubmittedIdentity underpins the edit-path pin-drop: replacement edits
// pair by persisted ID, so an edited name/birthday must be detected as a
// different child even though it keeps the same row.
func TestSameSubmittedIdentity(t *testing.T) {
	t.Parallel()

	existing := &enrollmentModels.RequestChild{
		FirstName:   "Anna",
		LastName:    "Müller",
		DateOfBirth: "2019-04-12",
	}

	t.Run("unchanged identity matches (case/space-insensitive)", func(t *testing.T) {
		assert.True(t, sameSubmittedIdentity(existing, SubmitChild{
			FirstName:   "  anna ",
			LastName:    "MÜLLER",
			DateOfBirth: timezone.NewDate(2019, 4, 12),
		}))
	})

	t.Run("edited name is a different child", func(t *testing.T) {
		assert.False(t, sameSubmittedIdentity(existing, SubmitChild{
			FirstName:   "Ben",
			LastName:    "Schmidt",
			DateOfBirth: timezone.NewDate(2019, 4, 12),
		}))
	})

	t.Run("edited birthday is a different child", func(t *testing.T) {
		assert.False(t, sameSubmittedIdentity(existing, SubmitChild{
			FirstName:   "Anna",
			LastName:    "Müller",
			DateOfBirth: timezone.NewDate(2020, 1, 1),
		}))
	})

	t.Run("nil existing never matches", func(t *testing.T) {
		assert.False(t, sameSubmittedIdentity(nil, SubmitChild{
			FirstName:   "Anna",
			LastName:    "Müller",
			DateOfBirth: timezone.NewDate(2019, 4, 12),
		}))
	})
}
