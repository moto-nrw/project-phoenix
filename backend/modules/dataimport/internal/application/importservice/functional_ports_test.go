package importpkg

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

type PersonDirectoryFuncs struct {
	CreatePersonFunc        func(_ context.Context, input ports.CreatePerson) (ports.Person, error)
	UpdatePersonFunc        func(_ context.Context, input ports.UpdatePerson) (ports.Person, error)
	FindPersonFunc          func(_ context.Context, id int64) (ports.Person, error)
	FindPersonByTagFunc     func(_ context.Context, tag string) (ports.Person, error)
	FindPersonByAccountFunc func(_ context.Context, accountID int64) (ports.Person, error)
	SearchPersonsFunc       func(_ context.Context, filter ports.PersonFilter) ([]ports.Person, error)
	ListPersonsByIDFunc     func(_ context.Context, ids []int64) ([]ports.Person, error)
}

func (f PersonDirectoryFuncs) CreatePerson(arg0 context.Context, arg1 ports.CreatePerson) (ports.Person, error) {
	return f.CreatePersonFunc(arg0, arg1)
}
func (f PersonDirectoryFuncs) UpdatePerson(arg0 context.Context, arg1 ports.UpdatePerson) (ports.Person, error) {
	return f.UpdatePersonFunc(arg0, arg1)
}
func (f PersonDirectoryFuncs) FindPerson(arg0 context.Context, arg1 int64) (ports.Person, error) {
	return f.FindPersonFunc(arg0, arg1)
}
func (f PersonDirectoryFuncs) FindPersonByTag(arg0 context.Context, arg1 string) (ports.Person, error) {
	return f.FindPersonByTagFunc(arg0, arg1)
}
func (f PersonDirectoryFuncs) FindPersonByAccount(arg0 context.Context, arg1 int64) (ports.Person, error) {
	return f.FindPersonByAccountFunc(arg0, arg1)
}
func (f PersonDirectoryFuncs) SearchPersons(arg0 context.Context, arg1 ports.PersonFilter) ([]ports.Person, error) {
	return f.SearchPersonsFunc(arg0, arg1)
}
func (f PersonDirectoryFuncs) ListPersonsByID(arg0 context.Context, arg1 []int64) ([]ports.Person, error) {
	return f.ListPersonsByIDFunc(arg0, arg1)
}

type GuardianDirectoryFuncs struct {
	FindGuardianByEmailFunc   func(_ context.Context, email string) (ports.Guardian, error)
	ListGuardiansByIDFunc     func(_ context.Context, ids []int64) ([]ports.Guardian, error)
	ListGuardianPhonesFunc    func(_ context.Context, id int64) ([]ports.GuardianPhone, error)
	ListStudentGuardiansFunc  func(_ context.Context, studentID int64) ([]ports.GuardianWithLink, error)
	CreateGuardianFunc        func(_ context.Context, input ports.GuardianInput) (ports.Guardian, error)
	UpdateGuardianFunc        func(_ context.Context, id int64, input ports.GuardianInput) error
	AddGuardianPhoneFunc      func(_ context.Context, id int64, input ports.GuardianPhoneInput) (ports.GuardianPhone, error)
	LinkGuardianToStudentFunc func(_ context.Context, input ports.LinkGuardian) (ports.GuardianLink, error)
	UpdateGuardianLinkFunc    func(_ context.Context, _ int64, input ports.GuardianLinkUpdate) error
}

func (f GuardianDirectoryFuncs) FindGuardianByEmail(arg0 context.Context, arg1 string) (ports.Guardian, error) {
	return f.FindGuardianByEmailFunc(arg0, arg1)
}
func (f GuardianDirectoryFuncs) ListGuardiansByID(arg0 context.Context, arg1 []int64) ([]ports.Guardian, error) {
	return f.ListGuardiansByIDFunc(arg0, arg1)
}
func (f GuardianDirectoryFuncs) ListGuardianPhones(arg0 context.Context, arg1 int64) ([]ports.GuardianPhone, error) {
	return f.ListGuardianPhonesFunc(arg0, arg1)
}
func (f GuardianDirectoryFuncs) ListStudentGuardians(arg0 context.Context, arg1 int64) ([]ports.GuardianWithLink, error) {
	return f.ListStudentGuardiansFunc(arg0, arg1)
}
func (f GuardianDirectoryFuncs) CreateGuardian(arg0 context.Context, arg1 ports.GuardianInput) (ports.Guardian, error) {
	return f.CreateGuardianFunc(arg0, arg1)
}
func (f GuardianDirectoryFuncs) UpdateGuardian(arg0 context.Context, arg1 int64, arg2 ports.GuardianInput) error {
	return f.UpdateGuardianFunc(arg0, arg1, arg2)
}
func (f GuardianDirectoryFuncs) AddGuardianPhone(arg0 context.Context, arg1 int64, arg2 ports.GuardianPhoneInput) (ports.GuardianPhone, error) {
	return f.AddGuardianPhoneFunc(arg0, arg1, arg2)
}
func (f GuardianDirectoryFuncs) LinkGuardianToStudent(arg0 context.Context, arg1 ports.LinkGuardian) (ports.GuardianLink, error) {
	return f.LinkGuardianToStudentFunc(arg0, arg1)
}
func (f GuardianDirectoryFuncs) UpdateGuardianLink(arg0 context.Context, arg1 int64, arg2 ports.GuardianLinkUpdate) error {
	return f.UpdateGuardianLinkFunc(arg0, arg1, arg2)
}

type ClassListMembershipFuncs struct {
	ListClassListEntriesFunc func(_ context.Context, filter ports.ClassListEntryFilter) ([]ports.ClassListEntry, error)
	CreateClassListEntryFunc func(_ context.Context, input ports.CreateClassListEntry) (ports.ClassListEntry, error)
}

func (f ClassListMembershipFuncs) ListClassListEntries(arg0 context.Context, arg1 ports.ClassListEntryFilter) ([]ports.ClassListEntry, error) {
	return f.ListClassListEntriesFunc(arg0, arg1)
}
func (f ClassListMembershipFuncs) CreateClassListEntry(arg0 context.Context, arg1 ports.CreateClassListEntry) (ports.ClassListEntry, error) {
	return f.CreateClassListEntryFunc(arg0, arg1)
}
