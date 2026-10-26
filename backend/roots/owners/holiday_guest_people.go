package owners

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
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
func NewHolidayGuests(people HolidayGuestDirectory, care HolidayGuestCarePlan, staff HolidayGuestStaff, accessLog DataAccessAppender, today func() calendar.Date) enrollmentOwner.HolidayGuests {
	return enrollmentCompose.NewHolidayGuests(NewHolidayGuestPeople(people), NewHolidayGuestCareDays(care, staff),
		NewHolidayGuestAccessLog(accessLog), today)
}

// schedulerHolidayGuests binds the Worker's hourly reconciliation to the
// holiday guests of the school whose tenant transaction the Worker opened.
func schedulerHolidayGuests(people HolidayGuestDirectory, care HolidayGuestCarePlan, staff HolidayGuestStaff, accessLog DataAccessAppender) schedulerPorts.HolidayGuestReconciliation {
	guests := NewHolidayGuests(people, care, staff, accessLog, nil)
	return func(ctx context.Context) error {
		return guests.ReconcileHolidayGuests(ctx, tenant.FromContext(ctx))
	}
}

// NewHolidayGuestPeople binds the child of a holiday guest (#4666, ADR 0120)
// to the People Directory: a person with names and date of birth and an
// student of the class "Feriengast" whose care covers the booked week:
// pending and in no group before the week, active in the group the school
// picked from its first day.
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
		EnrolledFrom: from, EnrolledUntil: until, Pending: record.Status == peopledirectory.StudentStatusPending,
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
	status := peopledirectory.StudentStatusActive
	if child.Pending {
		status = peopledirectory.StudentStatusPending
	}
	return peopledirectory.EnrollmentStudent{
		PersonID: personID, SchoolClass: enrollmentOwner.HolidayGuestSchoolClass, Status: status,
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

// HolidayGuestCarePlan is the Care Plan surface the booked days of a holiday
// guest are written through: the dated arrival and pickup exceptions.
type HolidayGuestCarePlan interface {
	CreateArrivalException(context.Context, careplan.ArrivalException) (careplan.ArrivalException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
}

// HolidayGuestStaff resolves the staff member of the account a plan is
// written for: Care Plan records its author.
type HolidayGuestStaff interface {
	ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error)
}

// NewHolidayGuestCareDays binds the booked days of a holiday guest (#4666,
// ADR 0120) to Care Plan: a dated arrival and pickup exception for each day,
// with the times of the booked time model and the reason "Ferienbetreuung",
// so the day planning of the group expects the guest on exactly those days.
func NewHolidayGuestCareDays(care HolidayGuestCarePlan, staff HolidayGuestStaff) enrollmentOwner.HolidayGuestCareDays {
	if care == nil || staff == nil {
		return nil
	}
	return holidayGuestCareDays{care: care, staff: staff}
}

type holidayGuestCareDays struct {
	care  HolidayGuestCarePlan
	staff HolidayGuestStaff
}

const holidayGuestCareReason = "Ferienbetreuung"

func (c holidayGuestCareDays) PlanHolidayGuestDays(ctx context.Context, studentID, actorAccountID int64, days []calendar.Date, arrival, pickup string) error {
	if actorAccountID <= 0 {
		return enrollmentOwner.ErrHolidayGuestPlanAuthorMissing
	}
	staffID, err := c.staff.ResolveStaffIDByAccountID(ctx, actorAccountID)
	if err != nil {
		return fmt.Errorf("%w: %w", enrollmentOwner.ErrHolidayGuestPlanAuthorMissing, err)
	}
	arrivalAt, err := holidayGuestClock(arrival)
	if err != nil {
		return err
	}
	pickupAt, err := holidayGuestClock(pickup)
	if err != nil {
		return err
	}
	for _, day := range days {
		// Fresh values per day: the insert scans the stored row back into
		// the pointed-to values.
		arrivalAt, pickupAt, reason := arrivalAt, pickupAt, holidayGuestCareReason
		if _, err := c.care.CreateArrivalException(ctx, careplan.ArrivalException{
			StudentID: studentID, ExceptionDate: careplan.Date(day), ExpectedArrival: &arrivalAt, Reason: &reason,
			Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
		}); err != nil {
			return err
		}
		if _, err := c.care.CreatePickupException(ctx, careplan.PickupException{
			StudentID: studentID, ExceptionDate: careplan.Date(day), PickupTime: &pickupAt, Reason: &reason,
			Source: careplan.ExceptionSourceStaff, CreatedBy: staffID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (c holidayGuestCareDays) ClearHolidayGuestDays(ctx context.Context, studentID int64) error {
	if err := c.care.DeleteArrivalExceptionsByStudent(ctx, studentID); err != nil {
		return err
	}
	return c.care.DeletePickupExceptionsByStudent(ctx, studentID)
}

// holidayGuestClock is the wall clock of a time model ("08:00") for a Care
// Plan TIME column, anchored like Care Plan's own HTTP writes.
func holidayGuestClock(value string) (time.Time, error) {
	parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+value)
	if err != nil {
		return time.Time{}, fmt.Errorf("holiday guest care days: clock %q: %w", value, err)
	}
	return parsed, nil
}
