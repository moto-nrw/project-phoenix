package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

type TagAssignments = devicescan.TagAssignments

// NewTagAssignments composes bracelet lookup and staff assignment without the scan recorder.
func NewTagAssignments(users usersSvc.PersonService, logger *slog.Logger) devicescan.TagAssignments {
	if users == nil {
		panic("tag assignment composition: users are required")
	}
	return application.NewTagAssignments(tagPeople{people{users: users}}, principals{}, clock{now: time.Now}, logger)
}

func (p tagPeople) FindStaff(ctx context.Context, staffID int64) (*ports.StaffMember, error) {
	staff, err := p.users.GetStaffByID(ctx, staffID)
	if err != nil || staff == nil {
		return nil, err
	}
	return &ports.StaffMember{ID: staff.ID, PersonID: staff.PersonID}, nil
}

func (p tagPeople) FindPerson(ctx context.Context, personID int64) (*ports.Person, error) {
	person, err := p.users.Get(ctx, personID)
	if err != nil || person == nil {
		return nil, err
	}
	return &ports.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, HasTag: person.TagID != nil, TagID: person.TagID}, nil
}

func (p tagPeople) LinkTag(ctx context.Context, personID int64, tag string) error {
	return p.users.LinkToRFIDCard(ctx, personID, tag)
}

func (p tagPeople) UnlinkTag(ctx context.Context, personID int64) error {
	return p.users.UnlinkFromRFIDCard(ctx, personID)
}

type tagPeople struct{ people }

func (p tagPeople) TeacherRole(ctx context.Context, staffID int64) (string, string, bool, error) {
	teacher, err := p.users.GetTeacherByStaffID(ctx, staffID)
	if err != nil || teacher == nil {
		return "", "", false, err
	}
	return teacher.Role, teacher.Specialization, true, nil
}
