package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// BirthdayCapability is the People Directory birthday display the factory
// hands to its consumers.
type BirthdayCapability = peopleModule.Birthdays

// BirthdayRepositories are the retained repositories the birthday display
// reads and the staff opt-out writes through (#1542).
type BirthdayRepositories struct {
	Students userModels.StudentRepository
	Staff    userModels.StaffRepository
	Persons  userModels.PersonRepository
}

// NewBirthdays binds the People Directory birthday capability over the
// retained repositories and the tenant settings.
func NewBirthdays(repos BirthdayRepositories, settings BirthdaySettingsSource, logger *slog.Logger, now func() time.Time) peopleModule.Birthdays {
	return peopleCompose.NewBirthdays(peopleCompose.BirthdayDependencies{
		Store:    birthdayStore{repos: repos},
		Settings: birthdaySettings{settings: settings},
		Logger:   logger,
		Now:      now,
	})
}

type birthdayStore struct{ repos BirthdayRepositories }

func (s birthdayStore) StudentBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error) {
	rows, err := s.repos.Students.FindBirthdaysOn(ctx, monthDays(days))
	return birthdayEntries(rows, peopleCompose.BirthdayKindStudent), err
}

func (s birthdayStore) StaffBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error) {
	rows, err := s.repos.Staff.FindBirthdaysOn(ctx, monthDays(days))
	return birthdayEntries(rows, peopleCompose.BirthdayKindStaff), err
}

func (s birthdayStore) StaffBirthdaysForExport(ctx context.Context) ([]peopleCompose.BirthdayEntry, error) {
	rows, err := s.repos.Staff.ListBirthdaysForExport(ctx)
	return birthdayEntries(rows, peopleCompose.BirthdayKindStaff), err
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

func (s birthdayStore) SetStaffBirthdayOptOut(ctx context.Context, staffID int64, optOut bool) error {
	return s.repos.Staff.SetBirthdayDisplayOptOut(ctx, staffID, optOut)
}

func monthDays(days []peopleCompose.BirthdayMonthDay) []userModels.MonthDay {
	result := make([]userModels.MonthDay, 0, len(days))
	for _, day := range days {
		result = append(result, userModels.MonthDay{Month: day.Month, Day: day.Day})
	}
	return result
}

func birthdayEntries(rows []userModels.BirthdayEntry, kind peopleCompose.BirthdayKind) []peopleCompose.BirthdayEntry {
	entries := make([]peopleCompose.BirthdayEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, peopleCompose.BirthdayEntry{
			Kind: kind, ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, Birthday: row.Birthday,
			GroupName: row.GroupName, SchoolClass: row.SchoolClass,
		})
	}
	return entries
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
// marker method, like api/common.IsNotFound: the retained lookups report a
// missing row either as this error or as a nil result.
func isRepositoryNotFound(err error) bool {
	var notFound interface{ RepositoryNotFound() }
	return errors.As(err, &notFound)
}
