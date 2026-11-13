package careplanhttp_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-ogs/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

// TestTenantSettingKeysMatchTheRegistry pins every key and value this
// resource reads to the Settings Platform's public name: a key renamed in the
// registry must fail here, not resolve as an unknown definition at runtime.
func TestTenantSettingKeysMatchTheRegistry(t *testing.T) {
	t.Parallel()

	keys := careplanhttp.TenantSettingKeysForTest
	assert.Equal(t, settings.KeyAttendanceLogEnabled, keys.AttendanceLogEnabled)
	assert.Equal(t, settings.KeyAttendanceVisibleDays, keys.AttendanceVisibleDays)
	assert.Equal(t, settings.KeyRoomDetailVisibleDays, keys.RoomDetailVisibleDays)
	assert.Equal(t, settings.KeyPrivacyConsentRetentionDays, keys.PrivacyConsentRetentionDays)
	assert.Equal(t, settings.KeyEnrollmentBookingsAuthoritative, keys.EnrollmentBookingsAuthoritative)
	assert.Equal(t, settings.KeyCareDefaultArrivalTime, keys.CareDefaultArrivalTime)
	assert.Equal(t, settings.KeyCareDefaultPickupTime, keys.CareDefaultPickupTime)
	assert.Equal(t, settings.KeyParentRequestReasonPolicy, keys.ParentRequestReasonPolicy)
	assert.Equal(t, settings.ReasonPolicyNobody, parentrequests.ReasonPolicyNobody)
	assert.Equal(t, settings.ReasonPolicyGuardians, parentrequests.ReasonPolicyGuardians)
	assert.Equal(t, settings.ReasonPolicyStaff, parentrequests.ReasonPolicyStaff)
	assert.Equal(t, settings.ReasonPolicyBoth, parentrequests.ReasonPolicyBoth)
	assert.Equal(t, settings.KeySessionEndTime, keys.SessionEndTime)
	assert.Equal(t, settings.KeyStudentAbsenceEditScope, keys.StudentAbsenceEditScope)
	assert.Equal(t, settings.KeyPresenceMode, keys.PresenceMode)
	assert.Equal(t, settings.KeyStudentPhotosEnabled, keys.StudentPhotosEnabled)
	assert.Equal(t, settings.KeyClassArrivalExceptionEditors, keys.ClassArrivalExceptionEditors)
	assert.Equal(t, settings.KeyFeedbackEnabled, keys.FeedbackEnabled)
	assert.Equal(t, settings.StudentAbsenceEditScopeAllStaff, keys.StudentAbsenceEditScopeAllStaff)
	assert.Equal(t, settings.ClassArrivalExceptionEditorsAllStaff, keys.ClassArrivalExceptionEditorsAllStaff)
	assert.Equal(t, settings.SchoolPeriodCount, keys.SchoolPeriodCount)
	for period := 1; period <= keys.SchoolPeriodCount; period++ {
		assert.Equal(t, settings.SchoolPeriodEndKey(period), keys.SchoolPeriodEnd(period))
	}
}
