package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	birthdaysAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/inbound/birthdays"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// BirthdayRepositories are the retained rows the birthday display reads.
// The root supplies the staff opt-out workflow command separately (#1542).
type BirthdayRepositories struct {
	Students birthdayStudents
	Staff    birthdayStaff
	Persons  birthdayPersons
}

// The retained People Directory reads of the birthday display.
type (
	birthdayStudents interface {
		FindBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error)
	}
	birthdayStaff interface {
		FindBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error)
		ListBirthdaysForExport(ctx context.Context) ([]peopleCompose.BirthdayEntry, error)
		FindByPersonID(ctx context.Context, personID int64) (*peopleModule.Staff, error)
	}
	birthdayPersons interface {
		FindByAccountID(ctx context.Context, accountID int64) (*peopleModule.DirectoryPersonSnapshot, error)
	}
)

// NewBirthdays binds People Directory birthday reads and the staff opt-out
// workflow to the HTTP entry surface.
func NewBirthdays(repos BirthdayRepositories, writes staffoffboarding.StaffBirthdayWrites, settings BirthdaySettingsSource, logger *slog.Logger, now func() time.Time) birthdaysAPI.Service {
	reads := peopleCompose.NewBirthdays(peopleCompose.BirthdayDependencies{
		Store:    birthdayStore{repos: repos},
		Settings: birthdaySettings{settings: settings},
		Logger:   logger,
		Now:      now,
	})
	return staffoffboarding.NewBirthdayPreferences(reads, writes, logger)
}

type birthdayStore struct{ repos BirthdayRepositories }

func (s birthdayStore) StudentBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error) {
	rows, err := s.repos.Students.FindBirthdaysOn(ctx, days)
	return rows, err
}

func (s birthdayStore) StaffBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error) {
	rows, err := s.repos.Staff.FindBirthdaysOn(ctx, days)
	return rows, err
}

func (s birthdayStore) StaffBirthdaysForExport(ctx context.Context) ([]peopleCompose.BirthdayEntry, error) {
	rows, err := s.repos.Staff.ListBirthdaysForExport(ctx)
	return rows, err
}

func (s birthdayStore) FindPersonByAccount(ctx context.Context, accountID int64) (int64, bool, error) {
	person, err := s.repos.Persons.FindByAccountID(ctx, accountID)
	if err != nil {
		if isRepositoryNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if person == nil {
		return 0, false, nil
	}
	return person.ID, true, nil
}

func (s birthdayStore) FindStaffByPerson(ctx context.Context, personID int64) (peopleCompose.BirthdayStaff, bool, error) {
	staff, err := s.repos.Staff.FindByPersonID(ctx, personID)
	if err != nil {
		if isRepositoryNotFound(err) {
			return peopleCompose.BirthdayStaff{}, false, nil
		}
		return peopleCompose.BirthdayStaff{}, false, err
	}
	if staff == nil {
		return peopleCompose.BirthdayStaff{}, false, nil
	}
	return peopleCompose.BirthdayStaff{ID: staff.ID, OptOut: staff.BirthdayDisplayOptOut}, true, nil
}

// BirthdaySettingsSource is the slice of the tenant settings service the
// birthday display reads: two boolean settings.
type BirthdaySettingsSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type birthdaySettings struct{ settings BirthdaySettingsSource }

func (s birthdaySettings) BirthdayDisplayEnabled(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModel.KeyBirthdayDisplayEnabled)
}

func (s birthdaySettings) BirthdayDisplayIncludesStaff(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModel.KeyBirthdayDisplayIncludeStaff)
}

// isRepositoryNotFound recognises the repositories' not-found sentinel by its
// marker method, like api/common.IsNotFound (models/base is not an import the
// root composition may add): the retained lookups report a
// missing row either as this error or as a nil result.
func isRepositoryNotFound(err error) bool {
	var notFound interface{ RepositoryNotFound() }
	return errors.As(err, &notFound)
}

func buildBirthdayEntry(db transactionruntime.Database, membership schoolmembership.Capability, repos BirthdayRepositories, settings BirthdaySettingsSource, logger *slog.Logger, now func() time.Time) (birthdaysAPI.Service, error) {
	writes, err := owners.NewStaffBirthdayWrites(db, membership)
	if err != nil {
		return nil, err
	}
	return NewBirthdays(repos, writes, settings, logger, now), nil
}
