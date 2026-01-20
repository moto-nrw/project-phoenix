package students

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// StudentResponseOpts groups parameters for creating a student response to reduce function parameter count
type StudentResponseOpts struct {
	Student          *users.Student
	Person           *users.Person
	Group            *education.Group
	HasFullAccess    bool
	LocationOverride *string
}

// StudentResponseServices groups service dependencies for student response creation
type StudentResponseServices struct {
	ActiveService activeService.Service
	PersonService userService.PersonService
}

// populatePersonAndGuardianData fills the response with person and guardian information
// based on access level permissions
func populatePersonAndGuardianData(response *StudentResponse, person *users.Person, student *users.Student, group *education.Group, hasFullAccess bool) {
	if person != nil {
		response.FirstName = person.FirstName
		response.LastName = person.LastName
		// Format birthday as YYYY-MM-DD string if available
		if person.Birthday != nil {
			response.Birthday = person.Birthday.Format(dateFormatYYYYMMDD)
		}
		// Only include RFID tag for users with full access
		if hasFullAccess && person.TagID != nil {
			response.TagID = *person.TagID
		}
	}

	// Only include guardian email and phone for users with full access
	if hasFullAccess {
		if student.GuardianEmail != nil {
			response.GuardianEmail = *student.GuardianEmail
		}

		if student.GuardianPhone != nil {
			response.GuardianPhone = *student.GuardianPhone
		}
	}

	if student.GroupID != nil {
		response.GroupID = *student.GroupID
	}

	if group != nil {
		response.GroupName = group.Name
	}
}

// populatePublicStudentFields sets fields visible to all authenticated staff
func populatePublicStudentFields(response *StudentResponse, student *users.Student) {
	if student.HealthInfo != nil {
		response.HealthInfo = *student.HealthInfo
	}
	if student.Bus != nil {
		response.Bus = *student.Bus
	}
	if student.PickupStatus != nil {
		response.PickupStatus = *student.PickupStatus
	}
}

// populateSensitiveStudentFields sets fields visible only to supervisors/admins
func populateSensitiveStudentFields(response *StudentResponse, student *users.Student) {
	if student.ExtraInfo != nil && *student.ExtraInfo != "" {
		response.ExtraInfo = *student.ExtraInfo
	}
	if student.SupervisorNotes != nil {
		response.SupervisorNotes = *student.SupervisorNotes
	}
	if student.Sick != nil {
		response.Sick = *student.Sick
	}
	if student.SickSince != nil {
		response.SickSince = student.SickSince
	}
}

// populateSnapshotSensitiveFields sets sensitive fields for the snapshot version
// Note: This differs from populateSensitiveStudentFields by including HealthInfo
func populateSnapshotSensitiveFields(response *StudentResponse, student *users.Student) {
	if student.ExtraInfo != nil && *student.ExtraInfo != "" {
		response.ExtraInfo = *student.ExtraInfo
	}
	if student.HealthInfo != nil {
		response.HealthInfo = *student.HealthInfo
	}
	if student.SupervisorNotes != nil {
		response.SupervisorNotes = *student.SupervisorNotes
	}
	if student.Sick != nil {
		response.Sick = *student.Sick
	}
	if student.SickSince != nil {
		response.SickSince = student.SickSince
	}
}

// populateSnapshotPublicFields sets fields visible to all staff in snapshot version
func populateSnapshotPublicFields(response *StudentResponse, student *users.Student) {
	if student.Bus != nil {
		response.Bus = *student.Bus
	}
	if student.PickupStatus != nil {
		response.PickupStatus = *student.PickupStatus
	}
}

// presentOrTransit returns the appropriate location for a checked-in student
// without a specific room assignment, based on access level.
func presentOrTransit(hasFullAccess bool) common.StudentLocationInfo {
	if hasFullAccess {
		return common.StudentLocationInfo{Location: "Unterwegs"}
	}
	return common.StudentLocationInfo{Location: "Anwesend"}
}

// absentInfo returns the "Abwesend" location, optionally with checkout time for full access users.
func absentInfo(hasFullAccess bool, checkOutTime *time.Time) common.StudentLocationInfo {
	if hasFullAccess && checkOutTime != nil {
		return common.StudentLocationInfo{Location: "Abwesend", Since: checkOutTime}
	}
	return common.StudentLocationInfo{Location: "Abwesend"}
}

