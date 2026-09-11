package importpkg

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/moto-nrw/project-phoenix/services/import/ports"
)

// In-memory owner fakes for the decision tests of the import package. They
// implement only the port methods the configs call; anything else panics so
// a test cannot silently depend on an unmodelled owner behaviour.

type fakePersons struct {
	mu      sync.Mutex
	nextID  int64
	persons map[int64]ports.Person
	created []ports.CreatePerson
	updated []ports.UpdatePerson
	err     error
}

func newFakePersons(persons ...ports.Person) *fakePersons {
	f := &fakePersons{persons: map[int64]ports.Person{}}
	for _, person := range persons {
		f.persons[person.ID] = person
		if person.ID > f.nextID {
			f.nextID = person.ID
		}
	}
	return f
}

func (f *fakePersons) CreatePerson(_ context.Context, input ports.CreatePerson) (ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return ports.Person{}, f.err
	}
	f.nextID++
	person := ports.Person{ID: f.nextID, FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday, TagID: input.TagID, AccountID: input.AccountID}
	f.persons[person.ID] = person
	f.created = append(f.created, input)
	return person, nil
}

func (f *fakePersons) UpdatePerson(_ context.Context, input ports.UpdatePerson) (ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	person, ok := f.persons[input.ID]
	if !ok {
		return ports.Person{}, ports.ErrPersonNotFound
	}
	person.FirstName, person.LastName, person.Birthday, person.TagID, person.AccountID = input.FirstName, input.LastName, input.Birthday, input.TagID, input.AccountID
	f.persons[input.ID] = person
	f.updated = append(f.updated, input)
	return person, nil
}

func (f *fakePersons) FindPerson(_ context.Context, id int64) (ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return ports.Person{}, f.err
	}
	person, ok := f.persons[id]
	if !ok {
		return ports.Person{}, ports.ErrPersonNotFound
	}
	return person, nil
}

func (f *fakePersons) FindPersonByTag(_ context.Context, tag string) (ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, person := range f.persons {
		if person.TagID != nil && *person.TagID == tag {
			return person, nil
		}
	}
	return ports.Person{}, ports.ErrPersonNotFound
}

func (f *fakePersons) FindPersonByAccount(_ context.Context, accountID int64) (ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, person := range f.persons {
		if person.AccountID != nil && *person.AccountID == accountID {
			return person, nil
		}
	}
	return ports.Person{}, ports.ErrPersonNotFound
}

func (f *fakePersons) SearchPersons(_ context.Context, filter ports.PersonFilter) ([]ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var result []ports.Person
	for _, person := range f.persons {
		if filter.FirstNamePrefix != "" && !strings.HasPrefix(strings.ToLower(person.FirstName), strings.ToLower(filter.FirstNamePrefix)) {
			continue
		}
		if filter.LastNamePrefix != "" && !strings.HasPrefix(strings.ToLower(person.LastName), strings.ToLower(filter.LastNamePrefix)) {
			continue
		}
		if filter.FirstNameEquals != "" && !strings.EqualFold(strings.TrimSpace(person.FirstName), filter.FirstNameEquals) {
			continue
		}
		if filter.LastNameEquals != "" && !strings.EqualFold(strings.TrimSpace(person.LastName), filter.LastNameEquals) {
			continue
		}
		result = append(result, person)
	}
	return result, nil
}

func (f *fakePersons) ListPersonsByID(_ context.Context, ids []int64) ([]ports.Person, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []ports.Person
	for _, id := range ids {
		if person, ok := f.persons[id]; ok {
			result = append(result, person)
		}
	}
	return result, nil
}

type fakeStudents struct {
	mu       sync.Mutex
	students []ports.Student
	err      error
}

func (f *fakeStudents) ListStudentsByPersonID(_ context.Context, ids []int64) ([]ports.Student, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	var result []ports.Student
	for _, student := range f.students {
		for _, id := range ids {
			if student.PersonID == id {
				result = append(result, student)
			}
		}
	}
	return result, nil
}

