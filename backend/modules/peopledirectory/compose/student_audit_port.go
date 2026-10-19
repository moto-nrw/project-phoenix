package compose

import (
	applicationBindings "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// The contracts below are what the retained services call when they mutate an
// audited student field; they left services/users with the person and student
// services (#3753). The rules behind them — which fields are tracked,
// how a change reads and where it is stored — belong to the People Directory
// owner and the Audit Platform; #3349 moved them there. The composition root
// binds an implementation (api.NewStudentAuditFor). Reading
// the trail back is the owner's StudentAuditQuery, not part of this contract.

// NewStudentAuditService resolves the authenticated editor and hands every
// recorded change to the owner capability behind recorder.
func NewStudentAuditService(actor peoplePorts.RequestAuditActor, recorder peoplePorts.StudentAuditRecorder) peoplePorts.StudentAuditService {
	if actor == nil {
		panic("request audit actor is required")
	}
	return &applicationBindings.StudentAuditActorPort{Recorder: recorder, Actor: actor}
}
