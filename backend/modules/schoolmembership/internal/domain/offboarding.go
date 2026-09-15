package domain

import "errors"

var ErrOffboardingConflict = errors.New("staff membership changed since offboarding preview")

type RetirementPreview struct {
	Retirement Retirement
	Revision   string
}

type Retirement struct {
	StaffID          int64
	PersonID         int64
	TeacherID        int64
	GroupAssignments int64
	ClassAssignments int64
}
