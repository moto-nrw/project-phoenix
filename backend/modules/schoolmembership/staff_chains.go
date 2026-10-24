package schoolmembership

import (
	"context"
	"fmt"
)

// StaffChainLookup is the slice of the capability the staff chains read.
type StaffChainLookup interface {
	ListStaff(context.Context, StaffFilter) ([]Staff, error)
	ListTeachers(context.Context, TeacherFilter) ([]Teacher, error)
}

// StaffChain is the live staff row behind a person and, when present, the
// live teacher profile of that row.
type StaffChain struct {
	PersonID    int64
	TenantID    int64
	StaffID     int64
	TeacherID   int64
	TeacherRole string
}

// StaffChainsByPerson returns the staff chain of each person, keyed by
// person ID. A person without a live staff row is absent. A person with
// several staff rows keeps the lowest staff ID, and a staff row with several
// teacher profiles the lowest teacher ID, as the former "ORDER BY person_id,
// staff id" join did.
func StaffChainsByPerson(ctx context.Context, lookup StaffChainLookup, personIDs []int64) (map[int64]StaffChain, error) {
	result := make(map[int64]StaffChain, len(personIDs))
	if len(personIDs) == 0 {
		return result, nil
	}
	members, err := lookup.ListStaff(ctx, StaffFilter{PersonIDs: personIDs, MembershipOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load staff for caregiver chains: %w", err)
	}
	if len(members) == 0 {
		return result, nil
	}
	staffIDs := make([]int64, 0, len(members))
	for _, member := range members {
		staffIDs = append(staffIDs, member.ID)
	}
	teachers, err := lookup.ListTeachers(ctx, TeacherFilter{StaffIDs: staffIDs})
	if err != nil {
		return nil, fmt.Errorf("load teachers for caregiver chains: %w", err)
	}
	teacherByStaff := make(map[int64]Teacher, len(teachers))
	for _, teacher := range teachers {
		if current, found := teacherByStaff[teacher.StaffID]; found && current.ID <= teacher.ID {
			continue
		}
		teacherByStaff[teacher.StaffID] = teacher
	}
	// ListStaff sorts by ID, so the first row per person is the lowest one.
	for _, member := range members {
		if _, found := result[member.PersonID]; found {
			continue
		}
		chain := StaffChain{PersonID: member.PersonID, TenantID: member.TenantID, StaffID: member.ID}
		if teacher, found := teacherByStaff[member.ID]; found {
			chain.TeacherID = teacher.ID
			chain.TeacherRole = teacher.Role
		}
		result[member.PersonID] = chain
	}
	return result, nil
}
