package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The cases below moved here with the consent story (#3349); they previously
// drove services/users.studentConsentService. The projection itself now lives
// with People Directory, the trail with Audit Platform.

type stubStudentConsentTrail struct {
	auditModels.StudentConsentChangeRepository
	entries []*auditModels.StudentConsentChange
	changes []*auditModels.StudentConsentChange
}

func (r *stubStudentConsentTrail) Create(_ context.Context, entry *auditModels.StudentConsentChange) error {
	recorded := *entry
	r.entries = append(r.entries, &recorded)
	return nil
}

func (r *stubStudentConsentTrail) ListByStudentID(_ context.Context, _ int64) ([]*auditModels.StudentConsentChange, error) {
	return r.changes, nil
}

// stubStudentConsentDirectory stands in for the owner projection: the rules
// it applies are covered in modules/peopledirectory; what this seam owns is
// the translation to the retained model types.
type stubStudentConsentDirectory struct {
	states   []peopleModule.StudentConsentState
	snapshot peopleModule.StudentConsentSnapshot
	canPhoto bool
}

func (d *stubStudentConsentDirectory) CurrentStudentConsents(
	_ context.Context,
	snapshot peopleModule.StudentConsentSnapshot,
	canManagePhoto bool,
) ([]peopleModule.StudentConsentState, error) {
	d.snapshot, d.canPhoto = snapshot, canManagePhoto
	return d.states, nil
}

func TestStudentConsentsReturnTheCurrentSharedPortalState(t *testing.T) {
	t.Parallel()

	grantedAt := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	withdrawnAt := time.Date(2026, time.August, 31, 15, 0, 0, 0, time.UTC)
	directory := &stubStudentConsentDirectory{states: []peopleModule.StudentConsentState{
		{Key: peopleModule.StudentConsentAGB, State: peopleModule.StudentConsentStateGranted, ChangedAt: &grantedAt},
		{Key: peopleModule.StudentConsentDataProcessing, State: peopleModule.StudentConsentStateGranted, ChangedAt: &grantedAt},
		{Key: peopleModule.StudentConsentEmailContact, State: peopleModule.StudentConsentStateNotRecorded},
		{Key: peopleModule.StudentConsentPhoto, State: peopleModule.StudentConsentStateWithdrawn, ChangedAt: &withdrawnAt, CanGrant: true},
	}}
	student := &userModels.Student{
		AGBAcceptedAt:            &grantedAt,
		DataProcessingAcceptedAt: &grantedAt,
	}
	student.ID = 91

	states, err := repositories.NewStudentConsentsFor(directory, &stubStudentConsentTrail{}).
		CurrentStates(context.Background(), student, true)

	require.NoError(t, err)
	// The owner sees the row the caller already read, not a second query.
	assert.Equal(t, peopleModule.StudentConsentSnapshot{
		StudentID: 91, AGBAcceptedAt: &grantedAt, DataProcessingAcceptedAt: &grantedAt,
	}, directory.snapshot)
	assert.True(t, directory.canPhoto)
	require.Len(t, states, 4)
	assert.Equal(t, userModels.StudentConsentStateGranted, states[0].State)
	assert.Equal(t, userModels.StudentConsentStateGranted, states[1].State)
	assert.Equal(t, userModels.StudentConsentStateNotRecorded, states[2].State)
	assert.Equal(t, userModels.StudentConsentStateWithdrawn, states[3].State)
	assert.Equal(t, withdrawnAt, *states[3].ChangedAt)
	assert.False(t, states[3].CanWithdraw)
	assert.True(t, states[3].CanGrant)
}

// A student that was never persisted has no consent state to report.
func TestStudentConsentsRejectUnpersistedStudent(t *testing.T) {
	t.Parallel()

	seam := repositories.NewStudentConsentsFor(&stubStudentConsentDirectory{}, &stubStudentConsentTrail{})
	_, err := seam.CurrentStates(context.Background(), &userModels.Student{}, false)
	require.ErrorContains(t, err, "persisted student is required")
	_, err = seam.CurrentStates(context.Background(), nil, false)
	require.ErrorContains(t, err, "persisted student is required")
}

func TestStudentConsentsRecordOnlyBooleanStateTransitions(t *testing.T) {
	t.Parallel()

	trail := &stubStudentConsentTrail{}
	recorder := repositories.NewStudentConsentsFor(&stubStudentConsentDirectory{}, trail)
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
	require.Len(t, trail.entries, 2)
	assert.Equal(t, auditModels.StudentConsentDataProcessing, trail.entries[0].ConsentKey)
	assert.Equal(t, auditModels.StudentConsentGranted, trail.entries[0].Action)
	assert.Equal(t, oldTime, trail.entries[0].CreatedAt)
	assert.Equal(t, auditModels.StudentConsentPhoto, trail.entries[1].ConsentKey)
	assert.Equal(t, auditModels.StudentConsentWithdrawn, trail.entries[1].Action)
	assert.Equal(t, newTime, trail.entries[1].CreatedAt)
	assert.Equal(t, actor, *trail.entries[1].ActorAccountID)
}

func TestStudentConsentsRecordGrantedFieldsOnCreate(t *testing.T) {
	t.Parallel()

	trail := &stubStudentConsentTrail{}
	recorder := repositories.NewStudentConsentsFor(&stubStudentConsentDirectory{}, trail)
	grantedAt := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)
	after := &userModels.Student{EmailContactAcceptedAt: &grantedAt}
	after.ID = 88

	err := recorder.RecordTransitions(
		context.Background(), nil, after,
		auditModels.StudentConsentSourceImport, nil, grantedAt,
	)
	require.NoError(t, err)
	require.Len(t, trail.entries, 1)
	assert.Equal(t, auditModels.StudentConsentEmailContact, trail.entries[0].ConsentKey)
	assert.Nil(t, trail.entries[0].ActorAccountID)
}
