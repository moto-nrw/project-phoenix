package api

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	staffCalendarHTTP "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/http"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/staffdirectoryview"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
)

// The adapters in this file serve the legacy users.StaffRepository,
// users.TeacherRepository and users.GuestRepository interfaces over the School
// Membership capability. The membership rows come from the owner; the person
// half comes from the People Directory repository and the account, role and
// permission half from identity access. Nothing here builds SQL — a
// composition that used to be one join is now one call per owner.

// staffIdentityQuery is the consumer-owned account-facts port for staff projections.
type staffIdentityQuery interface {
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
	CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, error)
	ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// staffMembershipDeps binds staff projections to their owners without legacy auth repositories.
type staffMembershipDeps struct {
	persons  staffPersonRows
	identity staffIdentityQuery
	// employment is Workforce's owner of the employment-only staff writes
	// (#2753): notes and the birthday opt-out.
	employment staffEmploymentWrites
	// groupTeachers are the teaching assignments the teacher projections
	// resolve their groups through.
	groupTeachers groupTeacherRows
}

// groupTeacherRows are the teaching assignments of one group.
type groupTeacherRows interface {
	FindByGroup(ctx context.Context, groupID int64) ([]*schoolmembership.GroupAssignment, error)
}

// staffEmploymentWrites are Workforce's employment-only staff writes the
// live staff employment runs inside the membership lock.
type staffEmploymentWrites interface {
	AppendStaffNotes(ctx context.Context, membershipID int64, notes string) (workforce.StaffEmployment, error)
	SetStaffBirthdayDisplayOptOut(ctx context.Context, membershipID int64, optOut bool) error
}

// staffPersonRows are the person reads the staff projections attach.
type staffPersonRows interface {
	FindByID(ctx context.Context, id interface{}) (*peopleValues.DirectoryPersonSnapshot, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
}

func NewStaffMembershipDeps(persons staffPersonRows, identity staffIdentityQuery, employment workforce.StaffEmploymentCommand, groupTeachers groupTeacherRows) *staffMembershipDeps {
	return &staffMembershipDeps{persons: persons, identity: identity, employment: employment, groupTeachers: groupTeachers}
}

// StaffMembershipRepository serves users.StaffRepository.
type StaffMembershipRepository struct {
	Membership schoolmembership.Capability
	Deps       *staffMembershipDeps
}

var _ peoplePorts.StaffRepository = StaffMembershipRepository{}

// membershipNotFound rebuilds the not-found error shape the legacy
// repositories returned, so callers keep classifying with both
// errors.Is(err, sql.ErrNoRows) and peoplePorts.IsRepositoryNoRows(err).
func membershipNotFound(op string) error { return repositoryNotFound(op) }

// membershipError maps an owner error onto the legacy repository error shape.
func membershipError(op string, err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, schoolmembership.ErrStaffNotFound),
		errors.Is(err, schoolmembership.ErrTeacherNotFound),
		errors.Is(err, schoolmembership.ErrGuestNotFound):
		return membershipNotFound(op)
	case errors.Is(err, schoolmembership.ErrPersonnelNumberConflict):
		// Classified in the legacy vocabulary, so the service layer keeps
		// recognizing the duplicate without importing the owner.
		return repositoryError(op, peopleValues.ErrPersonnelNumberConflict)
	default:
		return repositoryError(op, err)
	}
}

func membershipID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}

func staffFieldsFromLegacy(staff *peopleValues.Staff) schoolmembership.StaffFields {
	return schoolmembership.StaffFields{
		PersonID:              staff.PersonID,
		StaffNotes:            staff.StaffNotes,
		EmploymentType:        staff.EmploymentType,
		WorkTimeModelID:       staff.WorkTimeModelID,
		PersonnelNumber:       staff.PersonnelNumber,
		RotationAnchorDate:    calendarDateString(staff.RotationAnchorDate),
		BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut,
	}
}

func applyStaffToLegacy(target *peopleValues.Staff, value schoolmembership.Staff) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.PersonID = value.PersonID
	target.StaffNotes = value.StaffNotes
	target.EmploymentType = value.EmploymentType
	target.WorkTimeModelID = value.WorkTimeModelID
	target.PersonnelNumber = value.PersonnelNumber
	target.RotationAnchorDate = parseCalendarDate(value.RotationAnchorDate)
	target.BirthdayDisplayOptOut = value.BirthdayDisplayOptOut
	target.DeletedAt = value.DeletedAt
	target.IsGuest = value.IsGuest
}

func toLegacyStaff(value schoolmembership.Staff) *peopleValues.Staff {
	staff := new(peopleValues.Staff)
	applyStaffToLegacy(staff, value)
	return staff
}

