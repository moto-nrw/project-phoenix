package care

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

func TestPendingCareRequest_HidesAnotherGuardiansRequest(t *testing.T) {
	t.Parallel()

	pending := &carerequests.Request{SubmittedBy: 41}
	assert.Nil(t, pendingCareRequest(pending, nil, 42, false))
}

func TestOfferingDecisionBelongsOnlyToSubmittingGuardian(t *testing.T) {
	t.Parallel()

	decision := &careplan.OfferingChangeDecision{SubmittedBy: 41, Reason: "private"}
	visibility := submitterOnlyVisibility{}
	assert.Nil(t, visibleOfferingDecision(decision, 42, visibility))
	assert.Same(t, decision, visibleOfferingDecision(decision, 41, visibility))
}
