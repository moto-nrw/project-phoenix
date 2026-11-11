package owners

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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
// their children to the People Directory, the names of their home schools
// to Organisation & Tenancy (#5630) and every transfer and sheet read to the
// school's access log at the Audit Platform; health opens and seals the
// health information (#4722). A nil today uses the Berlin calendar day.
func NewHolidayGuests(people HolidayGuestDirectory, schools HolidayHostSchools, care HolidayGuestCarePlan, accessLog DataAccessAppender, health enrollmentOwner.HolidayHealthCipher, today func() calendar.Date) enrollmentOwner.HolidayGuests {
	return enrollmentCompose.NewHolidayGuests(NewHolidayGuestPeople(people), NewHolidayHostSchoolNames(schools), NewHolidayGuestCareDays(care), health,
		NewHolidayGuestAccessLog(accessLog), today)
}

// HolidayHostSchools is the Organisation & Tenancy read that names the
// schools of the hosting school's Träger.
type HolidayHostSchools interface {
	FindSchool(ctx context.Context, id int64) (organizationtenancy.School, error)
	ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error)
}

// NewHolidayHostSchoolNames names the schools of the hosting school's
// Träger, deleted ones included, for the home schools of the participant
// list (ADR 0116) and of the guest copies (#5630). In the school's tenant
// transaction it reads platform.schools, which the school role may read.
func NewHolidayHostSchoolNames(schools HolidayHostSchools) enrollmentOwner.HolidayHostSchoolNames {
	if schools == nil {
		return nil
	}
	return holidayHostSchoolNames{schools: schools}
}

type holidayHostSchoolNames struct {
	schools HolidayHostSchools
}

func (n holidayHostSchoolNames) HolidayHostSchoolNames(ctx context.Context, schoolID int64) (map[int64]string, error) {
	school, err := n.schools.FindSchool(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	schools, err := n.schools.ListSchoolsByOrganization(ctx, school.OrganizationID)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(schools))
	for _, sibling := range schools {
		names[sibling.ID] = sibling.Name
	}
	return names, nil
}

// schedulerHolidayGuests binds the Worker's hourly reconciliation to the
// holiday guests of the school whose tenant transaction the Worker opened.
func schedulerHolidayGuests(people HolidayGuestDirectory, schools HolidayHostSchools, care HolidayGuestCarePlan, accessLog DataAccessAppender, health enrollmentOwner.HolidayHealthCipher) schedulerPorts.HolidayGuestReconciliation {
	guests := NewHolidayGuests(people, schools, care, accessLog, health, nil)
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

// SetHolidayGuestHealthInfo writes the health information of a guest into
// the health field of its child through the People Directory's enrollment
// profile command, the one that writes the rest of the copy (#4722); nil
// clears it.
func (p holidayGuestPeople) SetHolidayGuestHealthInfo(ctx context.Context, studentID int64, healthInfo *string) error {
	return p.directory.ApplyEnrollmentProfile(ctx, studentID, peopleEnrollment.ProfilePatch{HealthInfoSet: true, HealthInfo: healthInfo})
}

func holidayGuestStudent(personID int64, child enrollmentOwner.HolidayGuestChild) peopledirectory.EnrollmentStudent {
	status := peopledirectory.StudentStatusActive
	if child.Pending {
		status = peopledirectory.StudentStatusPending
	}
	return peopledirectory.EnrollmentStudent{
		PersonID: personID, SchoolClass: peopledirectory.HolidayGuestSchoolClass, Status: status,
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
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
}

// NewHolidayGuestCareDays binds the booked days of a holiday guest (#4666,
// ADR 0120) to Care Plan: a dated arrival and pickup exception for each day,
// with the times of the booked time model and the reason "Ferienbetreuung",
// so the day planning of the group expects the guest on exactly those days.
// The booking is the author (source holiday_booking, no staff member), so the
// days do not depend on who picked the group (addendum of 2026-11-11).
func NewHolidayGuestCareDays(care HolidayGuestCarePlan) enrollmentOwner.HolidayGuestCareDays {
	if care == nil {
		return nil
	}
	return holidayGuestCareDays{care: care}
}

type holidayGuestCareDays struct {
	care HolidayGuestCarePlan
}

const holidayGuestCareReason = "Ferienbetreuung"

func (c holidayGuestCareDays) PlanHolidayGuestDays(ctx context.Context, studentID int64, days []calendar.Date, arrival, pickup string) error {
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
			Source: careplan.ExceptionSourceHolidayBooking,
		}); err != nil {
			return err
		}
		if _, err := c.care.CreatePickupException(ctx, careplan.PickupException{
			StudentID: studentID, ExceptionDate: careplan.Date(day), PickupTime: &pickupAt, Reason: &reason,
			Source: careplan.ExceptionSourceHolidayBooking,
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

// UnplannedHolidayGuests reads the arrival and pickup exceptions of the
// children in two statements and returns the children with neither.
func (c holidayGuestCareDays) UnplannedHolidayGuests(ctx context.Context, studentIDs []int64) ([]int64, error) {
	filter := careplan.StudentScheduleFilter{StudentIDs: studentIDs}
	arrivals, err := c.care.ListArrivalExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	pickups, err := c.care.ListPickupExceptions(ctx, filter)
	if err != nil {
		return nil, err
	}
	planned := make(map[int64]bool, len(studentIDs))
	for _, arrival := range arrivals {
		planned[arrival.StudentID] = true
	}
	for _, pickup := range pickups {
		planned[pickup.StudentID] = true
	}
	var unplanned []int64
	for _, studentID := range studentIDs {
		if !planned[studentID] {
			unplanned = append(unplanned, studentID)
		}
	}
	return unplanned, nil
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

// NewHolidayStudentIdentities binds the names and the date of birth of a
// child of the hosting school (#4722) to the People Directory: the student
// and its person, read in the school's tenant transaction.
func NewHolidayStudentIdentities(directory HolidayGuestDirectory) enrollmentOwner.HolidayStudentIdentities {
	if directory == nil {
		return nil
	}
	return holidayStudentIdentities{directory: directory}
}

type holidayStudentIdentities struct {
	directory HolidayGuestDirectory
}

func (i holidayStudentIdentities) HolidayStudentIdentity(ctx context.Context, studentID int64) (enrollmentOwner.HolidayStudentIdentity, error) {
	record, err := i.directory.ReadEnrollmentStudent(ctx, studentID, "")
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return enrollmentOwner.HolidayStudentIdentity{}, fmt.Errorf("%w: %w", enrollmentOwner.ErrHolidayStudentNotFound, err)
	}
	if err != nil {
		return enrollmentOwner.HolidayStudentIdentity{}, err
	}
	person, err := i.directory.FindPerson(ctx, record.PersonID)
	if err != nil {
		return enrollmentOwner.HolidayStudentIdentity{}, err
	}
	birthDate, err := optionalDate(person.Birthday)
	if err != nil {
		return enrollmentOwner.HolidayStudentIdentity{}, err
	}
	return enrollmentOwner.HolidayStudentIdentity{FirstName: person.FirstName, LastName: person.LastName, BirthDate: birthDate}, nil
}