func toLegacyStaffList(values []schoolmembership.Staff) []*peopleValues.Staff {
	result := make([]*peopleValues.Staff, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyStaff(value))
	}
	return result
}

// --- CRUD ---

func (r StaffMembershipRepository) Create(ctx context.Context, entity *peopleValues.Staff) error {
	if entity == nil {
		return repositoryError("create staff", errors.New("staff cannot be nil"))
	}
	created, err := r.Membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: staffFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create staff", err)
	}
	applyStaffToLegacy(entity, created)
	return nil
}

func (r StaffMembershipRepository) Update(ctx context.Context, entity *peopleValues.Staff) error {
	if entity == nil {
		return repositoryError("update staff", errors.New("staff cannot be nil"))
	}
	updated, err := r.Membership.UpdateStaff(ctx, schoolmembership.UpdateStaff{ID: entity.ID, StaffFields: staffFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("update staff", err)
	}
	applyStaffToLegacy(entity, updated)
	return nil
}

func (r StaffMembershipRepository) Delete(ctx context.Context, id any) error {
	staffID, err := membershipID(id)
	if err != nil {
		return repositoryError("delete staff", err)
	}
	return membershipError("delete staff", r.Membership.DeleteStaff(ctx, staffID))
}

func (r StaffMembershipRepository) FindByID(ctx context.Context, id any) (*peopleValues.Staff, error) {
	staffID, err := membershipID(id)
	if err != nil {
		return nil, repositoryError("find staff by id", err)
	}
	value, err := r.Membership.FindStaff(ctx, staffID)
	if err != nil {
		return nil, membershipError("find staff by id", err)
	}
	return toLegacyStaff(value), nil
}

func (r StaffMembershipRepository) FindByIDForUpdate(ctx context.Context, id int64) (*peopleValues.Staff, error) {
	value, err := r.Membership.FindStaffForMutation(ctx, id)
	if err != nil {
		return nil, membershipError("find staff by id for update", err)
	}
	return toLegacyStaff(value), nil
}

func (r StaffMembershipRepository) FindByPersonID(ctx context.Context, personID int64) (*peopleValues.Staff, error) {
	value, err := r.Membership.FindStaffByPerson(ctx, personID)
	if err != nil {
		return nil, membershipError("find by person ID", err)
	}
	return toLegacyStaff(value), nil
}

// List keeps the legacy equality-filter shape. Only the keys real callers pass
// are supported; anything else is an explicit error instead of a silently
// ignored filter.
func (r StaffMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*peopleValues.Staff, error) {
	filter := schoolmembership.StaffFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		switch field {
		case "id":
			id, err := membershipID(value)
			if err != nil {
				return nil, repositoryError("list staff", err)
			}
			filter.IDs = append(filter.IDs, id)
		case "person_id":
			id, err := membershipID(value)
			if err != nil {
				return nil, repositoryError("list staff", err)
			}
			filter.PersonIDs = append(filter.PersonIDs, id)
		default:
			return nil, repositoryError("list staff", fmt.Errorf("unsupported staff filter %q", field))
		}
	}
	values, err := r.Membership.ListStaff(ctx, filter)
	if err != nil {
		return nil, membershipError("list staff", err)
	}
	return toLegacyStaffList(values), nil
}

// --- person compositions ---

// hydrateStaffPersons attaches the person rows to the staff members. Persons
// that are gone (soft-deleted or missing) leave Person nil, which is what the
// LEFT JOIN produced.
func (r StaffMembershipRepository) hydrateStaffPersons(ctx context.Context, members []*peopleValues.Staff) error {
	if len(members) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(members))
	for _, member := range members {
		if member.PersonID > 0 {
			ids = append(ids, member.PersonID)
		}
	}
	persons, err := r.Deps.persons.FindByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, member := range members {
		if person, found := persons[member.PersonID]; found {
			member.Person = person
		}
		// The People Directory owns the account link, so School Membership
		// resolves the marker only after its projection is attached.
		member.IsGuest = schoolmembership.ExternalCaregiver(member.IsGuest, member.Person != nil, member.Person != nil && member.Person.AccountID != nil)
	}
	return nil
}

func (r StaffMembershipRepository) ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error) {
	values, err := r.Membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, membershipError("list all with person", err)
	}
	members := toLegacyStaffList(values)
	if err := r.hydrateStaffPersons(ctx, members); err != nil {
		return nil, repositoryError("list all with person", err)
	}
	return members, nil
}

