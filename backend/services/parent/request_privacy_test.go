package parent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

func TestMergeCareExceptions_HidesAnotherGuardiansReason(t *testing.T) {
	t.Parallel()

	authorID := int64(41)
	reason := "Vertraulicher Grund"
	rows := []*scheduleModels.StudentPickupException{{
		ExceptionDate:     timezone.TodayDate(),
		Reason:            &reason,
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &authorID,
	}}

	own := mergeCareExceptions(rows, nil, authorID)
	require.Len(t, own, 1)
	require.NotNil(t, own[0].Reason)
	assert.Equal(t, reason, *own[0].Reason)

	other := mergeCareExceptions(rows, nil, 42)
	require.Len(t, other, 1, "the effective pickup state remains shared")
	assert.Nil(t, other[0].Reason, "the submitting guardian's free text stays private")
}

func TestPendingCareRequest_HidesAnotherGuardiansRequest(t *testing.T) {
	t.Parallel()

	pending := &scheduleModels.CareScheduleChangeRequest{SubmittedBy: 41}
	assert.Nil(t, pendingCareRequest(pending, nil, 42, false))
}

func TestOwnExcusedRequests_HidesAnotherGuardiansRequest(t *testing.T) {
	t.Parallel()

	requests := []*activeModels.ExcusedAbsenceRequest{
		{SubmittedBy: 41, Note: "private"},
		{SubmittedBy: 42, Note: "mine"},
	}

	visible := visibleExcusedRequests(requests, 42, &requestShareVisibility{})
	require.Len(t, visible, 1)
	assert.Equal(t, "mine", visible[0].Note)
}

func TestOfferingDecisionBelongsOnlyToSubmittingGuardian(t *testing.T) {
	t.Parallel()

	decision := &enrollmentService.OfferingChangeDecision{SubmittedBy: 41, Reason: "private"}
	visibility := &requestShareVisibility{}
	assert.Nil(t, visibleOfferingDecision(decision, 42, visibility))
	assert.Same(t, decision, visibleOfferingDecision(decision, 41, visibility))
}
