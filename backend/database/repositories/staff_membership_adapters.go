package repositories

import (
	"context"
	"errors"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The adapters in this file serve the legacy users.StaffRepository,
// users.TeacherRepository and users.GuestRepository interfaces over the School
// Membership capability. The membership rows come from the owner; the person
// half comes from the People Directory repository and the account, role and
// permission half from identity access. Nothing here builds SQL — a
// composition that used to be one join is now one call per owner.

// membershipTenantQuery is the identity-access lookup that decides which of a
// set of accounts may act at a tenant.
type membershipTenantQuery interface {
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
}

// membershipPermissionQuery resolves effective permission names per account.
type membershipPermissionQuery interface {
	FindEffectivePermissionNamesByAccountIDsForTenant(ctx context.Context, accountIDs []int64, tenantID int64) (map[int64][]string, error)
}

// membershipRoleQuery answers the two role questions the staff compositions ask.
type membershipRoleQuery interface {
	CountRoleNameMatchesByAccountIDs(ctx context.Context, accountIDs []int64, roleNames []string) (map[int64]int, error)
	ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error)
}

// staffMembershipDeps are the owner-side lookups the membership adapters
// compose with. They are captured from the concrete repositories at factory
// construction: the public interfaces stay untouched, and a wrapper that only
// forwards the interface can never hide one of these methods by accident.
type staffMembershipDeps struct {
	persons     userModels.PersonRepository
	accounts    authModels.AccountRepository
	memberships membershipTenantQuery
	permissions membershipPermissionQuery
	roles       membershipRoleQuery
	// groupTeachers is read lazily: education owns education.group_teacher and
	// the factory may still rebind that repository after construction.
	groupTeachers func() educationModels.GroupTeacherRepository
}

// newStaffMembershipDeps asserts the extra owner methods once, at construction,
// so a missing one is a startup panic instead of a runtime surprise.
func newStaffMembershipDeps(
	persons userModels.PersonRepository,
	accounts authModels.AccountRepository,
	accountTenants authModels.AccountTenantRepository,
	permissionRepo authModels.PermissionRepository,
	roleRepo authModels.RoleRepository,
) *staffMembershipDeps {
	memberships, ok := accountTenants.(membershipTenantQuery)
	if !ok {
		panic(fmt.Sprintf("repository factory: account tenant repository %T must serve staff membership lookups", accountTenants))
	}
	permissionQuery, ok := permissionRepo.(membershipPermissionQuery)
	if !ok {
		panic(fmt.Sprintf("repository factory: permission repository %T must serve batched effective permissions", permissionRepo))
	}
	roleQuery, ok := roleRepo.(membershipRoleQuery)
	if !ok {
		panic(fmt.Sprintf("repository factory: role repository %T must serve staff role lookups", roleRepo))
	}
	return &staffMembershipDeps{
		persons:     persons,
		accounts:    accounts,
		memberships: memberships,
		permissions: permissionQuery,
		roles:       roleQuery,
	}
}

// staffMembershipRepository serves users.StaffRepository.
type staffMembershipRepository struct {
	membership schoolmembership.Capability
	deps       *staffMembershipDeps
}

var _ userModels.StaffRepository = staffMembershipRepository{}

// membershipNotFound rebuilds the not-found error shape the legacy
// repositories returned, so callers keep classifying with both
// errors.Is(err, sql.ErrNoRows) and usersRepo.IsNotFound(err).
func membershipNotFound(op string) error { return usersRepo.NotFoundError(op) }

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
		return usersRepo.WrapError(op, userModels.ErrPersonnelNumberConflict)
	default:
		return usersRepo.WrapError(op, err)
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

func staffFieldsFromLegacy(staff *userModels.Staff) schoolmembership.StaffFields {
	return schoolmembership.StaffFields{
		PersonID:              staff.PersonID,
		StaffNotes:            staff.StaffNotes,
		EmploymentType:        staff.EmploymentType,
		WorkTimeModelID:       staff.WorkTimeModelID,
		PersonnelNumber:       staff.PersonnelNumber,
		RotationAnchorDate:    usersRepo.CalendarDateString(staff.RotationAnchorDate),
		BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut,
	}
}

