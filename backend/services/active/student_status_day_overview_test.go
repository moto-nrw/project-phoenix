package active

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

func TestStudentEnrolledOn(t *testing.T) {
	date := timezone.NewDate(2026, 8, 20)
	before := date.AddDays(-1)
	after := date.AddDays(1)

	tests := []struct {
		name    string
		student *userModels.Student
		want    bool
	}{
		{name: "inside enrollment interval", student: &userModels.Student{EnrolledFrom: &before, EnrolledUntil: &after}, want: true},
		{name: "before enrollment", student: &userModels.Student{EnrolledFrom: &after, Status: userModels.StudentStatusPending}},
		{name: "immediately active before enrollment", student: &userModels.Student{EnrolledFrom: &after, Status: userModels.StudentStatusActive}, want: true},
		{name: "after enrollment", student: &userModels.Student{EnrolledUntil: &before, Status: userModels.StudentStatusActive}},
		{name: "inactive legacy student", student: &userModels.Student{Status: userModels.StudentStatusInactive}},
		{name: "active legacy student", student: &userModels.Student{Status: userModels.StudentStatusActive}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, studentEnrolledOn(test.student, date, date))
		})
	}
}