func (r StaffMembershipRepository) FindWithPerson(ctx context.Context, id int64) (*peopleValues.Staff, error) {
	value, err := r.Membership.FindStaff(ctx, id)
	if err != nil {
		return nil, membershipError("find with person - staff", err)
	}
	staff := toLegacyStaff(value)
	if staff.PersonID > 0 {
		person, personErr := r.Deps.persons.FindByID(ctx, staff.PersonID)
		switch {
		case personErr == nil:
			staff.Person = person
		case peoplePorts.IsRepositoryNoRows(personErr):
			// Person not found is acceptable — staff.Person stays nil.
		default:
			return nil, repositoryError("find with person - load person", personErr)
		}
	}
	return staff, nil
}

func (r StaffMembershipRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error) {
	if len(ids) == 0 {
		return make(map[int64]*peopleValues.Staff), nil
	}
	// Live rows only: the legacy query scanned the Staff model, whose
	// soft-delete tag excluded offboarded staff, and the callers validate
	// assignable supervisors and live ledger staff with it.
	values, err := r.Membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids})
	if err != nil {
		return nil, membershipError("find by IDs", err)
	}
	result := make(map[int64]*peopleValues.Staff, len(values))
	for _, member := range toLegacyStaffList(values) {
		result[member.ID] = member
	}
	return result, nil
}

func (r StaffMembershipRepository) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error) {
	if len(ids) == 0 {
		return make(map[int64]*peopleValues.Staff), nil
	}
	// Offboarded staff stay resolvable here: the legacy query scanned a plain
	// result struct without the soft-delete filter, and the historical
	// projections (audit log, exports) rely on that.
	values, err := r.Membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids, IncludeDeleted: true})
	if err != nil {
		return nil, membershipError("find with person by IDs", err)
	}
	members := toLegacyStaffList(values)
	if err := r.hydrateStaffPersons(ctx, members); err != nil {
		return nil, repositoryError("find with person by IDs", err)
	}
	result := make(map[int64]*peopleValues.Staff, len(members))
	for _, member := range members {
		result[member.ID] = member
	}
	return result, nil
}

// The account, contact and birthday reads are the staff directory view's
// joins; the methods below bind the owners and map its answers onto the
// legacy shapes.

// staffDirectory binds the staff directory view to the membership in use.
func (r StaffMembershipRepository) staffDirectory() (*staffdirectoryview.Directory, error) {
	return staffdirectoryview.New(staffdirectoryview.Dependencies{
		Members: r.Membership, Persons: staffViewPersons{rows: r.Deps.persons}, Accounts: r.Deps.identity,
		CallerTenant: peopleCompose.CallerTenantID, HasPermission: securityruntime.HasPermission,
	})
}

// staffDirectoryError maps the view's errors onto the legacy repository
// shapes: a School Membership failure like every membership error, a missing
// contact as not found, anything else as a repository error.
func staffDirectoryError(op string, err error) error {
	var membership *staffdirectoryview.MembershipError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &membership):
		return membershipError(op, membership.Err)
	case errors.Is(err, staffdirectoryview.ErrStaffContactNotFound):
		return membershipNotFound(op)
	default:
		return repositoryError(op, err)
	}
}

// staffViewPersons maps the People Directory person rows onto the view's
// persons; a row the repository does not find is the view's not-found.
type staffViewPersons struct{ rows staffPersonRows }

func (p staffViewPersons) Person(ctx context.Context, id int64) (staffdirectoryview.Person, error) {
	person, err := p.rows.FindByID(ctx, id)
	if err != nil {
		if peoplePorts.IsRepositoryNoRows(err) {
			return staffdirectoryview.Person{}, staffdirectoryview.ErrPersonNotFound
		}
		return staffdirectoryview.Person{}, err
	}
	if person == nil {
		return staffdirectoryview.Person{}, staffdirectoryview.ErrPersonNotFound
	}
	return staffViewPerson(person), nil
}

func (p staffViewPersons) PersonsByID(ctx context.Context, ids []int64) (map[int64]staffdirectoryview.Person, error) {
	rows, err := p.rows.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]staffdirectoryview.Person, len(rows))
	for id, person := range rows {
		if person != nil {
			result[id] = staffViewPerson(person)
		}
	}
	return result, nil
}

func staffViewPerson(person *peopleValues.DirectoryPersonSnapshot) staffdirectoryview.Person {
	return staffdirectoryview.Person{
		ID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
		AccountID: person.AccountID, Birthday: person.Birthday,
	}
}

func staffRoleInfo(contact staffdirectoryview.StaffContact) *peopleValues.StaffWithRoleInfo {
	return &peopleValues.StaffWithRoleInfo{
		StaffID: contact.StaffID, CreatedAt: contact.CreatedAt, UpdatedAt: contact.UpdatedAt,
		PersonID: contact.PersonID, FirstName: contact.FirstName, LastName: contact.LastName,
		AccountID: contact.AccountID, Email: contact.Email,
	}
}

