package domain

type OffboardingSnapshot struct {
	StaffID       int64
	From          string
	Absences      []StaffAbsence
	Shifts        []StaffShift
	Series        []StaffShiftSeries
	Substitutions []GroupSubstitution
}
