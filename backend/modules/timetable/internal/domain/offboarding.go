package domain

import (
	"errors"
	"time"
)

var ErrOffboardingConflict = errors.New("timetable changed since offboarding preview")

type OffboardingCounts struct {
	InstanceAssignments int64
	PlannedSupervisors  int64
}

type OffboardingPreview struct {
	Counts   OffboardingCounts
	Revision string
}

type OffboardingInstance struct {
	Assignment        InstanceStaff
	Date              string
	Status            string
	InstanceUpdatedAt time.Time
}

type OffboardingSnapshot struct {
	StaffID     int64
	From        string
	Instances   []OffboardingInstance
	Supervisors []PlannedSupervisor
}
