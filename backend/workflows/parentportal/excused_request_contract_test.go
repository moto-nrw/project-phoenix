package parentportal_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// TestExcusedRequestContractMatchesPersistedVocabulary pins the persisted
// values the Care Plan excused-absence contract (#3093) re-declares without a
// compile-time link: the status-day columns the Student Presence models still
// own, and the ledger and pill vocabulary of the People Directory models.
// A drift here would silently split one table between two spellings.
func TestExcusedRequestContractMatchesPersistedVocabulary(t *testing.T) {
	t.Parallel()

	assert.Equal(t, absencerecords.StudentStatusDaySick, careplan.StudentStatusDaySick)
	assert.Equal(t, absencerecords.StudentStatusDayExcused, careplan.StudentStatusDayExcused)
	assert.Equal(t, absencerecords.StudentStatusDayClassTrip, careplan.StudentStatusDayClassTrip)
	assert.Equal(t, absencerecords.StudentStatusDayPresent, careplan.StudentStatusDayPresent)
	assert.Equal(t, absencerecords.StudentStatusSourceManual, careplan.StudentStatusSourceManual)
	assert.Equal(t, absencerecords.StudentStatusSourceParent, careplan.StudentStatusSourceParent)
	assert.Equal(t, absencerecords.StudentStatusDayStatuses(), careplan.StudentStatusDayStatusesExcept(""))

	assert.Equal(t, absencerecords.ExcusedRequestStatusPending, careplan.ExcusedRequestStatusPending)
	assert.Equal(t, absencerecords.ExcusedRequestStatusApproved, careplan.ExcusedRequestStatusApproved)
	assert.Equal(t, absencerecords.ExcusedRequestStatusRejected, careplan.ExcusedRequestStatusRejected)
	assert.Equal(t, absencerecords.ExcusedRequestStatusWithdrawn, careplan.ExcusedRequestStatusWithdrawn)
	assert.Equal(t, absencerecords.ExcusedRequestStatusDone, careplan.ExcusedRequestStatusDone)
	assert.Equal(t, absencerecords.ExcusedRequestStatusCareEnded, careplan.ExcusedRequestStatusCareEnded)

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
