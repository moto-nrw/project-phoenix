package application

import (
	"net/mail"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The owner edge: every input is validated and normalised here, before the
// operation is observed or reaches the store.

func invalid(reason string) error { return &schoolmembership.InvalidMembershipError{Reason: reason} }

func validateStaffFields(fields *schoolmembership.StaffFields) error {
	if fields.PersonID <= 0 {
		return invalid("person ID is required")
	}
	if err := validateEmploymentType(fields.EmploymentType); err != nil {
		return err
	}
	if fields.WorkTimeModelID != nil && *fields.WorkTimeModelID <= 0 {
		return invalid("work time model ID must be positive")
	}
	fields.PersonnelNumber = normalizePersonnelNumber(fields.PersonnelNumber)
	return validateDate(fields.RotationAnchorDate, "rotation anchor date")
}

func validateTeacherFields(fields *schoolmembership.TeacherFields) error {
	if fields.StaffID <= 0 {
		return invalid("staff ID is required")
	}
	fields.Specialization = strings.TrimSpace(fields.Specialization)
	fields.Role = strings.TrimSpace(fields.Role)
	return nil
}

func validateGuestFields(fields *schoolmembership.GuestFields) error {
	if fields.StaffID <= 0 {
		return invalid("staff ID is required")
	}
	fields.ActivityExpertise = strings.TrimSpace(fields.ActivityExpertise)
	if fields.ActivityExpertise == "" {
		return invalid("activity expertise is required")
	}
	fields.Organization = strings.TrimSpace(fields.Organization)
	fields.ContactEmail = strings.TrimSpace(fields.ContactEmail)
	if fields.ContactEmail != "" {
		if _, err := mail.ParseAddress(fields.ContactEmail); err != nil {
			return invalid("invalid contact email format")
		}
	}
	fields.ContactPhone = strings.TrimSpace(fields.ContactPhone)
	if fields.ContactPhone != "" && !validPhone(fields.ContactPhone) {
		return invalid("invalid contact phone format")
	}
	if err := validateDate(fields.StartDate, "start date"); err != nil {
		return err
	}
	if err := validateDate(fields.EndDate, "end date"); err != nil {
		return err
	}
	if fields.StartDate != "" && fields.EndDate != "" && fields.EndDate < fields.StartDate {
		return invalid("end date cannot be before start date")
	}
	return nil
}

// validPhone mirrors the shared phone rule of the legacy guest model: an
// optional leading plus, then digits with spaces, slashes, dashes or
// parentheses, at least six digits in total.
func validPhone(value string) bool {
	digits := 0
	for index, r := range value {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == '+' && index == 0:
		case r == ' ' || r == '/' || r == '-' || r == '(' || r == ')':
		default:
			return false
		}
	}
	return digits >= 6 && digits <= 20
}

func validateEmploymentType(value *string) error {
	if value == nil {
		return nil
	}
	switch *value {
	case schoolmembership.EmploymentTypeFullTime, schoolmembership.EmploymentTypePartTime, schoolmembership.EmploymentTypeMinijob:
		return nil
	default:
		return invalid("employment_type must be 'full_time', 'part_time', or 'minijob'")
	}
}

// normalizePersonnelNumber trims the number; a blank one is no number.
func normalizePersonnelNumber(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validateClassListEntryFields(fields *schoolmembership.ClassListEntryFields) error {
	fields.FirstName = strings.TrimSpace(fields.FirstName)
	fields.LastName = strings.TrimSpace(fields.LastName)
	fields.SchoolClass = strings.TrimSpace(fields.SchoolClass)
	switch {
	case fields.FirstName == "":
		return invalid("first name is required")
	case fields.LastName == "":
		return invalid("last name is required")
	case fields.SchoolClass == "":
		return invalid("school class is required")
	}
	return nil
}

func normalizeClassListEntryFilter(filter schoolmembership.ClassListEntryFilter) schoolmembership.ClassListEntryFilter {
	filter.IDs = uniquePositive(filter.IDs)
	filter.FirstName = strings.TrimSpace(filter.FirstName)
	filter.LastName = strings.TrimSpace(filter.LastName)
	filter.SchoolClass = strings.TrimSpace(filter.SchoolClass)
	return filter
}

func validateClassAssignment(staffID int64, schoolClass *string) error {
	*schoolClass = strings.TrimSpace(*schoolClass)
	if staffID <= 0 {
		return invalid("staff ID is required")
	}
	if *schoolClass == "" {
		return invalid("school class is required")
	}
	return nil
}

func validateGroupAssignment(groupID, teacherID int64) error {
	if groupID <= 0 {
		return invalid("group ID is required")
	}
	if teacherID <= 0 {
		return invalid("teacher ID is required")
	}
	return nil
}

func validateStudentEnrollment(input schoolmembership.StudentEnrollment, allowUnchangedAlumnus bool) error {
	if input.StudentID <= 0 {
		return invalid("student ID is required")
	}
	if input.GroupID != nil && *input.GroupID <= 0 {
		return invalid("group ID must be positive")
	}
	if input.SchoolClass == "" {
		return invalid("school class is required")
	}
	if !lifecycleStatus(input.Status) && (!allowUnchangedAlumnus || input.Status != "alumnus") {
		return invalid("enrollment requires a non-alumni lifecycle status")
	}
	if err := validateDate(input.EnrolledFrom, "enrollment start"); err != nil {
		return err
	}
	return validateDate(input.EnrolledUntil, "enrollment end")
}

// lifecycleStatus reports a non-alumni lifecycle status.
func lifecycleStatus(status string) bool {
	return status == "active" || status == "pending" || status == "inactive"
}

func validChildQuotaCheck(check schoolmembership.ChildQuotaCheck) bool {
	return check == schoolmembership.EnforceChildQuota || check == schoolmembership.SkipChildQuotaForGradeTransitionRevert
}

func validateDate(value, label string) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(schoolmembership.DateLayout, value); err != nil {
		return invalid(label + " must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func uniquePositive(ids []int64) []int64 {
	if ids == nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