func staffRoleInfos(contacts []staffdirectoryview.StaffContact) []*peopleValues.StaffWithRoleInfo {
	result := make([]*peopleValues.StaffWithRoleInfo, 0, len(contacts))
	for _, contact := range contacts {
		result = append(result, staffRoleInfo(contact))
	}
	return result
}

func staffBirthdayEntries(birthdays []staffdirectoryview.StaffBirthday) []peopleCompose.BirthdayEntry {
	if birthdays == nil {
		return nil
	}
	entries := make([]peopleCompose.BirthdayEntry, 0, len(birthdays))
	for _, birthday := range birthdays {
		entries = append(entries, peopleCompose.BirthdayEntry{
			Kind: peopleCompose.BirthdayKindStaff, ID: birthday.StaffID,
			FirstName: birthday.FirstName, LastName: birthday.LastName, Birthday: birthday.Birthday,
		})
	}
	return entries
}

func (r StaffMembershipRepository) ListAccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error) {
	const op = "list account IDs by staff IDs"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	result, err := directory.AccountIDsByStaffIDs(ctx, staffIDs)
	return result, staffDirectoryError(op, err)
}

func (r StaffMembershipRepository) ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error) {
	const op = "list all staff account IDs"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	result, err := directory.AllStaffAccountIDs(ctx)
	return result, staffDirectoryError(op, err)
}

func (r StaffMembershipRepository) ListStaffWithPermission(ctx context.Context, permissionName string) ([]*peopleValues.StaffWithRoleInfo, error) {
	const op = "list staff with permission"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	contacts, err := directory.StaffWithPermission(ctx, permissionName)
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	return staffRoleInfos(contacts), nil
}

// FindReachableCalendarStaffIDs keeps the staff members whose account holds
// the permission School Calendar invites by.
func (r StaffMembershipRepository) FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	const op = "find reachable calendar staff"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	result, err := directory.ReachableStaffIDs(ctx, ids, staffCalendarHTTP.StaffInvitePermission)
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	return result, nil
}

func (r StaffMembershipRepository) GetStaffContactInfo(ctx context.Context, staffID int64) (*peopleValues.StaffWithRoleInfo, error) {
	const op = "get staff contact info"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	contact, err := directory.StaffContact(ctx, staffID)
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	return staffRoleInfo(contact), nil
}

func (r StaffMembershipRepository) ListStaffByRoles(ctx context.Context, roles []string) ([]*peopleValues.StaffWithRoleInfo, error) {
	const op = "list staff by roles"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	contacts, err := directory.StaffByRoles(ctx, roles)
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	return staffRoleInfos(contacts), nil
}

func (r StaffMembershipRepository) FindBirthdaysOn(ctx context.Context, days []peopleCompose.BirthdayMonthDay) ([]peopleCompose.BirthdayEntry, error) {
	const op = "find staff birthdays"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	wanted := make([]staffdirectoryview.MonthDay, 0, len(days))
	for _, day := range days {
		wanted = append(wanted, staffdirectoryview.MonthDay{Month: day.Month, Day: day.Day})
	}
	birthdays, err := directory.BirthdaysOn(ctx, wanted)
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	return staffBirthdayEntries(birthdays), nil
}

func (r StaffMembershipRepository) ListBirthdaysForExport(ctx context.Context) ([]peopleCompose.BirthdayEntry, error) {
	const op = "list staff birthdays for export"
	directory, err := r.staffDirectory()
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	birthdays, err := directory.AllBirthdays(ctx)
	if err != nil {
		return nil, staffDirectoryError(op, err)
	}
	return staffBirthdayEntries(birthdays), nil
}

// The employment-only writes go to their Workforce owner through the
// staff offboarding workflow's live-staff guard: the opt-out and the notes
// apply to live staff members only, as before the split.

func (r StaffMembershipRepository) SetBirthdayDisplayOptOut(ctx context.Context, staffID int64, optOut bool) error {
	const op = "set staff birthday display opt-out"
	employment, err := staffoffboarding.NewLiveStaffEmployment(r.Membership, r.Deps.employment)
	if err != nil {
		return membershipError(op, err)
	}
	return membershipError(op, MembershipEmploymentError(employment.SetBirthdayDisplayOptOut(ctx, staffID, optOut)))
}

func (r StaffMembershipRepository) AddNotes(ctx context.Context, id int64, notes string) error {
	employment, err := staffoffboarding.NewLiveStaffEmployment(r.Membership, r.Deps.employment)
	if err != nil {
		return membershipError("add notes", err)
	}
	return membershipError("add notes", MembershipEmploymentError(employment.AppendNotes(ctx, id, notes)))
}