func applyStaffToLegacy(target *userModels.Staff, value schoolmembership.Staff) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.PersonID = value.PersonID
	target.StaffNotes = value.StaffNotes
	target.EmploymentType = value.EmploymentType
	target.WorkTimeModelID = value.WorkTimeModelID
	target.PersonnelNumber = value.PersonnelNumber
	target.RotationAnchorDate = usersRepo.ParseCalendarDate(value.RotationAnchorDate)
	target.BirthdayDisplayOptOut = value.BirthdayDisplayOptOut
	target.DeletedAt = value.DeletedAt
}

func toLegacyStaff(value schoolmembership.Staff) *userModels.Staff {
	staff := new(userModels.Staff)
	applyStaffToLegacy(staff, value)
	return staff
}

func toLegacyStaffList(values []schoolmembership.Staff) []*userModels.Staff {
	result := make([]*userModels.Staff, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyStaff(value))
	}
	return result
}

// --- CRUD ---

func (r staffMembershipRepository) Create(ctx context.Context, entity *userModels.Staff) error {
	if entity == nil {
		return usersRepo.WrapError("create staff", errors.New("staff cannot be nil"))
	}
	created, err := r.membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: staffFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create staff", err)
	}
	applyStaffToLegacy(entity, created)
	return nil
}

func (r staffMembershipRepository) Update(ctx context.Context, entity *userModels.Staff) error {
	if entity == nil {
		return usersRepo.WrapError("update staff", errors.New("staff cannot be nil"))
	}
	updated, err := r.membership.UpdateStaff(ctx, schoolmembership.UpdateStaff{ID: entity.ID, StaffFields: staffFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("update staff", err)
	}
	applyStaffToLegacy(entity, updated)
	return nil
}

func (r staffMembershipRepository) Delete(ctx context.Context, id any) error {
	staffID, err := membershipID(id)
	if err != nil {
		return usersRepo.WrapError("delete staff", err)
	}
	return membershipError("delete staff", r.membership.DeleteStaff(ctx, staffID))
}

func (r staffMembershipRepository) FindByID(ctx context.Context, id any) (*userModels.Staff, error) {
	staffID, err := membershipID(id)
	if err != nil {
		return nil, usersRepo.WrapError("find staff by id", err)
	}
	value, err := r.membership.FindStaff(ctx, staffID)
	if err != nil {
		return nil, membershipError("find staff by id", err)
	}
	return toLegacyStaff(value), nil
}

func (r staffMembershipRepository) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Staff, error) {
	value, err := r.membership.FindStaffForMutation(ctx, id)
	if err != nil {
		return nil, membershipError("find staff by id for update", err)
	}
	return toLegacyStaff(value), nil
}

func (r staffMembershipRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	value, err := r.membership.FindStaffByPerson(ctx, personID)
	if err != nil {
		return nil, membershipError("find by person ID", err)
	}
	return toLegacyStaff(value), nil
}

// List keeps the legacy equality-filter shape. Only the keys real callers pass
// are supported; anything else is an explicit error instead of a silently
// ignored filter.
func (r staffMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*userModels.Staff, error) {
	filter := schoolmembership.StaffFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		switch field {
		case "id":
			id, err := membershipID(value)
			if err != nil {
				return nil, usersRepo.WrapError("list staff", err)
			}
			filter.IDs = append(filter.IDs, id)
		case "person_id":
			id, err := membershipID(value)
			if err != nil {
				return nil, usersRepo.WrapError("list staff", err)
			}
			filter.PersonIDs = append(filter.PersonIDs, id)
		case "work_time_model_id":
			id, err := membershipID(value)
			if err != nil {
				return nil, usersRepo.WrapError("list staff", err)
			}
			filter.WorkTimeModelID = &id
		default:
			return nil, usersRepo.WrapError("list staff", fmt.Errorf("unsupported staff filter %q", field))
		}
	}
	values, err := r.membership.ListStaff(ctx, filter)
	if err != nil {
		return nil, membershipError("list staff", err)
	}
	return toLegacyStaffList(values), nil
}

// --- person compositions ---

// hydrateStaffPersons attaches the person rows to the staff members. Persons
// that are gone (soft-deleted or missing) leave Person nil, which is what the
// LEFT JOIN produced.
func (r staffMembershipRepository) hydrateStaffPersons(ctx context.Context, members []*userModels.Staff) error {
	if len(members) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(members))
	for _, member := range members {
		if member.PersonID > 0 {
			ids = append(ids, member.PersonID)
		}
	}
	persons, err := r.deps.persons.FindByIDs(ctx, ids)
	if err != nil {
		return err
	}
	for _, member := range members {
		if person, found := persons[member.PersonID]; found {
			member.Person = person
		}
	}
	return nil
}

