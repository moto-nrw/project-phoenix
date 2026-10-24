package communicationtest

import (
	"encoding/json"

	compose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	eventContract "github.com/moto-nrw/project-phoenix/modules/communication/eventcontract"
)

func NewEmitterWithDecisions(events eventContract.Events, notifier eventContract.DecisionNotifier) eventContract.Emitter {
	return compose.NewDecisionEmitter(events, notifier)
}

// ChildEventSnapshot is the test's decoded view of an otherwise opaque detail.
type ChildEventSnapshot struct {
	eventContract.ChildEvent
	Payload map[string]any
}

func SnapshotChildEvent(event eventContract.ChildEvent) ChildEventSnapshot {
	var facts map[string]any
	if event.Payload != nil {
		raw, err := event.Payload.MarshalJSON()
		if err != nil {
			panic(err)
		}
		if err = json.Unmarshal(raw, &facts); err != nil {
			panic(err)
		}
	}
	return ChildEventSnapshot{ChildEvent: event, Payload: facts}
}