func (f *fakeStudents) ReadEnrollmentStudent(context.Context, int64, string) (ports.EnrollmentRecord, error) {
	panic("not modelled")
}

func (f *fakeStudents) CreateEnrollmentStudent(context.Context, ports.EnrollmentStudent) (ports.CreatedEnrollmentStudent, error) {
	panic("not modelled")
}

func (f *fakeStudents) RenewEnrollmentStudent(context.Context, int64, ports.EnrollmentStudent) error {
	panic("not modelled")
}

func (f *fakeStudents) ApplyEnrollmentProfile(context.Context, int64, ports.EnrollmentProfilePatch) error {
	panic("not modelled")
}

type fakeClassList struct {
	mu      sync.Mutex
	nextID  int64
	entries []ports.ClassListEntry
	created []ports.CreateClassListEntry
	listErr error
	create  func(ports.CreateClassListEntry) (ports.ClassListEntry, error)
}

func (f *fakeClassList) ListClassListEntries(_ context.Context, filter ports.ClassListEntryFilter) ([]ports.ClassListEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listErr != nil {
		return nil, f.listErr
	}
	var result []ports.ClassListEntry
	for _, entry := range f.entries {
		if strings.EqualFold(strings.TrimSpace(entry.FirstName), filter.FirstName) &&
			strings.EqualFold(strings.TrimSpace(entry.LastName), filter.LastName) &&
			strings.EqualFold(strings.TrimSpace(entry.SchoolClass), filter.SchoolClass) {
			result = append(result, entry)
		}
	}
	return result, nil
}

func (f *fakeClassList) CreateClassListEntry(_ context.Context, input ports.CreateClassListEntry) (ports.ClassListEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.create != nil {
		return f.create(input)
	}
	// The owner's unique index over the case-folded name and class, which is
	// what refuses a racing second create.
	for _, entry := range f.entries {
		if strings.EqualFold(strings.TrimSpace(entry.FirstName), strings.TrimSpace(input.FirstName)) &&
			strings.EqualFold(strings.TrimSpace(entry.LastName), strings.TrimSpace(input.LastName)) &&
			strings.EqualFold(strings.TrimSpace(entry.SchoolClass), strings.TrimSpace(input.SchoolClass)) {
			return ports.ClassListEntry{}, ports.ErrMembershipClassListEntryDuplicate
		}
	}
	f.nextID++
	entry := ports.ClassListEntry{ID: f.nextID, FirstName: input.FirstName, LastName: input.LastName, SchoolClass: input.SchoolClass, CreatedBy: input.CreatedBy}
	f.entries = append(f.entries, entry)
	f.created = append(f.created, input)
	return entry, nil
}

type fakeAudit struct {
	mu     sync.Mutex
	events []any
	err    error
}

func (f *fakeAudit) Append(_ context.Context, event any) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, event)
	return nil
}

var errFakeOwner = errors.New("kaputt")

// fakeImporterIDs hands each decision test its own importer id, so no test
// depends on a literal one another test could collide with.
var fakeImporterIDs atomic.Int64

func fakeImporterID() int64 { return fakeImporterIDs.Add(1) }

// fakeGuardians records guardian profiles, phone numbers and links the
// import creates; lookups answer from the recorded state.
type fakeGuardians struct {
	mu        sync.Mutex
	nextID    int64
	guardians map[int64]ports.Guardian
	phones    map[int64][]ports.GuardianPhone
	links     map[int64][]ports.GuardianLink
	linked    []ports.LinkGuardian
	updated   []ports.GuardianLinkUpdate
	linkErr   error
}

func newFakeGuardians(guardians ...ports.Guardian) *fakeGuardians {
	f := &fakeGuardians{guardians: map[int64]ports.Guardian{}, phones: map[int64][]ports.GuardianPhone{}, links: map[int64][]ports.GuardianLink{}}
	for _, guardian := range guardians {
		f.guardians[guardian.ID] = guardian
		if guardian.ID > f.nextID {
			f.nextID = guardian.ID
		}
	}
	return f
}

