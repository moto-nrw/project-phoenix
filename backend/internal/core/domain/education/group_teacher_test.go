package education

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

func TestGroupTeacher_Validate(t *testing.T) {
	tests := []struct {
		name    string
		gt      *GroupTeacher
		wantErr bool
	}{
		{
			name: "valid group teacher",
			gt: &GroupTeacher{
				GroupID:   1,
				TeacherID: 1,
			},
			wantErr: false,
		},
		{
			name: "zero group ID",
			gt: &GroupTeacher{
				GroupID:   0,
				TeacherID: 1,
			},
			wantErr: true,
		},
		{
			name: "negative group ID",
			gt: &GroupTeacher{
				GroupID:   -1,
				TeacherID: 1,
			},
			wantErr: true,
		},
		{
			name: "zero teacher ID",
			gt: &GroupTeacher{
				GroupID:   1,
				TeacherID: 0,
			},
			wantErr: true,
		},
		{
			name: "negative teacher ID",
			gt: &GroupTeacher{
				GroupID:   1,
				TeacherID: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.gt.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GroupTeacher.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupTeacher_SetGroup(t *testing.T) {
	t.Run("set group", func(t *testing.T) {
		gt := &GroupTeacher{TeacherID: 1}
		group := &Group{
			Model: base.Model{ID: 42},
			Name:  "Test Group",
		}

		gt.SetGroup(group)

		if gt.Group != group {
			t.Error("GroupTeacher.SetGroup() did not set Group reference")
		}

		if gt.GroupID != 42 {
			t.Errorf("GroupTeacher.GroupID = %v, want 42", gt.GroupID)
		}
	})

	t.Run("set nil group", func(t *testing.T) {
		gt := &GroupTeacher{
			GroupID:   42,
			TeacherID: 1,
		}

		gt.SetGroup(nil)

		if gt.Group != nil {
			t.Error("GroupTeacher.SetGroup(nil) did not clear Group reference")
		}

		// GroupID is not cleared when setting nil - this matches the implementation
	})
}

func TestGroupTeacher_SetTeacher(t *testing.T) {
	t.Run("set teacher", func(t *testing.T) {
		gt := &GroupTeacher{GroupID: 1}
		teacher := &users.Teacher{
			Model:   base.Model{ID: 42},
			StaffID: 1,
		}

		gt.SetTeacher(teacher)

		if gt.Teacher != teacher {
			t.Error("GroupTeacher.SetTeacher() did not set Teacher reference")
		}

		if gt.TeacherID != 42 {
			t.Errorf("GroupTeacher.TeacherID = %v, want 42", gt.TeacherID)
		}
	})

	t.Run("set nil teacher", func(t *testing.T) {
		gt := &GroupTeacher{
			GroupID:   1,
			TeacherID: 42,
		}

		gt.SetTeacher(nil)

		if gt.Teacher != nil {
			t.Error("GroupTeacher.SetTeacher(nil) did not clear Teacher reference")
		}

		// TeacherID is not cleared when setting nil - this matches the implementation
	})
}

func TestGroupTeacher_BeforeAppendModel(t *testing.T) {
	gt := &GroupTeacher{GroupID: 1, TeacherID: 1}
	err := gt.BeforeAppendModel(nil)
	if err != nil {
		t.Errorf("GroupTeacher.BeforeAppendModel() error = %v", err)
	}
}
