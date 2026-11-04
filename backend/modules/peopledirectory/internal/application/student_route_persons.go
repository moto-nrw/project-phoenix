package application

import (
	"context"
	"errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentRoutePersons binds the person half of the student routes'
// PersonRecords port (#2731) to the retained person directory, which still
// decides it: the person writes with their account and RFID-card checks, the
// student-aware bracelet assignment, the dated day-log roster and the staff
// member behind a person. It only translates between the owner's public types
// and the retained rows.
type StudentRoutePersons struct {
	studentRoutePersonRows
	persons peoplePorts.PersonDirectory
}

// studentRoutePersonRows are the directory writes the student routes take as
// they are: the person delete and the bracelet release.
type studentRoutePersonRows interface {
	Delete(ctx context.Context, id interface{}) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
}

// NewStudentRoutePersons binds the retained person directory.
func NewStudentRoutePersons(persons peoplePorts.PersonDirectory) StudentRoutePersons {
	return StudentRoutePersons{studentRoutePersonRows: persons, persons: persons}
}

// CreatePerson validates and inserts the person through the retained directory.
func (p StudentRoutePersons) CreatePerson(ctx context.Context, input peopleModule.CreatePerson) (peopleModule.Person, error) {
	person := &peopleModule.DirectoryPersonSnapshot{
		FirstName: input.FirstName, LastName: input.LastName,
		Birthday: peopleModule.OptionalCalendarDate(input.Birthday),
		TagID:    input.TagID, AccountID: input.AccountID,
	}
	if err := p.persons.Create(ctx, person); err != nil {
		return peopleModule.Person{}, err
	}
	return studentRoutePerson(person), nil
}

// UpdatePerson rewrites the person through the retained directory.
func (p StudentRoutePersons) UpdatePerson(ctx context.Context, input peopleModule.UpdatePerson) (peopleModule.Person, error) {
	person := &peopleModule.DirectoryPersonSnapshot{
		FirstName: input.FirstName, LastName: input.LastName,
		Birthday: peopleModule.OptionalCalendarDate(input.Birthday),
		TagID:    input.TagID, AccountID: input.AccountID,
	}
	person.ID = input.ID
	if err := p.persons.Update(ctx, person); err != nil {
		return peopleModule.Person{}, err
	}
	return studentRoutePerson(person), nil
}

// AssignStudentTag links the bracelet to the child's person. The retained
// refusals of a graduated or missing child become the owner's not-found.
func (p StudentRoutePersons) AssignStudentTag(ctx context.Context, studentID int64, tagID string) error {
	err := p.persons.LinkStudentToRFIDCard(ctx, studentID, tagID)
	if errors.Is(err, userscontract.ErrStudentGraduated) || errors.Is(err, userscontract.ErrStudentNotFound) {
		return errors.Join(peopleModule.ErrStudentNotFound, err)
	}
	return err
}

// ListStudentsForDay returns the group children whose care covers the day.
func (p StudentRoutePersons) ListStudentsForDay(ctx context.Context, groupIDs []int64, date, today calendar.Date) ([]peopleModule.StudentRecord, error) {
	students, err := p.persons.GetEligibleStudentsByGroupIDsOnDate(ctx, groupIDs, date, today)
	if err != nil {
		return nil, err
	}
	return studentRouteRecords(students), nil
}

// ListSchoolStudentsForDay returns every child of the school whose care
// covers the day, with a group or without one, each once: a school in open
// care keeps no fixed groups, so its weekly protocol counts them all (#5401).
// The unfiltered class roster is every child of the school, graduates
// included, and the group roster's participation rule decides them.
func (p StudentRoutePersons) ListSchoolStudentsForDay(ctx context.Context, date, today calendar.Date) ([]peopleModule.StudentRecord, error) {
	candidates, err := p.persons.StudentRows().ListClassRoster(ctx, "")
	if err != nil {
		return nil, err
	}
	students, err := p.persons.FilterEligibleStudentsOnDate(ctx, candidates, date, today)
	if err != nil {
		return nil, err
	}
	return studentRouteRecords(students), nil
}

func studentRouteRecords(students []*peopleModule.StudentState) []peopleModule.StudentRecord {
	records := make([]peopleModule.StudentRecord, 0, len(students))
	for _, student := range students {
		if student != nil {
			records = append(records, studentRouteRecord(student))
		}
	}
	return records
}

// StaffIDForPerson resolves the staff member of a person; a person who is no
// staff member is reported as not found rather than as an error.
func (p StudentRoutePersons) StaffIDForPerson(ctx context.Context, personID int64) (int64, bool, error) {
	staff, err := p.persons.StaffRows().FindByPersonID(ctx, personID)
	if err != nil {
		var notFound interface{ RepositoryNotFound() }
		if errors.As(err, &notFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if staff == nil {
		return 0, false, nil
	}
	return staff.ID, true, nil
}

func studentRoutePerson(person *peopleModule.DirectoryPersonSnapshot) peopleModule.Person {
	return peopleModule.Person{
		ID: person.ID, CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt, TenantID: person.TenantID,
		FirstName: person.FirstName, LastName: person.LastName,
		Birthday: peopleModule.RenderCalendarDate(person.Birthday),
		TagID:    person.TagID, AccountID: person.AccountID, DeletedAt: person.DeletedAt,
	}
}

// studentRouteRecord projects a retained child row onto the owner's record,
// the departure plan included.
func studentRouteRecord(student *peopleModule.StudentState) peopleModule.StudentRecord {
	return peopleModule.StudentRecord{
		ID: student.ID, CreatedAt: student.CreatedAt, UpdatedAt: student.UpdatedAt, TenantID: student.TenantID,

		PersonID: student.PersonID, SchoolClass: student.SchoolClass,
		GroupID: student.GroupID, Status: string(student.Status),

		EnrolledFrom:  peopleModule.RenderCalendarDate(student.EnrolledFrom),
		EnrolledUntil: peopleModule.RenderCalendarDate(student.EnrolledUntil),

		AddressStreet:     student.AddressStreet,
		AddressCity:       student.AddressCity,
		AddressPostalCode: student.AddressPostalCode,

		ExtraInfo:    student.ExtraInfo,
		HealthInfo:   student.HealthInfo,
		PickupStatus: student.PickupStatus,

		DepartureDays:          student.DepartureDays,
		AllowedDepartureModes:  student.AllowedDepartureModes,
		PickupDays:             student.PickupDays,
		BusDays:                student.BusDays,
		DepartureCompanionNote: student.DepartureCompanionNote,

		Sick:         student.Sick,
		SickSince:    student.SickSince,
		Excused:      student.Excused,
		ExcusedSince: student.ExcusedSince,

		PhotoPath:           student.PhotoPath,
		PhotoConsentGivenAt: student.PhotoConsentGivenAt,
		PhotoConsentGivenBy: student.PhotoConsentGivenBy,

		AGBAcceptedAt:            student.AGBAcceptedAt,
		DataProcessingAcceptedAt: student.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   student.EmailContactAcceptedAt,
	}
}
