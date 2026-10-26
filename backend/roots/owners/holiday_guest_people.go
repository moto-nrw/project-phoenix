package owners

import (
	"context"
	"fmt"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// HolidayGuestDirectory is the People Directory surface the child of a
// holiday guest is written through: the person and student commands an
// enrollment approval uses, in the caller's tenant transaction.
type HolidayGuestDirectory interface {
	CreateChildPerson(context.Context, peopledirectory.ChildPerson) (peopledirectory.Person, error)
	FindPerson(context.Context, int64) (peopledirectory.Person, error)
	SetPersonDetails(context.Context, int64, peopledirectory.PersonDetails) error
	ReadEnrollmentStudent(context.Context, int64, string) (peopleEnrollment.Record, error)
	CreateEnrollmentStudent(context.Context, peopledirectory.EnrollmentStudent) (peopledirectory.CreatedEnrollmentStudent, error)
	RenewEnrollmentStudent(context.Context, int64, peopledirectory.EnrollmentStudent) error
	ApplyEnrollmentProfile(context.Context, int64, peopleEnrollment.ProfilePatch) error
}

// NewHolidayGuests binds the holiday guests (#4666, ADR 0120) to Enrollment,
// their children to the People Directory and every transfer and sheet read
// to the school's access log at the Audit Platform. A nil today uses the
// Berlin calendar day.
func NewHolidayGuests(people HolidayGuestDirectory, accessLog DataAccessAppender, today func() calendar.Date) enrollmentOwner.HolidayGuests {
	return enrollmentCompose.NewHolidayGuests(NewHolidayGuestPeople(people), NewHolidayGuestAccessLog(accessLog), today)
}

// schedulerHolidayGuests binds the Worker's hourly reconciliation to the
// holiday guests of the school whose tenant transaction the Worker opened.
func schedulerHolidayGuests(people HolidayGuestDirectory, accessLog DataAccessAppender) schedulerPorts.HolidayGuestReconciliation {
	guests := NewHolidayGuests(people, accessLog, nil)
	return func(ctx context.Context) error {
		return guests.ReconcileHolidayGuests(ctx, tenant.FromContext(ctx))
	}
}

// NewHolidayGuestPeople binds the child of a holiday guest (#4666, ADR 0120)
// to the People Directory: a person with names and date of birth and an
// active student of the class "Feriengast" whose care covers the booked
// week, in the group the school picked.
func NewHolidayGuestPeople(directory HolidayGuestDirectory) enrollmentOwner.HolidayGuestPeople {
	if directory == nil {
		return nil
	}
	return holidayGuestPeople{directory: directory}
}

type holidayGuestPeople struct {
	directory HolidayGuestDirectory
}

func (p holidayGuestPeople) CreateHolidayGuestChild(ctx context.Context, child enrollmentOwner.HolidayGuestChild) (int64, error) {
	person, err := p.directory.CreateChildPerson(ctx, peopledirectory.ChildPerson{
		FirstName: child.FirstName, LastName: child.LastName, Birthday: child.BirthDate.String(),
	})
	if err != nil {
		return 0, err
	}
	student := holidayGuestStudent(person.ID, child)
	student.InitialProfile = &peopleEnrollment.ProfilePatch{GroupIDSet: true, GroupID: child.GroupID}
	created, err := p.directory.CreateEnrollmentStudent(ctx, student)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (p holidayGuestPeople) ReadHolidayGuestChild(ctx context.Context, studentID int64) (enrollmentOwner.HolidayGuestChild, error) {
	record, err := p.directory.ReadEnrollmentStudent(ctx, studentID, "update")
	if err != nil {
		return enrollmentOwner.HolidayGuestChild{}, err
	}
	person, err := p.directory.FindPerson(ctx, record.PersonID)
	if err != nil {
		return enrollmentOwner.HolidayGuestChild{}, err
	}
	birthDate, err := optionalDate(person.Birthday)
	if err != nil {
		return enrollmentOwner.HolidayGuestChild{}, err
	}
	from, err := optionalDate(record.EnrolledFrom)
	if err != nil {
		return enrollmentOwner.HolidayGuestChild{}, err
	}
	until, err := optionalDate(record.EnrolledUntil)
	if err != nil {
		return enrollmentOwner.HolidayGuestChild{}, err
	}
	return enrollmentOwner.HolidayGuestChild{
		FirstName: person.FirstName, LastName: person.LastName, BirthDate: birthDate, GroupID: record.GroupID,
		EnrolledFrom: from, EnrolledUntil: until,
	}, nil
}

func (p holidayGuestPeople) UpdateHolidayGuestChild(ctx context.Context, studentID int64, child enrollmentOwner.HolidayGuestChild) error {
	record, err := p.directory.ReadEnrollmentStudent(ctx, studentID, "update")
	if err != nil {
		return err
	}
	if err := p.directory.SetPersonDetails(ctx, record.PersonID, peopledirectory.PersonDetails{
		FirstName: child.FirstName, LastName: child.LastName, Birthday: child.BirthDate.String(),
	}); err != nil {
		return err
	}
	if err := p.directory.RenewEnrollmentStudent(ctx, studentID, holidayGuestStudent(record.PersonID, child)); err != nil {
		return err
	}
	return p.directory.ApplyEnrollmentProfile(ctx, studentID, peopleEnrollment.ProfilePatch{GroupIDSet: true, GroupID: child.GroupID})
}

func holidayGuestStudent(personID int64, child enrollmentOwner.HolidayGuestChild) peopledirectory.EnrollmentStudent {
	return peopledirectory.EnrollmentStudent{
		PersonID: personID, SchoolClass: enrollmentOwner.HolidayGuestSchoolClass, Status: peopledirectory.StudentStatusActive,
		EnrolledFrom: child.EnrolledFrom.String(), EnrolledUntil: child.EnrolledUntil.String(),
	}
}

// optionalDate parses a People Directory calendar date; empty is unset.
func optionalDate(value string) (calendar.Date, error) {
	if value == "" {
		return "", nil
	}
	date, err := calendar.ParseDate(value)
	if err != nil {
		return "", fmt.Errorf("holiday guest child: %w", err)
	}
	return date, nil
}
