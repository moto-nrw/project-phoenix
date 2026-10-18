package parentmessaging

import (
	"context"
	"testing"

	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type transactionalEvents struct{ *fakeEvents }

func (transactionalEvents) InTransaction(context.Context) bool { return true }

type recordingDecisionNotifier struct{ guardians []int64 }

func (r *recordingDecisionNotifier) NotifyRequestDecision(_ context.Context, _, _, guardianAccountID int64, _, _ string, _ int64, _, _ string) error {
	r.guardians = append(r.guardians, guardianAccountID)
	return nil
}

func TestNewEmitterWithDecisionsDeliversDecisionsFromConstruction(t *testing.T) {
	t.Parallel()
	refID := int64(41)
	decision := eventContract.ChildEvent{EventType: eventContract.EventRequestStatus, ActorKind: eventContract.ActorStaff, RequestStatus: eventContract.RequestStatusDone, RefTable: "requests", RefID: &refID}
	events := transactionalEvents{&fakeEvents{guardians: []eventContract.PortalGuardian{{AccountID: 7, PortalLocale: "de"}}}}

	notifier := &recordingDecisionNotifier{}
	require.NoError(t, NewEmitterWithDecisions(events, notifier).EnqueueRequestDecision(context.Background(), 3, 5, 7, decision))
	assert.Equal(t, []int64{7}, notifier.guardians)

	require.NoError(t, NewEmitterWithDecisions(events, nil).EnqueueRequestDecision(context.Background(), 3, 5, 7, decision),
		"without a notifier the emitter delivers no decisions, as NewEmitter")
}
