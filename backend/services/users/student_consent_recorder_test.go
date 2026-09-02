package users

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

type recordingStudentConsentRepo struct {
	auditModels.StudentConsentChangeRepository
	entries []*auditModels.StudentConsentChange
	changes []*auditModels.StudentConsentChange
}

func (r *recordingStudentConsentRepo) Create(_ context.Context, entry *auditModels.StudentConsentChange) error {
	copy := *entry
	r.entries = append(r.entries, &copy)
	return nil
}

func (r *recordingStudentConsentRepo) ListByStudentID(_ context.Context, _ int64) ([]*auditModels.StudentConsentChange, error) {
	return r.changes, nil
}

func TestStudentConsentServiceReturnsTheCurrentSharedPortalState(t *testing.T) {
	t.Parallel()

	grantedAt := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	withdrawnAt := time.Date(2026, time.August, 31, 15, 0, 0, 0, time.UTC)
	repo := &recordingStudentConsentRepo{changes: []*auditModels.StudentConsentChange{
		{
			ConsentKey: auditModels.StudentConsentPhoto,
			Action:     auditModels.StudentConsentWithdrawn,
		},
	}}
	repo.changes[0].CreatedAt = withdrawnAt
	student := &userModels.Student{
		AGBAcceptedAt:            &grantedAt,
		DataProcessingAcceptedAt: &grantedAt,
	}
	student.ID = 91

	states, err := NewStudentConsentService(repo).CurrentStates(
		context.Background(),
		student,
		true,
	)

	require.NoError(t, err)
	require.Len(t, states, 4)
	assert.Equal(t, StudentConsentStateGranted, states[0].State)
	assert.Equal(t, StudentConsentStateGranted, states[1].State)
	assert.Equal(t, StudentConsentStateNotRecorded, states[2].State)
	assert.Equal(t, StudentConsentStateWithdrawn, states[3].State)
	assert.Equal(t, withdrawnAt, *states[3].ChangedAt)
	assert.False(t, states[3].CanWithdraw)
	assert.True(t, states[3].CanGrant)
}

func TestStudentConsentRecorderRecordsOnlyBooleanStateTransitions(t *testing.T) {
	t.Parallel()

	repo := &recordingStudentConsentRepo{}
	recorder := NewStudentConsentService(repo)
	oldTime := time.Date(2025, time.September, 1, 8, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)
	actor := int64(42)
	before := &userModels.Student{
		AGBAcceptedAt:          &oldTime,
		PhotoConsentGivenAt:    &oldTime,
		EmailContactAcceptedAt: nil,
	}
	before.ID = 77
	after := &userModels.Student{
		AGBAcceptedAt:            &newTime, // changed timestamp, same granted state
		DataProcessingAcceptedAt: &oldTime,
		PhotoConsentGivenAt:      nil,
	}
	after.ID = 77

	err := recorder.RecordTransitions(
		context.Background(),
		before,
		after,
		auditModels.StudentConsentSourceTenantPortal,
		&actor,
		newTime,
	)
	require.NoError(t, err)
	require.Len(t, repo.entries, 2)
	assert.Equal(t, auditModels.StudentConsentDataProcessing, repo.entries[0].ConsentKey)
	assert.Equal(t, auditModels.StudentConsentGranted, repo.entries[0].Action)
	assert.Equal(t, oldTime, repo.entries[0].CreatedAt)
	assert.Equal(t, auditModels.StudentConsentPhoto, repo.entries[1].ConsentKey)
	assert.Equal(t, auditModels.StudentConsentWithdrawn, repo.entries[1].Action)
	assert.Equal(t, newTime, repo.entries[1].CreatedAt)
	assert.Equal(t, actor, *repo.entries[1].ActorAccountID)
}

func TestStudentConsentRecorderRecordsGrantedFieldsOnCreate(t *testing.T) {
	t.Parallel()

	repo := &recordingStudentConsentRepo{}
	recorder := NewStudentConsentService(repo)
	grantedAt := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)
	after := &userModels.Student{EmailContactAcceptedAt: &grantedAt}
	after.ID = 88

	err := recorder.RecordTransitions(
		context.Background(), nil, after,
		auditModels.StudentConsentSourceImport, nil, grantedAt,
	)
	require.NoError(t, err)
	require.Len(t, repo.entries, 1)
	assert.Equal(t, auditModels.StudentConsentEmailContact, repo.entries[0].ConsentKey)
	assert.Nil(t, repo.entries[0].ActorAccountID)
}
