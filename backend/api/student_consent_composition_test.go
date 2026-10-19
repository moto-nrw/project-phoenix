package api

import (
	"context"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The cases below pin the consent seam of this package (#3349): the
// projection lives with People Directory, the trail with Audit Platform, and
// the translation to the retained model types here.

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
	student := &peopleModule.StudentState{
		AGBAcceptedAt:            &grantedAt,
		DataProcessingAcceptedAt: &grantedAt,
	}
	student.ID = 91

	states, err := owners.NewStudentConsentsFor(directory, &stubStudentConsentTrail{}).
		CurrentStates(context.Background(), student, true)

	require.NoError(t, err)
	// The owner sees the row the caller already read, not a second query.
	assert.Equal(t, peopleModule.StudentConsentSnapshot{
		StudentID: 91, AGBAcceptedAt: &grantedAt, DataProcessingAcceptedAt: &grantedAt,
	}, directory.snapshot)
	assert.True(t, directory.canPhoto)
	require.Len(t, states, 4)
	assert.Equal(t, peopleModule.StudentConsentStateGranted, states[0].State)
	assert.Equal(t, peopleModule.StudentConsentStateGranted, states[1].State)
	assert.Equal(t, peopleModule.StudentConsentStateNotRecorded, states[2].State)
	assert.Equal(t, peopleModule.StudentConsentStateWithdrawn, states[3].State)
	assert.Equal(t, withdrawnAt, *states[3].ChangedAt)
	assert.False(t, states[3].CanWithdraw)
	assert.True(t, states[3].CanGrant)
}

// A student that was never persisted has no consent state to report.
func TestStudentConsentsRejectUnpersistedStudent(t *testing.T) {
	t.Parallel()

	seam := owners.NewStudentConsentsFor(&stubStudentConsentDirectory{}, &stubStudentConsentTrail{})
	_, err := seam.CurrentStates(context.Background(), &peopleModule.StudentState{}, false)
	require.ErrorContains(t, err, "persisted student is required")
	_, err = seam.CurrentStates(context.Background(), nil, false)
	require.ErrorContains(t, err, "persisted student is required")
}

func TestStudentConsentsRecordOnlyBooleanStateTransitions(t *testing.T) {
	t.Parallel()

	trail := &stubStudentConsentTrail{}
	recorder := owners.NewStudentConsentsFor(&stubStudentConsentDirectory{}, trail)
	oldTime := time.Date(2025, time.September, 1, 8, 0, 0, 0, time.UTC)
	newTime := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)
	actor := int64(42)
	before := &peopleModule.StudentState{
		AGBAcceptedAt:          &oldTime,
		PhotoConsentGivenAt:    &oldTime,
		EmailContactAcceptedAt: nil,
	}
	before.ID = 77
	after := &peopleModule.StudentState{
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
	recorder := owners.NewStudentConsentsFor(&stubStudentConsentDirectory{}, trail)
	grantedAt := time.Date(2026, time.August, 31, 9, 0, 0, 0, time.UTC)
	after := &peopleModule.StudentState{EmailContactAcceptedAt: &grantedAt}
	after.ID = 88

	err := recorder.RecordTransitions(
		context.Background(), nil, after,
		"import", nil, grantedAt,
	)
	require.NoError(t, err)
	require.Len(t, trail.entries, 1)
	assert.Equal(t, auditModels.StudentConsentEmailContact, trail.entries[0].ConsentKey)
	assert.Nil(t, trail.entries[0].ActorAccountID)
}
