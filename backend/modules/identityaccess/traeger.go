package identityaccess

import (
	"context"
	"errors"
	"slices"
	"strings"
)

// Träger-Portal sign-in and Träger roles (ADR 0084, #4649). A person has one
// account; the Träger-Portal is a separate access to it with its own session
// (scope org), which names the Träger and no school. The Träger membership
// belongs to Organisation & Tenancy; the roles and their rights belong here.

var (
	// ErrNoTraegerMembership reports a correct password without an active
	// membership at an active Träger, or a Träger choice outside them.
	ErrNoTraegerMembership = errors.New("account has no active Träger membership")
	// ErrInvalidTraegerRole reports a role name outside the Träger roles.
	ErrInvalidTraegerRole = errors.New("invalid Träger role")
	// ErrTraegerTransactionRequired reports a participating Träger command
	// called without the caller's transaction.
	ErrTraegerTransactionRequired = errors.New("the Träger role command requires the caller's transaction")
)

// The Träger roles of ADR 0084 section 4. In version 1 each is a fixed bundle
// of Träger rights; a person may hold several. They are separate from school
// roles and reach no school permission.
const (
	TraegerRoleAdministration = "administration"
	TraegerRoleBilling        = "billing"
	TraegerRolePersonnel      = "personnel"
	TraegerRoleReadOnly       = "read_only"
)

// The Träger rights of ADR 0084 section 4. They travel only in Träger-Portal
// tokens (scope org); auth.permissions does not hold them and no school role
// grants them. Routes check the right, never the role name.
const (
	TraegerRightOverviewRead     = "traeger_overview:read"
	TraegerRightEnrollmentManage = "traeger_enrollment:manage"
	TraegerRightContractsManage  = "traeger_contracts:manage"
	TraegerRightBillingManage    = "traeger_billing:manage"
	TraegerRightFinanceExport    = "traeger_finance:export"
	TraegerRightStaffManage      = "traeger_staff:manage"
	TraegerRightReportsRead      = "traeger_reports:read"
	TraegerRightSettingsManage   = "traeger_settings:manage"
	TraegerRightAccountsManage   = "traeger_accounts:manage"
	TraegerRightAccessLogRead    = "traeger_access_log:read"
	TraegerRightSchoolSwitch     = "traeger_school:switch"
)

// traegerRoleRights is the role table of ADR 0084 section 4.
var traegerRoleRights = map[string][]string{
	TraegerRoleAdministration: {
		TraegerRightOverviewRead, TraegerRightEnrollmentManage, TraegerRightContractsManage, TraegerRightReportsRead,
		TraegerRightSettingsManage, TraegerRightAccountsManage, TraegerRightAccessLogRead, TraegerRightSchoolSwitch,
	},
	TraegerRoleBilling:   {TraegerRightOverviewRead, TraegerRightBillingManage, TraegerRightFinanceExport, TraegerRightReportsRead},
	TraegerRolePersonnel: {TraegerRightOverviewRead, TraegerRightStaffManage, TraegerRightReportsRead},
	TraegerRoleReadOnly:  {TraegerRightOverviewRead, TraegerRightReportsRead},
}

// traegerRoleLabels are the German names the Träger-Portal shows.
var traegerRoleLabels = map[string]string{
	TraegerRoleAdministration: "Verwaltung",
	TraegerRoleBilling:        "Abrechnung",
	TraegerRolePersonnel:      "Personal",
	TraegerRoleReadOnly:       "Nur lesen",
}

// TraegerRoleLabels names the roles in German, joined like a sentence:
// „Verwaltung und Abrechnung“. Unknown role names are left out.
func TraegerRoleLabels(roles []string) string {
	labels := make([]string, 0, len(roles))
	for _, role := range roles {
		if label, ok := traegerRoleLabels[role]; ok {
			labels = append(labels, label)
		}
	}
	if len(labels) <= 1 {
		return strings.Join(labels, "")
	}
	return strings.Join(labels[:len(labels)-1], ", ") + " und " + labels[len(labels)-1]
}

// IsTraegerRole reports whether role names a Träger role.
func IsTraegerRole(role string) bool {
	_, ok := traegerRoleRights[role]
	return ok
}

// TraegerRights is the sorted union of the rights of the given roles. Unknown
// role names grant nothing.
func TraegerRights(roles []string) []string {
	rights := []string{}
	for _, role := range roles {
		for _, right := range traegerRoleRights[role] {
			if !slices.Contains(rights, right) {
				rights = append(rights, right)
			}
		}
	}
	slices.Sort(rights)
	return rights
}

// TraegerOrganization is a Träger an account may sign in to.
type TraegerOrganization struct {
	ID   int64
	Name string
}

// TraegerLoginStatus discriminates the Träger login answers.
type TraegerLoginStatus string

const (
	// TraegerLoginAuthenticated carries a Träger session.
	TraegerLoginAuthenticated TraegerLoginStatus = "authenticated"
	// TraegerLoginSelectionRequired lists the Träger of a member of several;
	// the login is repeated with one of them.
	TraegerLoginSelectionRequired TraegerLoginStatus = "organization_selection_required"
)

// TraegerLoginResult is the Träger login answer: a token pair, or the Träger
// to choose from.
type TraegerLoginResult struct {
	Status        TraegerLoginStatus
	AccessToken   string
	RefreshToken  string
	Organizations []TraegerOrganization
}

// TraegerAccountLogin signs an account in to the Träger-Portal. A zero
// organizationID picks the only Träger of the account or asks for a choice.
type TraegerAccountLogin interface {
	LoginTraeger(ctx context.Context, email, password, ipAddress, userAgent string, organizationID int64) (*TraegerLoginResult, error)
}

// TraegerAccount is an account at a Träger with its Träger roles.
type TraegerAccount struct {
	AccountID int64
	Email     string
	Roles     []string
}

// TraegerRoles grants and lists Träger roles. Both join the caller's
// administrative transaction: the Träger membership command of Organisation &
// Tenancy owns the unit of work (ADR 0067, command participation).
type TraegerRoles interface {
	AssignTraegerRole(ctx context.Context, organizationID, accountID int64, role string) error
	ListTraegerAccounts(ctx context.Context, organizationID int64, accountIDs []int64) ([]TraegerAccount, error)
}
