package parentportal_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/stretchr/testify/assert"
)

// TestExcusedRequestContractMatchesPersistedVocabulary pins the persisted
// values the Care Plan excused-absence contract (#3093) re-declares without a
// compile-time link: the status-day columns the Student Presence models still
// own, and the ledger and pill vocabulary of the People Directory models.
// A drift here would silently split one table between two spellings.
func TestExcusedRequestContractMatchesPersistedVocabulary(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	protocol := api.ParentPersistedFixtureProtocol(owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)))

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

	assert.Equal(t, protocol.ParentRequestTypeExcusedAbsence, careplan.ParentRequestTypeExcusedAbsence)
	assert.Equal(t, protocol.ParentRequestEventSubmitted, careplan.ParentRequestEventSubmitted)
	assert.Equal(t, protocol.ParentRequestEventGuardianEdit, careplan.ParentRequestEventGuardianEdit)
	assert.Equal(t, protocol.ParentRequestEventDecided, careplan.ParentRequestEventDecided)
	assert.Equal(t, protocol.ParentRequestEventMarkedDone, careplan.ParentRequestEventMarkedDone)
	assert.Equal(t, protocol.ParentRequestEventCorrected, careplan.ParentRequestEventCorrected)

	assert.Equal(t, protocol.ParentMessageEventRequestCreated, careplan.ParentMessageEventRequestCreated)
	assert.Equal(t, protocol.ParentMessageEventRequestStatus, careplan.ParentMessageEventRequestStatus)
	assert.Equal(t, protocol.ParentMessageSenderGuardian, careplan.ParentMessageActorGuardian)
	assert.Equal(t, protocol.ParentMessageSenderStaff, careplan.ParentMessageActorStaff)
	assert.Equal(t, protocol.ParentMessageRequestStatusOpen, careplan.ParentMessageRequestStatusOpen)
	assert.Equal(t, protocol.ParentMessageRequestStatusDone, careplan.ParentMessageRequestStatusDone)
	assert.Equal(t, protocol.ParentMessageRequestStatusRejected, careplan.ParentMessageRequestStatusReject)
	assert.Equal(t, protocol.ParentMessageRequestExcusedAbsence, careplan.ParentMessageRequestExcused)
	assert.Equal(t, protocol.ParentMessageRequestSickAbsence, careplan.ParentMessageRequestSick)
}
