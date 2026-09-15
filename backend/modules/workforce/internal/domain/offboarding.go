package domain

import "errors"

var ErrOffboardingConflict = errors.New("workforce changed since offboarding preview")
var ErrOffboardingInUse = errors.New("staff has an active or future group handover")

type OffboardingCounts struct {
	Absences      int64
	Shifts        int64
	Series        int64
	Substitutions int64
}

type OffboardingPreview struct {
	Counts   OffboardingCounts
	Revision string
	Blocked  bool
}

type OffboardingSnapshot struct {
	StaffID       int64
	From          string
	Absences      []StaffAbsence
	Shifts        []StaffShift
	Series        []StaffShiftSeries
	Substitutions []GroupSubstitution
}
