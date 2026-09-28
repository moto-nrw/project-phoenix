package test

import (
	"github.com/moto-nrw/project-phoenix/models/enrollment"
)

// MaxSchoolGradeLevel mirrors School Structure's highest Jahrgang
// (schoolclass.MaxGradeLevel). The Timetable owner's template suites pass it
// as the grade limit of a template write (#3424 slice S2); neither they nor
// this package may import School Structure's domain.
const MaxSchoolGradeLevel = 13

// CareOffering is the Enrollment care-offering row the Timetable owner's
// template suites seed as a template source (#3424 slice S2).
type CareOffering = enrollment.CareOffering

// NewSourceCareOffering builds an active, optional care offering on fixed
// days with a 14:30 pickup per day, ready for the enrollment service's
// Create. activityGroupID sets the legacy template bridge; nil leaves it
// unlinked.
func NewSourceCareOffering(phaseID int64, name string, days []string, activityGroupID *int64) *CareOffering {
	pickupTimes := make(map[string]string, len(days))
	for _, day := range days {
		pickupTimes[day] = "14:30"
	}
	return &enrollment.CareOffering{
		PhaseID:            phaseID,
		ActivityGroupID:    activityGroupID,
		Name:               name,
		DaysOfWeekMode:     enrollment.DaysOfWeekModeFixed,
		AvailableDays:      days,
		PickupTimes:        pickupTimes,
		IsActive:           true,
		CountsAsCare:       true,
		CountsAsCareSet:    true,
		SelectionRule:      enrollment.SelectionRuleOptional,
		AutoAddGradeLevels: []int{},
	}
}