func (r staffMembershipRepository) ListAllWithPerson(ctx context.Context) ([]*userModels.Staff, error) {
	values, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, membershipError("list all with person", err)
	}
	members := toLegacyStaffList(values)
	if err := r.hydrateStaffPersons(ctx, members); err != nil {
		return nil, usersRepo.WrapError("list all with person", err)
	}
	return members, nil
}

func (r staffMembershipRepository) FindWithPerson(ctx context.Context, id int64) (*userModels.Staff, error) {
	value, err := r.membership.FindStaff(ctx, id)
	if err != nil {
		return nil, membershipError("find with person - staff", err)
	}
	staff := toLegacyStaff(value)
	if staff.PersonID > 0 {
		person, personErr := r.deps.persons.FindByID(ctx, staff.PersonID)
		switch {
		case personErr == nil:
			staff.Person = person
		case usersRepo.IsNotFound(personErr):
			// Person not found is acceptable — staff.Person stays nil.
		default:
			return nil, usersRepo.WrapError("find with person - load person", personErr)
		}
	}
	return staff, nil
}

func (r staffMembershipRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	if len(ids) == 0 {
		return make(map[int64]*userModels.Staff), nil
	}
	values, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids})
	if err != nil {
		return nil, membershipError("find by IDs", err)
	}
	result := make(map[int64]*userModels.Staff, len(values))
	for _, member := range toLegacyStaffList(values) {
		result[member.ID] = member
	}
	return result, nil
}

func (r staffMembershipRepository) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	if len(ids) == 0 {
		return make(map[int64]*userModels.Staff), nil
	}
	values, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: ids})
	if err != nil {
		return nil, membershipError("find with person by IDs", err)
	}
	members := toLegacyStaffList(values)
	if err := r.hydrateStaffPersons(ctx, members); err != nil {
		return nil, usersRepo.WrapError("find with person by IDs", err)
	}
	result := make(map[int64]*userModels.Staff, len(members))
	for _, member := range members {
		result[member.ID] = member
	}
	return result, nil
}

// staffAccountLink is one resolved staff → person → account walk.
type staffAccountLink struct {
	staff     schoolmembership.Staff
	person    *userModels.Person
	accountID int64
}

// resolveStaffAccounts walks live staff members to their live person and the
// account the person is linked to. Staff without a live person or without an
// account are dropped, exactly as the INNER JOINs did.
func (r staffMembershipRepository) resolveStaffAccounts(ctx context.Context, members []schoolmembership.Staff) ([]staffAccountLink, error) {
	if len(members) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(members))
	for _, member := range members {
		personIDs = append(personIDs, member.PersonID)
	}
	persons, err := r.deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	links := make([]staffAccountLink, 0, len(members))
	for _, member := range members {
		person, found := persons[member.PersonID]
		if !found || person.AccountID == nil {
			continue
		}
		links = append(links, staffAccountLink{staff: member, person: person, accountID: *person.AccountID})
	}
	return links, nil
}

// activeAccountsByTenant narrows the links to accounts that can act at the
// tenant of their own staff row (the legacy join condition was
// account_tenant.tenant_id = staff.tenant_id, not the request tenant).
func (r staffMembershipRepository) activeAccountsByTenant(ctx context.Context, links []staffAccountLink) (map[int64]bool, error) {
	byTenant := make(map[int64][]int64)
	for _, link := range links {
		byTenant[link.staff.TenantID] = append(byTenant[link.staff.TenantID], link.accountID)
	}
	active := make(map[int64]bool, len(links))
	for tenantID, accountIDs := range byTenant {
		allowed, err := r.deps.memberships.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
		if err != nil {
			return nil, err
		}
		for _, accountID := range allowed {
			active[accountID] = true
		}
	}
	return active, nil
}

func (r staffMembershipRepository) accountIDsByStaff(ctx context.Context, filter schoolmembership.StaffFilter, op string) (map[int64]int64, error) {
	values, err := r.membership.ListStaff(ctx, filter)
	if err != nil {
		return nil, membershipError(op, err)
	}
	links, err := r.resolveStaffAccounts(ctx, values)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	active, err := r.activeAccountsByTenant(ctx, links)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	result := make(map[int64]int64, len(links))
	for _, link := range links {
		if active[link.accountID] {
			result[link.staff.ID] = link.accountID
		}
	}
	return result, nil
}

