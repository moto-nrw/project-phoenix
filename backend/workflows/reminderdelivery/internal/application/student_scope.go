// Package application computes visual-only staff reminders (issue #1457):
// upcoming pickups, overdue pickups, and activity starts. There is no sound and
// no push — the data is derived on request from schedules already in the system
// and rendered on the staff "Erinnerungen" page. All thresholds (which types
// are on, lead-time minutes) are tenant settings resolved at request time.
package application

import (
	"context"
	"fmt"
	"maps"
	"slices"

	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/ports"
)

func (s *service) pickupScopeStudentIDs(ctx context.Context, scope reminder.Scope, today string) ([]int64, error) {
	if scope.IsAdmin {
		return s.presentStudentIDsFromAttendance(ctx, today)
	}

	binary, err := s.binaryMode(ctx)
	if err != nil {
		return nil, err
	}
	if binary {
		return s.presentStudentIDsFromAttendance(ctx, today)
	}

	roomIDs, err := s.supervisedRoomIDs(ctx, scope)
	if err != nil {
		return nil, err
	}
	return s.presentStudentsInRooms(ctx, roomIDs)
}

// presentStudentIDsFromAttendance lists all students with an open attendance
// row today. Used for admins (all present children) and for caregivers in
// binary mode, where room-scoped visit rows do not exist.
func (s *service) presentStudentIDsFromAttendance(ctx context.Context, today string) ([]int64, error) {
	if s.Attendance == nil {
		return nil, nil
	}
	return s.Attendance.ListOpenAttendanceStudentIDs(ctx, today)
}

// binaryMode reports whether the tenant runs the binary presence mode, in which
// CreateVisit is a no-op and presence lives in active.attendance rather than
// active.visits. A settings resolution error is surfaced, consistent with the
// rest of the service — a broken read must not silently pick a presence source.
func (s *service) binaryMode(ctx context.Context) (bool, error) {
	if s.Settings == nil {
		return false, nil
	}
	return s.Settings.BinaryPresence(ctx)
}

// supervisedRoomIDs returns the room IDs the caregiver currently supervises.
func (s *service) supervisedRoomIDs(ctx context.Context, scope reminder.Scope) ([]int64, error) {
	if s.Supervision == nil {
		return nil, nil
	}

	supervisions, err := s.Supervision.GetStaffActiveSupervisions(ctx, scope.StaffID)
	if err != nil {
		return nil, err
	}
	groupIDs := make([]int64, 0, len(supervisions))
	for _, sup := range supervisions {
		if sup != nil {
			groupIDs = append(groupIDs, sup.GroupID)
		}
	}
	if len(groupIDs) == 0 {
		return nil, nil
	}

	groups, err := s.Supervision.GetActiveGroupsByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	roomSet := make(map[int64]struct{}, len(groups))
	for _, g := range groups {
		if g != nil {
			roomSet[g.RoomID] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(roomSet)), nil
}

// presentStudentsInRooms returns the deduplicated IDs of students currently
// present in any of the given rooms.
func (s *service) presentStudentsInRooms(ctx context.Context, roomIDs []int64) ([]int64, error) {
	if len(roomIDs) == 0 {
		return nil, nil
	}
	byRoom, err := s.Visits.ListOpenVisitStudentIDsByRoom(ctx)
	if err != nil {
		return nil, err
	}
	studentSet := make(map[int64]struct{})
	for _, roomID := range roomIDs {
		for _, id := range byRoom[roomID] {
			studentSet[id] = struct{}{}
		}
	}
	return slices.Collect(maps.Keys(studentSet)), nil
}

func (s *service) readableStudents(ctx context.Context, scope reminder.Scope, ids []int64) (map[int64]*ports.Student, error) {
	if s.Student == nil {
		// Without a student reader we can neither verify read access nor build a
		// title. Fail closed (return nothing) rather than expose unverified data.
		return map[int64]*ports.Student{}, nil
	}

	students, err := s.Student.FindReadScopeByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}

	allowed, err := s.pickupReadPredicate(ctx, scope)
	if err != nil {
		return nil, err
	}

	readable := make(map[int64]*ports.Student, len(students))
	for id, st := range students {
		if st == nil || !allowed(st) {
			continue
		}
		readable[id] = st
	}
	return readable, nil
}

// hydrateNames builds the display name + school class for the given (already
// read-gated) students. Only the due subset is hydrated: the pickup-reminder API
// contract promises a student name as the title, so a failed person read must
// fail the request rather than emit unusable reminders with empty titles.
func (s *service) hydrateNames(ctx context.Context, readable map[int64]*ports.Student, ids []int64) (map[int64]studentNameInfo, error) {
	result := make(map[int64]studentNameInfo, len(ids))

	personIDs := make([]int64, 0, len(ids))
	for _, id := range ids {
		if st := readable[id]; st != nil {
			personIDs = append(personIDs, st.PersonID)
		}
	}

	var persons map[int64]*ports.Person
	if s.Person != nil && len(personIDs) > 0 {
		var err error
		persons, err = s.Person.FindByIDs(ctx, personIDs)
		if err != nil {
			return nil, fmt.Errorf("load persons for student names: %w", err)
		}
	}

	for _, id := range ids {
		st := readable[id]
		if st == nil {
			continue
		}
		info := studentNameInfo{class: st.SchoolClass}
		if persons != nil {
			if p := persons[st.PersonID]; p != nil {
				info.name = p.Name
			}
		}
		result[id] = info
	}
	return result, nil
}

// pickupReadPredicate returns a per-student read-access test mirroring
// authorize.CanReadStudent for the pickup read path. Every reminder recipient
// is staff, so since #2329 all present students are readable — per-group
// gating is gone.
func (s *service) pickupReadPredicate(_ context.Context, _ reminder.Scope) (func(*ports.Student) bool, error) {
	return func(st *ports.Student) bool { return st != nil }, nil
}
