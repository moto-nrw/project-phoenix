package careplanhttp

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"
)

// TestTenantSettingKeysMatchTheRegistry pins every key and value this
// resource reads to the Settings Platform's public name: a key renamed in the
// registry must fail here, not resolve as an unknown definition at runtime.
func TestTenantSettingKeysMatchTheRegistry(t *testing.T) {
	t.Parallel()

	assert.Equal(t, settings.KeyAttendanceLogEnabled, settingAttendanceLogEnabled)
	assert.Equal(t, settings.KeyAttendanceVisibleDays, settingAttendanceVisibleDays)
	assert.Equal(t, settings.KeyRoomDetailVisibleDays, settingRoomDetailVisibleDays)
	assert.Equal(t, settings.KeyPrivacyConsentRetentionDays, settingPrivacyConsentRetentionDays)
	assert.Equal(t, settings.KeyEnrollmentBookingsAuthoritative, settingEnrollmentBookingsAuthoritative)
	assert.Equal(t, settings.KeyCareDefaultArrivalTime, settingCareDefaultArrivalTime)
	assert.Equal(t, settings.KeyCareDefaultPickupTime, settingCareDefaultPickupTime)
	assert.Equal(t, settings.KeyParentRequestReasonPolicy, settingParentRequestReasonPolicy)
	assert.Equal(t, settings.ReasonPolicyNobody, parentrequests.ReasonPolicyNobody)
	assert.Equal(t, settings.ReasonPolicyGuardians, parentrequests.ReasonPolicyGuardians)
	assert.Equal(t, settings.ReasonPolicyStaff, parentrequests.ReasonPolicyStaff)
	assert.Equal(t, settings.ReasonPolicyBoth, parentrequests.ReasonPolicyBoth)
	assert.Equal(t, settings.KeySessionEndTime, settingSessionEndTime)
	assert.Equal(t, settings.KeyStudentAbsenceEditScope, settingStudentAbsenceEditScope)
	assert.Equal(t, settings.KeyPresenceMode, settingPresenceMode)
	assert.Equal(t, settings.KeyStudentPhotosEnabled, settingStudentPhotosEnabled)
	assert.Equal(t, settings.KeyClassArrivalExceptionEditors, settingClassArrivalExceptionEditors)
	assert.Equal(t, settings.KeyFeedbackEnabled, settingFeedbackEnabled)
	assert.Equal(t, settings.StudentAbsenceEditScopeAllStaff, studentAbsenceEditScopeAllStaff)
	assert.Equal(t, settings.ClassArrivalExceptionEditorsAllStaff, classArrivalExceptionEditorsAllStaff)
	assert.Equal(t, settings.SchoolPeriodCount, schoolPeriodCount)
	for period := 1; period <= schoolPeriodCount; period++ {
		assert.Equal(t, settings.SchoolPeriodEndKey(period), schoolPeriodEndSetting(period))
	}
}