func (r staffMembershipRepository) ListAccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error) {
	if len(staffIDs) == 0 {
		return map[int64]int64{}, nil
	}
	return r.accountIDsByStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs}, "list account IDs by staff IDs")
}

func (r staffMembershipRepository) ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error) {
	return r.accountIDsByStaff(ctx, schoolmembership.StaffFilter{}, "list all staff account IDs")
}

// permittedStaffLinks resolves the live staff of the request tenant whose
// account is active there, together with their effective permission names.
func (r staffMembershipRepository) permittedStaffLinks(ctx context.Context, filter schoolmembership.StaffFilter, op string) ([]staffAccountLink, map[int64][]string, error) {
	tenantID := usersRepo.TenantIDFromContext(ctx)
	values, err := r.membership.ListStaff(ctx, filter)
	if err != nil {
		return nil, nil, membershipError(op, err)
	}
	links, err := r.resolveStaffAccounts(ctx, values)
	if err != nil {
		return nil, nil, usersRepo.WrapError(op, err)
	}
	accountIDs := make([]int64, 0, len(links))
	for _, link := range links {
		accountIDs = append(accountIDs, link.accountID)
	}
	allowed, err := r.deps.memberships.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
	if err != nil {
		return nil, nil, usersRepo.WrapError(op, err)
	}
	active := make(map[int64]bool, len(allowed))
	for _, accountID := range allowed {
		active[accountID] = true
	}
	reachable := make([]staffAccountLink, 0, len(links))
	for _, link := range links {
		if active[link.accountID] {
			reachable = append(reachable, link)
		}
	}
	names, err := r.deps.permissions.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, allowed, tenantID)
	if err != nil {
		return nil, nil, usersRepo.WrapError(op, err)
	}
	return reachable, names, nil
}

func (r staffMembershipRepository) ListStaffWithPermission(ctx context.Context, permissionName string) ([]*userModels.StaffWithRoleInfo, error) {
	const op = "list staff with permission"
	links, names, err := r.permittedStaffLinks(ctx, schoolmembership.StaffFilter{}, op)
	if err != nil {
		return nil, err
	}
	matched := make([]staffAccountLink, 0, len(links))
	accountIDs := make([]int64, 0, len(links))
	for _, link := range links {
		if usersRepo.HasEffectivePermission(permissionName, names[link.accountID]) {
			matched = append(matched, link)
			accountIDs = append(accountIDs, link.accountID)
		}
	}
	emails, err := r.deps.accounts.FindEmailsByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	result := make([]*userModels.StaffWithRoleInfo, 0, len(matched))
	for _, link := range matched {
		result = append(result, &userModels.StaffWithRoleInfo{
			StaffID:   link.staff.ID,
			PersonID:  link.person.ID,
			FirstName: link.person.FirstName,
			LastName:  link.person.LastName,
			AccountID: link.accountID,
			Email:     emails[link.accountID],
		})
	}
	return result, nil
}

func (r staffMembershipRepository) FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	const op = "find reachable calendar staff"
	links, names, err := r.permittedStaffLinks(ctx, schoolmembership.StaffFilter{IDs: ids}, op)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(links))
	for _, link := range links {
		if usersRepo.HasEffectivePermission(usersRepo.CalendarOwnPermission, names[link.accountID]) {
			result[link.staff.ID] = true
		}
	}
	return result, nil
}

func (r staffMembershipRepository) GetStaffContactInfo(ctx context.Context, staffID int64) (*userModels.StaffWithRoleInfo, error) {
	const op = "get staff contact info"
	value, err := r.membership.FindStaff(ctx, staffID)
	if err != nil {
		return nil, membershipError(op, err)
	}
	person, err := r.deps.persons.FindByID(ctx, value.PersonID)
	if err != nil {
		if usersRepo.IsNotFound(err) {
			return nil, membershipNotFound(op)
		}
		return nil, usersRepo.WrapError(op, err)
	}
	if person.AccountID == nil {
		return nil, membershipNotFound(op)
	}
	emails, err := r.deps.accounts.FindEmailsByAccountIDs(ctx, []int64{*person.AccountID})
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	email, found := emails[*person.AccountID]
	if !found {
		return nil, membershipNotFound(op)
	}
	return &userModels.StaffWithRoleInfo{
		StaffID:   value.ID,
		PersonID:  person.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		AccountID: *person.AccountID,
		Email:     email,
	}, nil
}