func (f *fakeGuardians) SearchGuardians(_ context.Context, text string, _ int) ([]ports.GuardianMatch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []ports.GuardianMatch
	for _, guardian := range f.guardians {
		if guardian.Email != nil && strings.Contains(strings.ToLower(*guardian.Email), strings.ToLower(text)) {
			result = append(result, ports.GuardianMatch{Guardian: guardian})
		}
	}
	return result, nil
}

func (f *fakeGuardians) ListGuardiansByID(_ context.Context, ids []int64) ([]ports.Guardian, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []ports.Guardian
	for _, id := range ids {
		if guardian, ok := f.guardians[id]; ok {
			result = append(result, guardian)
		}
	}
	return result, nil
}

func (f *fakeGuardians) ListGuardianPhones(_ context.Context, id int64) ([]ports.GuardianPhone, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ports.GuardianPhone(nil), f.phones[id]...), nil
}

func (f *fakeGuardians) ListStudentGuardians(_ context.Context, studentID int64) ([]ports.GuardianWithLink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []ports.GuardianWithLink
	for _, link := range f.links[studentID] {
		result = append(result, ports.GuardianWithLink{Guardian: f.guardians[link.GuardianProfileID], Link: link})
	}
	return result, nil
}

func (f *fakeGuardians) CreateGuardian(_ context.Context, input ports.GuardianInput) (ports.Guardian, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	guardian := ports.Guardian{
		ID: f.nextID, FirstName: input.FirstName, LastName: input.LastName, Email: input.Email,
		AddressStreet: input.AddressStreet, AddressCity: input.AddressCity, AddressPostalCode: input.AddressPostalCode,
		PreferredContactMethod: input.PreferredContactMethod, LanguagePreference: input.LanguagePreference, Notes: input.Notes,
	}
	f.guardians[guardian.ID] = guardian
	return guardian, nil
}

func (f *fakeGuardians) UpdateGuardian(_ context.Context, id int64, input ports.GuardianInput) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	guardian, ok := f.guardians[id]
	if !ok {
		return ports.ErrGuardianNotFound
	}
	guardian.FirstName, guardian.LastName, guardian.Email = input.FirstName, input.LastName, input.Email
	guardian.AddressStreet, guardian.AddressCity, guardian.AddressPostalCode = input.AddressStreet, input.AddressCity, input.AddressPostalCode
	guardian.PreferredContactMethod, guardian.LanguagePreference, guardian.Notes = input.PreferredContactMethod, input.LanguagePreference, input.Notes
	f.guardians[id] = guardian
	return nil
}

func (f *fakeGuardians) AddGuardianPhone(_ context.Context, id int64, input ports.GuardianPhoneInput) (ports.GuardianPhone, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	phone := ports.GuardianPhone{ID: f.nextID, GuardianProfileID: id, PhoneNumber: input.PhoneNumber, PhoneType: input.PhoneType, Label: input.Label, IsPrimary: input.IsPrimary, Priority: len(f.phones[id]) + 1}
	f.phones[id] = append(f.phones[id], phone)
	return phone, nil
}

func (f *fakeGuardians) LinkGuardianToStudent(_ context.Context, input ports.LinkGuardian) (ports.GuardianLink, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.linkErr != nil {
		return ports.GuardianLink{}, f.linkErr
	}
	f.nextID++
	link := ports.GuardianLink{
		ID: f.nextID, StudentID: input.StudentID, GuardianProfileID: input.GuardianProfileID,
		RelationshipType: input.RelationshipType, GuardianRole: input.GuardianRole, IsPrimary: input.IsPrimary,
		IsEmergencyContact: input.IsEmergencyContact, CanPickup: input.CanPickup, PickupNotes: input.PickupNotes, EmergencyPriority: input.EmergencyPriority,
	}
	f.links[input.StudentID] = append(f.links[input.StudentID], link)
	f.linked = append(f.linked, input)
	return link, nil
}

func (f *fakeGuardians) UpdateGuardianLink(_ context.Context, _ int64, input ports.GuardianLinkUpdate) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, input)
	return nil
}
