package care

import (
	"testing"

	"github.com/stretchr/testify/assert"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

func TestPendingCareRequest_HidesAnotherGuardiansRequest(t *testing.T) {
	t.Parallel()

	pending := &scheduleModels.CareScheduleChangeRequest{SubmittedBy: 41}
	assert.Nil(t, pendingCareRequest(pending, nil, 42, false))
}

func TestOfferingDecisionBelongsOnlyToSubmittingGuardian(t *testing.T) {
	t.Parallel()

	decision := &enrollmentService.OfferingChangeDecision{SubmittedBy: 41, Reason: "private"}
	visibility := submitterOnlyVisibility{}
	assert.Nil(t, visibleOfferingDecision(decision, 42, visibility))
	assert.Same(t, decision, visibleOfferingDecision(decision, 41, visibility))
}
