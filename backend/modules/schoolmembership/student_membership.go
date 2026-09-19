package schoolmembership

import "context"

type StudentEnrollment struct {
	StudentID     int64
	GroupID       *int64
	SchoolClass   string
	Status        string
	EnrolledFrom  string
	EnrolledUntil string
}

func validateStudentEnrollment(input StudentEnrollment, allowUnchangedAlumnus bool) error {
	if input.StudentID <= 0 {
		return invalid("student ID is required")
	}
	if input.GroupID != nil && *input.GroupID <= 0 {
		return invalid("group ID must be positive")
	}
	if input.SchoolClass == "" {
		return invalid("school class is required")
	}
	if input.Status != "active" && input.Status != "pending" && input.Status != "inactive" && (!allowUnchangedAlumnus || input.Status != "alumnus") {
		return invalid("enrollment requires a non-alumni lifecycle status")
	}
	if err := validateDate(input.EnrolledFrom, "enrollment start"); err != nil {
		return err
	}
	if err := validateDate(input.EnrolledUntil, "enrollment end"); err != nil {
		return err
	}
	return nil
}

func (m *Module) Enroll(ctx context.Context, input StudentEnrollment) (int64, error) {
	if input.Status == "" {
		input.Status = "active"
	}
	if err := validateStudentEnrollment(input, false); err != nil {
		return 0, err
	}
	return m.engine.EnrollStudent(ctx, input)
}

func (m *Module) RenewEnrollment(ctx context.Context, input StudentEnrollment) (int64, error) {
	if input.Status == "" {
		input.Status = "active"
	}
	if err := validateStudentEnrollment(input, true); err != nil {
		return 0, err
	}
	return m.engine.RenewStudentEnrollment(ctx, input)
}

func (m *Module) AssignGroup(ctx context.Context, studentID int64, groupID *int64) (bool, error) {
	if studentID <= 0 || (groupID != nil && *groupID <= 0) {
		return false, invalid("student and group IDs must be positive")
	}
	return m.engine.AssignStudentGroup(ctx, studentID, groupID)
}
