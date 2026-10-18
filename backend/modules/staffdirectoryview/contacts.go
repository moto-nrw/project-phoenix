package staffdirectoryview

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// StaffContact returns the staff member with person, account and e-mail
// address. A member without a live person, an account or an address is
// ErrStaffContactNotFound.
func (d *Directory) StaffContact(ctx context.Context, staffID int64) (StaffContact, error) {
	value, err := d.deps.Members.FindStaff(ctx, staffID)
	if err != nil {
		return StaffContact{}, &MembershipError{Err: err}
	}
	person, err := d.deps.Persons.Person(ctx, value.PersonID)
	if err != nil {
		if errors.Is(err, ErrPersonNotFound) {
			return StaffContact{}, ErrStaffContactNotFound
		}
		return StaffContact{}, err
	}
	if person.AccountID == nil {
		return StaffContact{}, ErrStaffContactNotFound
	}
	emails, err := d.deps.Accounts.ListAccountEmails(ctx, []int64{*person.AccountID})
	if err != nil {
		return StaffContact{}, err
	}
	email, found := emails[*person.AccountID]
	if !found {
		return StaffContact{}, ErrStaffContactNotFound
	}
	return StaffContact{
		StaffID: value.ID, PersonID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
		AccountID: *person.AccountID, Email: email,
	}, nil
}

// StaffByRoles mirrors the legacy join exactly, including two properties
// that read like bugs but are the shipped wire shape: it does NOT check
// whether the account is active or the tenant of the role assignment, and it
// emits one entry per matching role assignment rather than one per staff
// member.
func (d *Directory) StaffByRoles(ctx context.Context, roles []string) ([]StaffContact, error) {
	values, err := d.listStaff(ctx, schoolmembership.StaffFilter{MembershipOnly: true})
	if err != nil {
		return nil, err
	}
	links, err := d.resolveStaffAccounts(ctx, values)
	if err != nil {
		return nil, err
	}
	accountIDs := make([]int64, 0, len(links))
	for _, link := range links {
		accountIDs = append(accountIDs, link.accountID)
	}
	matches, err := d.deps.Accounts.CountRoleNameMatchesByAccountIDs(ctx, accountIDs, roles)
	if err != nil {
		return nil, err
	}
	emails, err := d.deps.Accounts.ListAccountEmails(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	result := make([]StaffContact, 0, len(links))
	for _, link := range links {
		email, hasAccount := emails[link.accountID]
		if !hasAccount {
			continue
		}
		for range matches[link.accountID] {
			result = append(result, StaffContact{
				StaffID: link.staff.ID, CreatedAt: link.staff.CreatedAt, UpdatedAt: link.staff.UpdatedAt,
				PersonID: link.person.ID, FirstName: link.person.FirstName, LastName: link.person.LastName,
				AccountID: link.accountID, Email: email,
			})
		}
	}
	return result, nil
}
