package carerequests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func TestMandatoryReasonPickupChangeAlwaysRequiresAReason(t *testing.T) {
	t.Parallel()
	studentID, guardianAccountID := int64(41), int64(42)
	day := calendar.Date("2026-10-19")
	pickup := time.Date(2026, 10, 19, 15, 30, 0, 0, time.UTC)

	input := MandatoryReasonPickupChange(studentID, guardianAccountID, day, pickup, "Arzttermin")

	assert.Equal(t, PickupChangeCreateInput{
		StudentID: studentID, GuardianAccountID: guardianAccountID,
		Date: day, PickupTime: pickup, Reason: "Arzttermin", ReasonRequired: true,
	}, input)
	_, err := ValidatePickupChange(MandatoryReasonPickupChange(studentID, guardianAccountID, day, pickup, " "), day)
	assert.ErrorIs(t, err, ErrInvalidPayload)
}
