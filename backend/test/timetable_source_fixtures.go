package test

import (
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
)

// MaxSchoolGradeLevel mirrors School Structure's highest Jahrgang
// (schoolclass.MaxGradeLevel). The Timetable owner's template suites pass it
// as the grade limit of a template write (#3424 slice S2); neither they nor
// this package may import School Structure's domain.
const MaxSchoolGradeLevel = 13

// NewSourceCareOffering builds the care-offering row the Timetable owner's
// template suites seed as a template source (#3424 slice S2): active and
// optional on fixed days with a 14:30 pickup per day, ready for
// InsertTestCareOffering once the caller sets its tenant. activityGroupID
// sets the legacy template bridge; nil leaves it unlinked.
func NewSourceCareOffering(phaseID int64, name string, days []string, activityGroupID *int64) *CareOffering {
	pickupTimes := make(map[string]string, len(days))
	for _, day := range days {
		pickupTimes[day] = "14:30"
	}
	return &CareOffering{
		PhaseID:            phaseID,
		ActivityGroupID:    activityGroupID,
		Name:               name,
		DaysOfWeekMode:     enrollmentOwner.DaysOfWeekModeFixed,
		AvailableDays:      days,
		PickupTimes:        pickupTimes,
		IsActive:           true,
		CountsAsCare:       true,
		AutoAddGradeLevels: []int{},
	}
}
