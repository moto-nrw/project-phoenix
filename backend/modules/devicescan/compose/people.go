package compose

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/application"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"

	"github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
)

// PersonDirectory is the People Directory slice the kiosk flows use: card
// lookup and assignment, the kiosk roster and the records behind a scan,
// which the root maps from the retained people service.
type PersonDirectory interface {
	devicescan.PersonLookup
	devicescan.TagCommands
	devicescan.StaffLookup
	devicescan.StudentLookup
}

// people resolves cards through the retained people service. Its not-found
// outcomes become the port sentinels; every other failure passes through
// unchanged so the flows keep their own classification.
type people struct{ PersonDirectory }

func (p people) FindPersonByTag(ctx context.Context, tag string) (*ports.Person, error) {
	return application.FindPersonByTagBinding(ctx, p.PersonDirectory, tag)
}

func (p people) FindStudentByPerson(ctx context.Context, personID int64) (*ports.Student, error) {
	return application.FindStudentByPersonBinding(ctx, p.PersonDirectory, sql.ErrNoRows, personID)
}

func (p people) FindStaffByPerson(ctx context.Context, personID int64) (*ports.StaffMember, error) {
	return application.FindStaffByPersonBinding(ctx, p.PersonDirectory, sql.ErrNoRows, personID)
}