// ListStaffByRoles mirrors the legacy join exactly, including two properties
// that read like bugs but are the shipped wire shape: it does NOT check
// auth.accounts.active or the tenant of the role assignment, and it emits one
// entry per matching role assignment rather than one per staff member.
func (r staffMembershipRepository) ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error) {
	const op = "list staff by roles"
	values, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, membershipError(op, err)
	}
	links, err := r.resolveStaffAccounts(ctx, values)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	accountIDs := make([]int64, 0, len(links))
	for _, link := range links {
		accountIDs = append(accountIDs, link.accountID)
	}
	matches, err := r.deps.roles.CountRoleNameMatchesByAccountIDs(ctx, accountIDs, roles)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	emails, err := r.deps.accounts.FindEmailsByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	result := make([]*userModels.StaffWithRoleInfo, 0, len(links))
	for _, link := range links {
		email, hasAccount := emails[link.accountID]
		if !hasAccount {
			continue
		}
		for range matches[link.accountID] {
			result = append(result, &userModels.StaffWithRoleInfo{
				StaffID:   link.staff.ID,
				CreatedAt: link.staff.CreatedAt,
				UpdatedAt: link.staff.UpdatedAt,
				PersonID:  link.person.ID,
				FirstName: link.person.FirstName,
				LastName:  link.person.LastName,
				AccountID: link.accountID,
				Email:     email,
			})
		}
	}
	return result, nil
}

// --- birthdays ---

// staffBirthdays projects live staff with a stored birth date onto the shared
// birthday entry. days nil means "every stored birthday" (the export);
// otherwise only the annually recurring days are kept. The opt-out filter is
// applied by the caller, not here, because the export deliberately ignores it.
func (r staffMembershipRepository) staffBirthdays(ctx context.Context, days []userModels.MonthDay, applyOptOut bool, op string) ([]userModels.BirthdayEntry, error) {
	values, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, membershipError(op, err)
	}
	members := make([]schoolmembership.Staff, 0, len(values))
	personIDs := make([]int64, 0, len(values))
	for _, value := range values {
		if applyOptOut && value.BirthdayDisplayOptOut {
			continue
		}
		members = append(members, value)
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := r.deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	wanted := make(map[userModels.MonthDay]bool, len(days))
	for _, day := range days {
		wanted[day] = true
	}
	entries := make([]userModels.BirthdayEntry, 0, len(members))
	for _, member := range members {
		person, found := persons[member.PersonID]
		if !found || person.Birthday == nil || person.Birthday.IsZero() {
			continue
		}
		if days != nil && !wanted[userModels.MonthDayOf(*person.Birthday)] {
			continue
		}
		entries = append(entries, userModels.BirthdayEntry{
			Kind:      userModels.BirthdayKindStaff,
			ID:        member.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
			Birthday:  *person.Birthday,
		})
	}
	return entries, nil
}

func (r staffMembershipRepository) FindBirthdaysOn(ctx context.Context, days []userModels.MonthDay) ([]userModels.BirthdayEntry, error) {
	if len(days) == 0 {
		return nil, nil
	}
	return r.staffBirthdays(ctx, days, true, "find staff birthdays")
}

func (r staffMembershipRepository) ListBirthdaysForExport(ctx context.Context) ([]userModels.BirthdayEntry, error) {
	return r.staffBirthdays(ctx, nil, false, "list staff birthdays for export")
}

func (r staffMembershipRepository) SetBirthdayDisplayOptOut(ctx context.Context, staffID int64, optOut bool) error {
	return membershipError("set staff birthday display opt-out", r.membership.SetBirthdayDisplayOptOut(ctx, staffID, optOut))
}

func (r staffMembershipRepository) ClearWorkTimeModel(ctx context.Context, id int64) error {
	return membershipError("clear work time model", r.membership.ClearWorkTimeModel(ctx, id))
}

func (r staffMembershipRepository) AddNotes(ctx context.Context, id int64, notes string) error {
	_, err := r.membership.AppendStaffNotes(ctx, id, notes)
	return membershipError("add notes", err)
}
