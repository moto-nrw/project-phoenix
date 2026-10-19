package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

type TagAssignments = devicescan.TagAssignments

// NewTagAssignments composes bracelet lookup and staff assignment without the scan recorder.
func NewTagAssignments(users PersonDirectory, logger *slog.Logger, principal devicescan.DevicePrincipal) devicescan.TagAssignments {
	if principal == nil {
		panic("device scan composition: principal is required")
	}
	if users == nil {
		panic("tag assignment composition: users are required")
	}
	return application.NewTagAssignments(tagPeople{people{users}}, principal, clock{now: time.Now}, logger)
}

func (p tagPeople) FindStaff(ctx context.Context, staffID int64) (*ports.StaffMember, error) {
	return application.FindStaffBinding(ctx, p.PersonDirectory, staffID)
}

func (p tagPeople) FindPerson(ctx context.Context, personID int64) (*ports.Person, error) {
	return application.FindPersonBinding(ctx, p.PersonDirectory, personID)
}

func (p tagPeople) LinkTag(ctx context.Context, personID int64, tag string) error {
	return p.LinkToRFIDCard(ctx, personID, tag)
}

func (p tagPeople) UnlinkTag(ctx context.Context, personID int64) error {
	return p.UnlinkFromRFIDCard(ctx, personID)
}

type tagPeople struct{ people }

func (p tagPeople) TeacherRole(ctx context.Context, staffID int64) (string, string, bool, error) {
	return application.TeacherRoleBinding(ctx, p.PersonDirectory, staffID)
}
