package staffdirectoryview

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// staffAccountLink is one resolved staff → person → account walk.
type staffAccountLink struct {
	staff     schoolmembership.Staff
	person    Person
	accountID int64
}

type tenantAccountID struct {
	tenantID  int64
	accountID int64
}

// resolveStaffAccounts walks live staff members to their live person and the
// account the person is linked to. Staff without a live person or without an
// account are dropped, exactly as the INNER JOINs did.
func (d *Directory) resolveStaffAccounts(ctx context.Context, members []schoolmembership.Staff) ([]staffAccountLink, error) {
	if len(members) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(members))
	for _, member := range members {
		personIDs = append(personIDs, member.PersonID)
	}
	persons, err := d.deps.Persons.PersonsByID(ctx, personIDs)
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
func (d *Directory) activeAccountsByTenant(ctx context.Context, links []staffAccountLink) (map[tenantAccountID]bool, error) {
	byTenant := make(map[int64][]int64)
	for _, link := range links {
		byTenant[link.staff.TenantID] = append(byTenant[link.staff.TenantID], link.accountID)
	}
	active := make(map[tenantAccountID]bool, len(links))
	for tenantID, accountIDs := range byTenant {
		allowed, err := d.deps.Accounts.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
		if err != nil {
			return nil, err
		}
		for _, accountID := range allowed {
			active[tenantAccountID{tenantID: tenantID, accountID: accountID}] = true
		}
	}
	return active, nil
}

// AccountIDsByStaffIDs maps the given live staff members to the account
// that can act at their own school. No staff member maps to nothing.
func (d *Directory) AccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error) {
	if len(staffIDs) == 0 {
		return map[int64]int64{}, nil
	}
	return d.accountIDsByStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs})
}

// AllStaffAccountIDs maps every live staff member to the account that can
// act at their own school.
func (d *Directory) AllStaffAccountIDs(ctx context.Context) (map[int64]int64, error) {
	return d.accountIDsByStaff(ctx, schoolmembership.StaffFilter{})
}

func (d *Directory) accountIDsByStaff(ctx context.Context, filter schoolmembership.StaffFilter) (map[int64]int64, error) {
	filter.MembershipOnly = true // account links read no employment field
	values, err := d.listStaff(ctx, filter)
	if err != nil {
		return nil, err
	}
	links, err := d.resolveStaffAccounts(ctx, values)
	if err != nil {
		return nil, err
	}
	active, err := d.activeAccountsByTenant(ctx, links)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]int64, len(links))
	for _, link := range links {
		if active[tenantAccountID{tenantID: link.staff.TenantID, accountID: link.accountID}] {
			result[link.staff.ID] = link.accountID
		}
	}
	return result, nil
}

// permittedStaffLinks resolves the live staff of the request tenant whose
// account is active there, together with their effective permission names.
func (d *Directory) permittedStaffLinks(ctx context.Context, filter schoolmembership.StaffFilter) ([]staffAccountLink, map[int64][]string, error) {
	tenantID := d.deps.CallerTenant(ctx)
	filter.MembershipOnly = true // permission links read no employment field
	values, err := d.listStaff(ctx, filter)
	if err != nil {
		return nil, nil, err
	}
	links, err := d.resolveStaffAccounts(ctx, values)
	if err != nil {
		return nil, nil, err
	}
	accountIDs := make([]int64, 0, len(links))
	for _, link := range links {
		accountIDs = append(accountIDs, link.accountID)
	}
	allowed, err := d.deps.Accounts.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
	if err != nil {
		return nil, nil, err
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
	names, err := d.deps.Accounts.FindEffectivePermissionNamesByAccountIDsForTenant(ctx, allowed, tenantID)
	if err != nil {
		return nil, nil, err
	}
	return reachable, names, nil
}

// StaffWithPermission lists the live staff of the request's school whose
// active account holds permission there, with their e-mail address.
func (d *Directory) StaffWithPermission(ctx context.Context, permission string) ([]StaffContact, error) {
	links, names, err := d.permittedStaffLinks(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, err
	}
	matched := make([]staffAccountLink, 0, len(links))
	accountIDs := make([]int64, 0, len(links))
	for _, link := range links {
		if d.deps.HasPermission(permission, names[link.accountID]) {
			matched = append(matched, link)
			accountIDs = append(accountIDs, link.accountID)
		}
	}
	emails, err := d.deps.Accounts.ListAccountEmails(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	result := make([]StaffContact, 0, len(matched))
	for _, link := range matched {
		result = append(result, StaffContact{
			StaffID: link.staff.ID, PersonID: link.person.ID,
			FirstName: link.person.FirstName, LastName: link.person.LastName,
			AccountID: link.accountID, Email: emails[link.accountID],
		})
	}
	return result, nil
}

// ReachableStaffIDs keeps the selected staff members whose active account
// at the request's school holds permission.
func (d *Directory) ReachableStaffIDs(ctx context.Context, ids []int64, permission string) (map[int64]bool, error) {
	links, names, err := d.permittedStaffLinks(ctx, schoolmembership.StaffFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(links))
	for _, link := range links {
		if d.deps.HasPermission(permission, names[link.accountID]) {
			result[link.staff.ID] = true
		}
	}
	return result, nil
}
