package activities

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Define attendance status constants
const (
	AttendancePresent = "PRESENT"
	AttendanceAbsent  = "ABSENT"
	AttendanceExcused = "EXCUSED"
	AttendanceUnknown = "UNKNOWN"
)

// Table name constants for BUN ORM schema qualification
const (
	tableActivitiesStudentEnrollments     = "activities.student_enrollments"
	tableExprStudentEnrollmentsAsEnrollment = `activities.student_enrollments AS "student_enrollment"`
)

// StudentEnrollment represents a student enrolled in an activity group
type StudentEnrollment struct {
	base.Model       `bun:"schema:activities,table:student_enrollments"`
	StudentID        int64     `bun:"student_id,notnull" json:"student_id"`
	ActivityGroupID  int64     `bun:"activity_group_id,notnull" json:"activity_group_id"`
	EnrollmentDate   time.Time `bun:"enrollment_date,notnull" json:"enrollment_date"`
	AttendanceStatus *string   `bun:"attendance_status" json:"attendance_status,omitempty"`

	// Relations - populated when using the ORM's relations
	Student       *users.Student `bun:"rel:belongs-to,join:student_id=id" json:"student,omitempty"`
	ActivityGroup *Group         `bun:"rel:belongs-to,join:activity_group_id=id" json:"activity_group,omitempty"`
}

func (se *StudentEnrollment) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(tableExprStudentEnrollmentsAsEnrollment)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActivitiesStudentEnrollments)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActivitiesStudentEnrollments)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActivitiesStudentEnrollments)
	}
	return nil
}

// GetID returns the entity's ID
func (se *StudentEnrollment) GetID() interface{} {
	return se.ID
}

// GetCreatedAt returns the creation timestamp
func (se *StudentEnrollment) GetCreatedAt() time.Time {
	return se.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (se *StudentEnrollment) GetUpdatedAt() time.Time {
	return se.UpdatedAt
}

// TableName returns the database table name
func (se *StudentEnrollment) TableName() string {
	return tableActivitiesStudentEnrollments
}

// IsValidAttendanceStatus checks if the attendance status is valid
func IsValidAttendanceStatus(status string) bool {
	validStatuses := map[string]bool{
		AttendancePresent: true,
		AttendanceAbsent:  true,
		AttendanceExcused: true,
		AttendanceUnknown: true,
	}
	return validStatuses[status]
}

// Validate ensures student enrollment data is valid
func (se *StudentEnrollment) Validate() error {
	if se.StudentID <= 0 {
		return errors.New("student ID is required")
	}

	if se.ActivityGroupID <= 0 {
		return errors.New("activity group ID is required")
	}

	if se.EnrollmentDate.IsZero() {
		se.EnrollmentDate = time.Now()
	}

	if se.AttendanceStatus != nil && !IsValidAttendanceStatus(*se.AttendanceStatus) {
		return errors.New("invalid attendance status")
	}

	return nil
}

// MarkPresent marks the student as present
func (se *StudentEnrollment) MarkPresent() {
	status := AttendancePresent
	se.AttendanceStatus = &status
}

// MarkAbsent marks the student as absent
func (se *StudentEnrollment) MarkAbsent() {
	status := AttendanceAbsent
	se.AttendanceStatus = &status
}

// MarkExcused marks the student as excused
func (se *StudentEnrollment) MarkExcused() {
	status := AttendanceExcused
	se.AttendanceStatus = &status
}

// ClearAttendance clears the attendance status
func (se *StudentEnrollment) ClearAttendance() {
	se.AttendanceStatus = nil
}
