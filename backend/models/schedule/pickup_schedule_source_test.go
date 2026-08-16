package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pure tests for the source/care_offering_id contract on
// StudentPickupSchedule (#2290): staff rows are the manual default,
// care_offering rows are materialized from an Angebots-Gehzeit and must
// carry the offering reference.

func validPickupScheduleRow() *StudentPickupSchedule {
	return &StudentPickupSchedule{
		StudentID:  7,
		Weekday:    WeekdayMonday,
		PickupTime: time.Date(1, 1, 1, 14, 30, 0, 0, time.UTC),
		CreatedBy:  3,
	}
}

func TestStudentPickupSchedule_Validate_EmptySourceDefaultsToStaff(t *testing.T) {
	row := validPickupScheduleRow()
	require.NoError(t, row.Validate())
	assert.Equal(t, PickupScheduleSourceStaff, row.Source)
}

func TestStudentPickupSchedule_Validate_AcceptsCareOfferingSource(t *testing.T) {
	row := validPickupScheduleRow()
	offeringID := int64(11)
	row.Source = PickupScheduleSourceCareOffering
	row.CareOfferingID = &offeringID
	assert.NoError(t, row.Validate())
}

func TestStudentPickupSchedule_Validate_RejectsUnknownSource(t *testing.T) {
	row := validPickupScheduleRow()
	row.Source = "guardian"
	err := row.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source")
}

func TestStudentPickupSchedule_Validate_CareOfferingSourceRequiresOfferingID(t *testing.T) {
	row := validPickupScheduleRow()
	row.Source = PickupScheduleSourceCareOffering
	err := row.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "care_offering_id")
}

func TestStudentPickupSchedule_Validate_StaffSourceClearsNoOffering(t *testing.T) {
	row := validPickupScheduleRow()
	row.Source = PickupScheduleSourceStaff
	assert.NoError(t, row.Validate())
	assert.Nil(t, row.CareOfferingID)
}
