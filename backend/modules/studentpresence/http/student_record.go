package studentpresencehttp

import (
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

const (
	studentStatusPending  = peopleModule.StudentStatusPending
	studentStatusActive   = peopleModule.StudentStatusActive
	studentStatusInactive = peopleModule.StudentStatusInactive
	studentStatusAlumnus  = peopleModule.StudentStatusAlumnus
)

// presenceRecord projects the child onto the Student Presence status-day
// write's view of it.
func presenceRecord(s *peopleModule.StudentState) *studentpresence.StudentRecord {
	if s == nil {
		return nil
	}
	return &studentpresence.StudentRecord{
		ID:            s.ID,
		TenantID:      s.TenantID,
		PersonID:      s.PersonID,
		GroupID:       s.GroupID,
		SchoolClass:   s.SchoolClass,
		Lifecycle:     studentLifecycle(s.Status),
		EnrolledFrom:  s.EnrolledFrom,
		EnrolledUntil: s.EnrolledUntil,
		Sick:          s.Sick,
		SickSince:     s.SickSince,
		Excused:       s.Excused,
		ExcusedSince:  s.ExcusedSince,
	}
}

// studentLifecycle maps the owner's lifecycle status onto the states the
// presence flows branch on. Every other status, today only "pending", is
// StudentLifecycleOther and counts as not-yet-active.
func studentLifecycle(status string) studentpresence.StudentLifecycle {
	switch status {
	case studentStatusActive:
		return studentpresence.StudentLifecycleActive
	case studentStatusInactive:
		return studentpresence.StudentLifecycleInactive
	case studentStatusAlumnus:
		return studentpresence.StudentLifecycleAlumnus
	default:
		return studentpresence.StudentLifecycleOther
	}
}
