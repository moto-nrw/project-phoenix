package owners

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentDeclarationPolicy(source communication.PortalDeclarationPolicy) parentportal.DeclarationPolicy {
	if source == nil {
		return nil
	}
	return parentportal.NewDeclarationPolicy(parentportal.DeclarationPolicyCallbacks{
		CanonicalRecord: func(pA parentportal.DeclarationSubmission) []byte {
			ownerPA := ParentMessagingToOwnerDeclarationSubmission(pA)
			result0 := source.PortalCanonicalRecord(ownerPA)
			return result0
		},
		CanonicalContent: func(pA parentportal.DeclarationVersion) []byte {
			ownerPA := ParentMessagingToOwnerDeclarationVersion(pA)
			result0 := source.PortalCanonicalContent(ownerPA)
			return result0
		},
		Closed: func(deadline *time.Time, now time.Time) bool {
			result0 := source.Closed(deadline, now)
			return result0
		},
		ChildState: func(signers string, deadline *time.Time, eligible []int64, latest map[int64]string, now time.Time) string {
			result0 := source.ChildState(signers, deadline, eligible, latest, now)
			return result0
		},
		AllowedActions: func(revocable bool, deadline *time.Time, canSubmit bool, latest string, now time.Time) []string {
			result0 := source.AllowedActions(revocable, deadline, canSubmit, latest, now)
			return result0
		},
		ActionAllowed: func(revocable bool, deadline *time.Time, canSubmit bool, latest string, action string, now time.Time) bool {
			result0 := source.ActionAllowed(revocable, deadline, canSubmit, latest, action, now)
			return result0
		},
		OwesAction: func(deadline *time.Time, canSubmit bool, latest string, state string, now time.Time) bool {
			result0 := source.OwesAction(deadline, canSubmit, latest, state, now)
			return result0
		},
	})
}
