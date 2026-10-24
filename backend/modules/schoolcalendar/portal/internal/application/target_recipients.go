package application

import (
	"fmt"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
)

// targetRecipients groups the already-loaded audience by staff member or
// guardian, retaining the portal-visible children behind each guardian invite.
type targetRecipients struct {
	staffIDs         map[int64]struct{}
	guardianStudents map[int64]map[int64]struct{}
	// allowEmptyParents lets a bulk parent target without reachable guardians
	// add nobody instead of rejecting the appointment.
	allowEmptyParents bool
}

func (r *targetRecipients) addTarget(readSet *targetResolutionReadSet, target AppointmentTarget) error {
	switch target.Type {
	case appointmentcap.TargetTypeAllStaff:
		// Only invite staff with an active account and calendar:own; unreachable
		// staff would leave RSVP appointments permanently pending.
		r.addStaff(readSet.reachableStaff)
	case appointmentcap.TargetTypeAllTeachers:
		// The Lehrkräfte reach the calendar in moto schule, not through
		// calendar:own; a school without them simply adds nobody, like a
		// school without staff for all_staff.
		r.addStaff(readSet.teachers)
	case appointmentcap.TargetTypeStaff:
		if !readSet.reachableStaff[*target.ID] {
			return fmt.Errorf("%w: staff target is not available", ErrInvalidRequest)
		}
		r.staffIDs[*target.ID] = struct{}{}
	case appointmentcap.TargetTypeGuardianProfile:
		return r.addGuardianTarget(readSet, *target.ID)
	case appointmentcap.TargetTypeAllSchoolParents:
		// Only active students' portal-active guardians receive school-wide
		// appointments. All reads have already been batched in the read set.
		return r.requireGuardians(r.addStudentsGuardians(readSet, activeStudentIDs(readSet.allSchoolStudents)), "no reachable guardians at this school")
	case appointmentcap.TargetTypeParentsByStudent:
		return r.requireGuardians(r.addStudentGuardians(readSet, *target.ID), "parent target has no reachable guardians")
	case appointmentcap.TargetTypeParentsByGroup:
		// A former student still assigned to the group must not receive its invite.
		studentIDs := activeStudentIDs(readSet.studentsByGroup[*target.ID])
		return r.requireGuardians(r.addStudentsGuardians(readSet, studentIDs), "parent target has no reachable guardians")
	case appointmentcap.TargetTypeParentsByClass:
		studentIDs := activeStudentIDs(readSet.studentsByClass[normalizeCalendarClass(*target.Value)])
		return r.requireGuardians(r.addStudentsGuardians(readSet, studentIDs), "parent target has no reachable guardians")
	}
	return nil
}

func (r *targetRecipients) addStaff(staffIDs map[int64]bool) {
	for staffID := range staffIDs {
		r.staffIDs[staffID] = struct{}{}
	}
}

// requireGuardians rejects a parent target that reached nobody, unless the
// caller allows empty parent targets (Schultermine, #4798).
func (r *targetRecipients) requireGuardians(added int, reason string) error {
	if added > 0 || r.allowEmptyParents {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrInvalidRequest, reason)
}

func (r *targetRecipients) addGuardianTarget(readSet *targetResolutionReadSet, guardianProfileID int64) error {
	if !readSet.activeGuardians[guardianProfileID] {
		return fmt.Errorf("%w: guardian target is not available", ErrInvalidRequest)
	}
	visible := false
	for _, link := range readSet.linksByGuardian[guardianProfileID] {
		if link.PortalAccess {
			visible = true
			studentID := link.StudentID
			r.addGuardian(readSet, guardianProfileID, &studentID)
		}
	}
	if !visible {
		return fmt.Errorf("%w: guardian target is not portal-visible", ErrInvalidRequest)
	}
	return nil
}

func (r *targetRecipients) addGuardian(readSet *targetResolutionReadSet, guardianProfileID int64, studentID *int64) bool {
	if guardianProfileID <= 0 || !readSet.activeGuardians[guardianProfileID] {
		return false
	}
	if _, ok := r.guardianStudents[guardianProfileID]; !ok {
		r.guardianStudents[guardianProfileID] = map[int64]struct{}{}
	}
	if studentID != nil && *studentID > 0 {
		r.guardianStudents[guardianProfileID][*studentID] = struct{}{}
	}
	return true
}

func (r *targetRecipients) addStudentGuardians(readSet *targetResolutionReadSet, studentID int64) int {
	added := 0
	for _, link := range readSet.linksByStudent[studentID] {
		if link.PortalAccess && r.addGuardian(readSet, link.GuardianProfileID, &studentID) {
			added++
		}
	}
	return added
}

func (r *targetRecipients) addStudentsGuardians(readSet *targetResolutionReadSet, studentIDs []int64) int {
	added := 0
	for _, studentID := range studentIDs {
		added += r.addStudentGuardians(readSet, studentID)
	}
	return added
}

func (r *targetRecipients) fields(status string) []appointmentcap.AppointmentRecipientFields {
	recipients := make([]appointmentcap.AppointmentRecipientFields, 0, len(r.staffIDs)+len(r.guardianStudents))
	for staffID := range r.staffIDs {
		id := staffID
		recipients = append(recipients, appointmentcap.AppointmentRecipientFields{
			RecipientType: appointmentcap.RecipientTypeStaff,
			StaffID:       &id,
			Status:        status,
		})
	}
	for guardianProfileID, studentIDs := range r.guardianStudents {
		id := guardianProfileID
		studentList := make([]int64, 0, len(studentIDs))
		for studentID := range studentIDs {
			studentList = append(studentList, studentID)
		}
		recipients = append(recipients, appointmentcap.AppointmentRecipientFields{
			RecipientType:     appointmentcap.RecipientTypeGuardianProfile,
			GuardianProfileID: &id,
			Status:            status,
			StudentIDs:        studentList,
		})
	}
	return recipients
}