// resolveStudentLocationWithTime determines a student's current location with timestamp
func resolveStudentLocationWithTime(ctx context.Context, studentID int64, hasFullAccess bool, activeService activeService.Service) common.StudentLocationInfo {
	attendanceStatus, err := activeService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return common.StudentLocationInfo{Location: "Abwesend"}
	}

	// Handle non-checked-in states (checked_out or other)
	if attendanceStatus.Status != "checked_in" {
		return absentInfo(hasFullAccess, attendanceStatus.CheckOutTime)
	}

	// Student is checked in - get current visit to check room assignment
	currentVisit, err := activeService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil || currentVisit.ActiveGroupID <= 0 {
		return presentOrTransit(hasFullAccess)
	}

	activeGroup, err := activeService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return presentOrTransit(hasFullAccess)
	}

	// Include room name for all authenticated staff (needed for supervised room checkout)
	if activeGroup.Room != nil && activeGroup.Room.Name != "" {
		return common.StudentLocationInfo{
			Location: fmt.Sprintf("Anwesend - %s", activeGroup.Room.Name),
			Since:    &currentVisit.EntryTime,
		}
	}

	return presentOrTransit(hasFullAccess)
}

// newStudentResponseWithOpts creates a student response using options structs
func newStudentResponseWithOpts(ctx context.Context, opts StudentResponseOpts, services StudentResponseServices) StudentResponse {
	student := opts.Student
	person := opts.Person
	group := opts.Group
	hasFullAccess := opts.HasFullAccess
	locationOverride := opts.LocationOverride
	response := StudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		SchoolClass: student.SchoolClass,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	}

	// Include legacy guardian name if available
	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}

	// Only include guardian contact info for users with full access
	if hasFullAccess && student.GuardianContact != nil {
		response.GuardianContact = *student.GuardianContact
	}

	// Resolve location
	if locationOverride != nil {
		response.Location = *locationOverride
	} else {
		locationInfo := resolveStudentLocationWithTime(ctx, student.ID, hasFullAccess, services.ActiveService)
		response.Location = locationInfo.Location
		response.LocationSince = locationInfo.Since
	}

	populatePersonAndGuardianData(&response, person, student, group, hasFullAccess)
	populatePublicStudentFields(&response, student)

	if hasFullAccess {
		populateSensitiveStudentFields(&response, student)
	}

	return response
}

// newStudentResponseFromSnapshot creates a student response using pre-loaded snapshot data
// This eliminates N+1 queries by using cached person, group, and location data
func newStudentResponseFromSnapshot(_ context.Context, student *users.Student, person *users.Person, group *education.Group, hasFullAccess bool, snapshot *common.StudentDataSnapshot) StudentResponse {
	response := StudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		SchoolClass: student.SchoolClass,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	}

	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}

	if hasFullAccess && student.GuardianContact != nil {
		response.GuardianContact = *student.GuardianContact
	}

	locationInfo := snapshot.ResolveLocationWithTime(student.ID, hasFullAccess)
	response.Location = locationInfo.Location
	response.LocationSince = locationInfo.Since

	populatePersonAndGuardianData(&response, person, student, group, hasFullAccess)
	populateSnapshotPublicFields(&response, student)

	if hasFullAccess {
		populateSnapshotSensitiveFields(&response, student)
	}

	return response
}

// newPrivacyConsentResponse converts a privacy consent model to a response
func newPrivacyConsentResponse(consent *users.PrivacyConsent) PrivacyConsentResponse {
	return PrivacyConsentResponse{
		ID:                consent.ID,
		StudentID:         consent.StudentID,
		PolicyVersion:     consent.PolicyVersion,
		Accepted:          consent.Accepted,
		AcceptedAt:        consent.AcceptedAt,
		ExpiresAt:         consent.ExpiresAt,
		DurationDays:      consent.DurationDays,
		RenewalRequired:   consent.RenewalRequired,
		DataRetentionDays: consent.DataRetentionDays,
		Details:           consent.Details,
		CreatedAt:         consent.CreatedAt,
		UpdatedAt:         consent.UpdatedAt,
	}
}

// teacherToSupervisorContact converts a teacher to a supervisor contact if valid
func teacherToSupervisorContact(teacher *users.Teacher) *SupervisorContact {
	if teacher == nil || teacher.Staff == nil || teacher.Staff.Person == nil {
		return nil
	}

	supervisor := &SupervisorContact{
		ID:        teacher.ID,
		FirstName: teacher.Staff.Person.FirstName,
		LastName:  teacher.Staff.Person.LastName,
		Role:      "teacher",
	}

	if teacher.Staff.Person.Account != nil {
		supervisor.Email = teacher.Staff.Person.Account.Email
	}

	return supervisor
}
