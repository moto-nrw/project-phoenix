package compose

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

type FeedbackStudents = devicescan.FeedbackStudents

type feedbackStudents struct{ users PersonDirectory }

func NewFeedbackStudents(users PersonDirectory) devicescan.FeedbackStudents {
	if users == nil {
		panic("feedback student composition: users are required")
	}
	return feedbackStudents{users: users}
}

func (r feedbackStudents) LockFeedbackStudent(ctx context.Context, studentID int64) (devicescan.FeedbackStudent, error) {
	student, err := r.users.GetStudentByIDForUpdate(ctx, studentID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && student == nil) {
		return devicescan.FeedbackStudent{}, devicescan.ErrFeedbackStudentNotFound
	}
	if err != nil {
		return devicescan.FeedbackStudent{}, err
	}
	return devicescan.FeedbackStudent{ID: student.ID, Alumnus: student.IsAlumnus()}, nil
}
