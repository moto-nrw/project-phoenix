package parent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// TestExcusedRequestContractMatchesPersistedVocabulary pins the persisted
// values the Care Plan excused-absence contract (#3093) re-declares without a
// compile-time link: the status-day columns the Student Presence models still
// own, and the ledger and pill vocabulary of the People Directory models.
// A drift here would silently split one table between two spellings.
func TestExcusedRequestContractMatchesPersistedVocabulary(t *testing.T) {
	t.Parallel()

	assert.Equal(t, activeModels.StudentStatusDaySick, careplan.StudentStatusDaySick)
	assert.Equal(t, activeModels.StudentStatusDayExcused, careplan.StudentStatusDayExcused)
	assert.Equal(t, activeModels.StudentStatusDayClassTrip, careplan.StudentStatusDayClassTrip)
	assert.Equal(t, activeModels.StudentStatusDayPresent, careplan.StudentStatusDayPresent)
	assert.Equal(t, activeModels.StudentStatusSourceManual, careplan.StudentStatusSourceManual)
	assert.Equal(t, activeModels.StudentStatusSourceParent, careplan.StudentStatusSourceParent)
	assert.Equal(t, activeModels.StudentStatusDayStatuses(), careplan.StudentStatusDayStatusesExcept(""))

	assert.Equal(t, activeModels.ExcusedRequestStatusPending, careplan.ExcusedRequestStatusPending)
	assert.Equal(t, activeModels.ExcusedRequestStatusApproved, careplan.ExcusedRequestStatusApproved)
	assert.Equal(t, activeModels.ExcusedRequestStatusRejected, careplan.ExcusedRequestStatusRejected)
	assert.Equal(t, activeModels.ExcusedRequestStatusWithdrawn, careplan.ExcusedRequestStatusWithdrawn)
	assert.Equal(t, activeModels.ExcusedRequestStatusDone, careplan.ExcusedRequestStatusDone)
	assert.Equal(t, activeModels.ExcusedRequestStatusCareEnded, careplan.ExcusedRequestStatusCareEnded)

	assert.Equal(t, usersModels.ParentRequestTypeExcusedAbsence, careplan.ParentRequestTypeExcusedAbsence)
	assert.Equal(t, usersModels.ParentRequestEventSubmitted, careplan.ParentRequestEventSubmitted)
	assert.Equal(t, usersModels.ParentRequestEventGuardianEdit, careplan.ParentRequestEventGuardianEdit)
	assert.Equal(t, usersModels.ParentRequestEventDecided, careplan.ParentRequestEventDecided)
	assert.Equal(t, usersModels.ParentRequestEventMarkedDone, careplan.ParentRequestEventMarkedDone)
	assert.Equal(t, usersModels.ParentRequestEventCorrected, careplan.ParentRequestEventCorrected)

	assert.Equal(t, usersModels.ParentMessageEventRequestCreated, careplan.ParentMessageEventRequestCreated)
	assert.Equal(t, usersModels.ParentMessageEventRequestStatus, careplan.ParentMessageEventRequestStatus)
	assert.Equal(t, usersModels.ParentMessageSenderGuardian, careplan.ParentMessageActorGuardian)
	assert.Equal(t, usersModels.ParentMessageSenderStaff, careplan.ParentMessageActorStaff)
	assert.Equal(t, usersModels.ParentMessageRequestStatusOpen, careplan.ParentMessageRequestStatusOpen)
	assert.Equal(t, usersModels.ParentMessageRequestStatusDone, careplan.ParentMessageRequestStatusDone)
	assert.Equal(t, usersModels.ParentMessageRequestStatusRejected, careplan.ParentMessageRequestStatusReject)
	assert.Equal(t, usersModels.ParentMessageRequestExcusedAbsence, careplan.ParentMessageRequestExcused)
	assert.Equal(t, usersModels.ParentMessageRequestSickAbsence, careplan.ParentMessageRequestSick)
}
