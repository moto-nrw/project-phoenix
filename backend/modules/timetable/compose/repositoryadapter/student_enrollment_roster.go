package repositoryadapter

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// FindByGroupID finds all enrollments for a specific group. The Student row
// on each enrollment is projected from the People Directory (#2662): id,
// timestamps, person, lifecycle status, class, group and care window. An
// enrollment whose student the directory does not return keeps a nil
// Student, as the former LEFT JOIN did. Student.Person is attached by the
// composition layer.
func (r *StudentEnrollmentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activities.StudentEnrollment, error) {
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	enrollments := make([]*activities.StudentEnrollment, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&enrollments).
		ModelTableExpr(tableExprActivitiesEnrollmentsAsEnrollment).
		Column("id", "created_at", "updated_at", "tenant_id", "student_id", "activity_group_id",
			"valid_from", "valid_until", "calendar_period_id", "enrollment_request_child_id",
			"selected_weekdays", "attendance_status", "weekday").
		Where(`"student_enrollment".activity_group_id = ?`, groupID)

	query = base.WithTenantFilter(ctx, query, "student_enrollment")

	if err := query.Order("student_enrollment.valid_from DESC").Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by group ID",
			Err: base.TranslateNotFound(err),
		}
	}
	if len(enrollments) == 0 {
		return enrollments, nil
	}

	ids := make([]int64, 0, len(enrollments))
	for _, enrollment := range enrollments {
		ids = append(ids, enrollment.StudentID)
	}
	students, err := r.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]DirectoryStudent, len(students))
	for _, student := range students {
		byID[student.ID] = student
	}
	for _, enrollment := range enrollments {
		if student, found := byID[enrollment.StudentID]; found {
			enrollment.Student = toRosterStudent(student)
		}
	}
	return enrollments, nil
}

func toRosterStudent(value DirectoryStudent) *users.Student {
	student := &users.Student{
		PersonID:      value.PersonID,
		SchoolClass:   value.SchoolClass,
		GroupID:       value.GroupID,
		Status:        users.StudentStatus(value.Status),
		EnrolledFrom:  parseDirectoryDate(value.EnrolledFrom),
		EnrolledUntil: parseDirectoryDate(value.EnrolledUntil),
	}
	student.ID = value.ID
	student.CreatedAt = value.CreatedAt
	student.UpdatedAt = value.UpdatedAt
	return student
}
