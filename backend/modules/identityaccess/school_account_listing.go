package identityaccess

import (
	"context"
	"slices"
	"strings"
)

// SchoolAccountListing contains identity-owned facts for operator account lists.
// AccountID zero with status "invited" denotes a pending invitation; names
// belong to that invitation. Person names and caregiver facts belong to other owners.
type SchoolAccountListing struct {
	SchoolID     int64
	AccountID    int64
	Email        string
	Active       bool
	FirstName    string
	LastName     string
	RoleName     string
	Status       string
	HasAdminRole bool
	HasUserRole  bool
}

// SchoolAccountListingStatusInvited is the status of a pending invitation's
// row.
const SchoolAccountListingStatusInvited = "invited"

// PendingInvitation reports the row of an invitation no account has accepted
// yet: it carries no account and lists after the accounts.
func (l SchoolAccountListing) PendingInvitation() bool {
	return l.AccountID == 0 && l.Status == SchoolAccountListingStatusInvited
}

// SchoolAccountListings reads only the explicit school set. The caller owns
// school visibility; the query preserves its ambient transaction and RLS.
type SchoolAccountListings interface {
	ListSchoolAccountListings(context.Context, []int64) ([]SchoolAccountListing, error)
}

func (m *Module) ListSchoolAccountListings(ctx context.Context, schoolIDs []int64) ([]SchoolAccountListing, error) {
	return m.engine.ListSchoolAccountListings(ctx, schoolIDs)
}

// ListedAccountKey is what the operator account listing orders a row by:
// the school it belongs to, whether a person backs the account there, and
// the names the row shows.
type ListedAccountKey struct {
	SchoolID   int64
	SchoolName string
	HasPerson  bool
	LastName   string
	FirstName  string
}

// CompareSchoolAccounts orders the account rows of one school by last and
// then first name, rows without a person last, as the earlier SQL order
// (NULLs last) did. Pending invitations take no part in it: they keep their
// place after the accounts.
func CompareSchoolAccounts(left, right ListedAccountKey) int {
	if left.HasPerson != right.HasPerson {
		if left.HasPerson {
			return -1
		}
		return 1
	}
	if order := strings.Compare(left.LastName, right.LastName); order != 0 {
		return order
	}
	return strings.Compare(left.FirstName, right.FirstName)
}

// CompareAccountsBySchoolID groups the account rows of several schools by
// school ID and orders each school's rows by CompareSchoolAccounts.
func CompareAccountsBySchoolID(left, right ListedAccountKey) int {
	if left.SchoolID != right.SchoolID {
		if left.SchoolID < right.SchoolID {
			return -1
		}
		return 1
	}
	return CompareSchoolAccounts(left, right)
}

// CompareAccountsBySchoolName is the final order of a listing across schools:
// school name, then last and first name. A stable sort keeps the earlier
// order among rows it ranks equal.
func CompareAccountsBySchoolName(left, right ListedAccountKey) int {
	if order := strings.Compare(left.SchoolName, right.SchoolName); order != 0 {
		return order
	}
	if order := strings.Compare(left.LastName, right.LastName); order != 0 {
		return order
	}
	return strings.Compare(left.FirstName, right.FirstName)
}

// OrderSchoolAccountListing is the order of one school's operator account
// listing: the account rows by CompareSchoolAccounts, then the pending
// invitations in their read order. enrich completes the account rows (person
// names and facts) before they are ordered; pending invitations take no part
// in it. key maps a row onto the fields the order compares.
func OrderSchoolAccountListing[T any](rows []T, pending func(T) bool, enrich func([]T) error, key func(T) ListedAccountKey) ([]T, error) {
	return orderAccountListing(rows, pending, enrich, key, CompareSchoolAccounts)
}

// OrderAccountListingBySchoolID is OrderSchoolAccountListing across several
// schools: the account rows by CompareAccountsBySchoolID, then the pending
// invitations in their read order.
func OrderAccountListingBySchoolID[T any](rows []T, pending func(T) bool, enrich func([]T) error, key func(T) ListedAccountKey) ([]T, error) {
	return orderAccountListing(rows, pending, enrich, key, CompareAccountsBySchoolID)
}

func orderAccountListing[T any](rows []T, pending func(T) bool, enrich func([]T) error, key func(T) ListedAccountKey, compare func(left, right ListedAccountKey) int) ([]T, error) {
	accounts := make([]T, 0, len(rows))
	invitations := make([]T, 0)
	for _, row := range rows {
		if pending(row) {
			invitations = append(invitations, row)
			continue
		}
		accounts = append(accounts, row)
	}
	if err := enrich(accounts); err != nil {
		return nil, err
	}
	slices.SortStableFunc(accounts, func(left, right T) int {
		return compare(key(left), key(right))
	})
	return append(accounts, invitations...), nil
}

// PersonStaffRecord is the school of one staff record of a person.
type PersonStaffRecord struct {
	PersonID int64
	SchoolID int64
}

// StaffSchoolByPerson is the school of each person's staff record. When a
// read lists several records for one person, the first one counts.
func StaffSchoolByPerson(records []PersonStaffRecord) map[int64]int64 {
	schools := make(map[int64]int64, len(records))
	for _, record := range records {
		if _, found := schools[record.PersonID]; !found {
			schools[record.PersonID] = record.SchoolID
		}
	}
	return schools
}

// StaffRecordCountsAtSchool reports whether a person's staff record backs the
// account at school: a found record counts only at the school it belongs
// to, so a record of another school never lends the account staff or
// caregiver facts here.
func StaffRecordCountsAtSchool(found bool, recordSchoolID, schoolID int64) bool {
	return found && recordSchoolID == schoolID
}
