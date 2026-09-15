package peopledirectory

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
)

// EnrollmentStudent contains only the student facts an enrollment decision
// creates or renews. It cannot overwrite health, consent or departure state.
// Dates are calendar days in YYYY-MM-DD format.
type EnrollmentStudent = enrollment.Input

type CreatedEnrollmentStudent = enrollment.CreatedStudent
type EnrollmentProfilePatch = enrollment.ProfilePatch
type EnrollmentRecord = enrollment.Record

// ReadEnrollmentStudent optionally locks the row using "" (read), "update",
// or "nowait". Locking takes the shared class-write gate before the row lock.
func (m *Module) ReadEnrollmentStudent(ctx context.Context, id int64, lock string) (EnrollmentRecord, error) {
	if id <= 0 || (lock != "" && lock != "update" && lock != "nowait") {
		return EnrollmentRecord{}, &InvalidStudentError{Reason: "invalid enrollment student read"}
	}
	return m.engine.ReadEnrollmentStudent(ctx, id, lock)
}

func (m *Module) LockEnrollmentClassWrites(ctx context.Context) error {
	return m.engine.LockEnrollmentClassWrites(ctx)
}

func (m *Module) ApplyEnrollmentProfile(ctx context.Context, id int64, input enrollment.ProfilePatch) error {
	if id <= 0 {
		return &InvalidStudentError{Reason: "student ID is required"}
	}
	if err := normalizeEnrollmentDeparture(&input); err != nil {
		return err
	}
	return m.engine.ApplyEnrollmentProfile(ctx, id, input)
}

// CreateEnrollmentStudent inserts a student for an existing person in the same
// tenant, under the shared class-write gate and the caller's transaction.
func (m *Module) CreateEnrollmentStudent(ctx context.Context, input EnrollmentStudent) (CreatedEnrollmentStudent, error) {
	if input.PersonID <= 0 {
		return CreatedEnrollmentStudent{}, &InvalidStudentError{Reason: "person ID is required"}
	}
	if err := normalizeEnrollmentStudent(&input); err != nil {
		return CreatedEnrollmentStudent{}, err
	}
	return m.engine.CreateEnrollmentStudent(ctx, input)
}

// RenewEnrollmentStudent updates only class, lifecycle dates/status and contact
// columns. Callers retain the old contact values for rollover without a form.
func (m *Module) RenewEnrollmentStudent(ctx context.Context, id int64, input EnrollmentStudent) error {
	if id <= 0 {
		return &InvalidStudentError{Reason: "student ID is required"}
	}
	if err := normalizeEnrollmentStudent(&input); err != nil {
		return err
	}
	return m.engine.RenewEnrollmentStudent(ctx, id, input)
}

func normalizeEnrollmentStudent(input *EnrollmentStudent) error {
	if input.SchoolClass == "" {
		return &InvalidStudentError{Reason: "school class is required"}
	}
	input.SchoolClass = strings.TrimSpace(input.SchoolClass)
	if err := validateBirthday(input.EnrolledFrom); err != nil {
		return &InvalidStudentError{Reason: "invalid enrolled_from calendar date"}
	}
	if err := validateBirthday(input.EnrolledUntil); err != nil {
		return &InvalidStudentError{Reason: "invalid enrolled_until calendar date"}
	}
	for _, value := range []struct {
		ptr   **string
		valid func(string) bool
		label string
	}{
		{&input.GuardianEmail, contact.IsValidEmailFormat, "guardian email"},
		{&input.GuardianPhone, contact.IsValidPhoneFormat, "guardian phone"},
	} {
		if *value.ptr == nil || **value.ptr == "" {
			continue
		}
		normalized := strings.TrimSpace(**value.ptr)
		if !value.valid(normalized) {
			return &InvalidStudentError{Reason: "invalid " + value.label + " format"}
		}
		*value.ptr = &normalized
	}
	return nil
}
