package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// MessageableStaffRepository answers who is a colleague at the current school
// and which side of the school they sit on: the staff messaging recipient
// picker, its authorization predicate and the role kind shown next to a name.
//
// It stays with People Directory because the rows it reads are person rows;
// School Membership and Identity & Access supply the facts it filters
// through. Communication owns the conversations, cursors and the inbox
// projection (#3221) and receives this lookup as a colleague directory.
type MessageableStaffRepository struct {
	db *bun.DB
	// staffAccounts resolves the login accounts of the school's live staff.
	// School Membership owns those rows, so the relation that used to be a
	// join is injected as a lookup; without one the repository fails closed.
	staffAccounts StaffAccountsFunc
	// identity holds the Identity & Access owner queries: the global account
	// switch (#2720), the school mapping and the role classification (#2721).
	identity StaffMessageIdentity
}

// StaffAccountsFunc returns the login accounts of the live staff members of
// the tenant in ctx. It is a plain function type so this package does not have
// to depend on the School Membership owner to state what it needs.
type StaffAccountsFunc func(ctx context.Context) ([]int64, error)

// StaffMessageIdentity are the Identity & Access owner queries the colleague
// lookup filters through instead of naming auth tables.
type StaffMessageIdentity struct {
	// ActiveSchoolAccounts narrows candidates to globally active accounts with
	// an active membership at the supplied school, in the ambient transaction.
	ActiveSchoolAccounts func(context.Context, int64, []int64) ([]int64, error)
	// RoleClasses classifies the roles accounts hold at a school.
	RoleClasses SchoolRoleClassQuery
}

// NewMessageableStaffRepository wires the colleague lookup.
//
// The "is a colleague at this school" relation needs the staff rows School
// Membership owns and the account facts Identity & Access owns, so the
// caller injects both.
func NewMessageableStaffRepository(db *bun.DB, staffAccounts StaffAccountsFunc, identity StaffMessageIdentity) *MessageableStaffRepository {
	return &MessageableStaffRepository{db: db, staffAccounts: staffAccounts, identity: identity}
}

// resolveStaffAccounts fails closed: without a resolver nobody is a colleague,
// so a misconfigured graph cannot widen who may be written to.
func (r *MessageableStaffRepository) resolveStaffAccounts(ctx context.Context) ([]int64, error) {
	if r.staffAccounts == nil {
		return nil, &modelBase.DatabaseError{Op: "resolve staff accounts", Err: errors.New("staff account resolver is required")}
	}
	return r.staffAccounts(ctx)
}

// colleagueQuery applies the "this account belongs to a colleague at this
// school" relation, shared verbatim by ListMessageableStaff and
// IsMessageableStaff.
//
// It lives in ONE place because the two MUST agree: the picker decides what a
// user is offered, the predicate decides what the API accepts, and any drift
// between them is an authorization hole reachable by hand-crafting a request.
//
// Four facts, because each answers a different question and the account
// lifecycle has two independent switches:
//   - the ACTIVE school mapping says "may act at this school". Identity &
//     Access owns it, so the owner's bounded account lookup checks it;
//   - users.persons is NOT enough: it also holds children and guests, who can
//     carry an account and an active tenant mapping;
//   - staff membership says "colleague at this school" — the caller passes the
//     accounts of the school's live staff, resolved through the School
//     Membership owner, and staffAccountFilter turns that into the predicate
//     the users.staff join used to be;
//   - auth.accounts.active is the GLOBAL switch. Account management
//     deactivates an account there WITHOUT touching the school mapping, so a
//     per-tenant check alone still lets a globally disabled account be
//     addressed and keep writing; the same owner lookup checks that switch.
//
// It fails closed like resolveStaffAccounts: a graph without the owner
// queries addresses nobody.
func (r *MessageableStaffRepository) colleagueQuery(ctx context.Context, query *bun.SelectQuery) (*bun.SelectQuery, error) {
	if r.identity.ActiveSchoolAccounts == nil {
		return nil, &modelBase.DatabaseError{Op: "resolve colleague relation", Err: errors.New("active school account lookup is required")}
	}
	staffAccountIDs, err := r.resolveStaffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	activeAccountIDs, err := r.identity.ActiveSchoolAccounts(ctx, tenant.FromContext(ctx), staffAccountIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve active staff accounts: %w", err)
	}
	query = query.Where(`person.deleted_at IS NULL`).
		Where(`person.tenant_id = ?`, tenant.FromContext(ctx))
	return staffAccountFilter(query, activeAccountIDs), nil
}

// staffAccountFilter narrows the colleague relation to the accounts of the
// school's live staff. An empty set matches nothing, which is what the
// dropped INNER JOIN did.
func staffAccountFilter(query *bun.SelectQuery, staffAccountIDs []int64) *bun.SelectQuery {
	if len(staffAccountIDs) == 0 {
		return query.Where("1 = 0")
	}
	return query.Where(`person.account_id IN (?)`, bun.List(staffAccountIDs))
}

// ListMessageableStaff returns the accounts the viewer may write to: people
// with an ACTIVE mapping to the current tenant, excluding the viewer.
//
// "Active" is the whole access rule for V1 — a colleague whose mapping went
// inactive (left the school) disappears from the picker and can no longer be
// addressed, while the existing conversation history stays readable.
func (r *MessageableStaffRepository) ListMessageableStaff(ctx context.Context, viewerAccountID int64) ([]*users.MessageableStaff, error) {
	var rows []*users.MessageableStaff
	query, err := r.colleagueQuery(ctx, base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.persons AS "person"`).
		ColumnExpr(`person.account_id AS account_id`).
		ColumnExpr(`COALESCE(NULLIF(btrim(COALESCE(person.first_name, '') || ' ' || COALESCE(person.last_name, '')), ''), 'Unbekannt') AS name`).
		Where(`person.account_id <> ?`, viewerAccountID).
		OrderExpr(`name ASC`))
	if err != nil {
		return nil, err
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list messageable staff", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}

// IsMessageableStaff reports whether the account may take part in an internal
// conversation at the current school. Authorization predicate for opening one.
//
// It MUST stay the exact predicate ListMessageableStaff filters the picker by,
// which is why both use colleagueQuery. Two things an active school mapping
// does NOT prove:
//   - a guardian who accepts an invitation gets exactly that row
//     (services/auth/guardian_invitation_service.go) and no persons row;
//   - a child or guest CAN carry a person row and an account, so the persons
//     join alone still admits them.
//
// Only the users.staff relation makes someone a colleague.
//
// The method is deliberately not called IsActiveTenantMember any more: that name
// described the query, not the question, and invited exactly this gap.
func (r *MessageableStaffRepository) IsMessageableStaff(ctx context.Context, accountID int64) (bool, error) {
	query, err := r.colleagueQuery(ctx, base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`users.persons AS "person"`).
		ColumnExpr(`1`).
		Where(`person.account_id = ?`, accountID).
		Limit(1))
	if err != nil {
		return false, err
	}
	exists, existsErr := query.Exists(ctx)
	err = existsErr
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check messageable staff", Err: base.TranslateNotFound(err)}
	}
	return exists, nil
}

// StaffRoleKinds classifies accounts by their roles at the current tenant.
//
// The Identity & Access owner decides which roles count: "admin" is the
// system admin role itself or any custom role whose base_role is admin,
// "lehrkraft" the platform system role of that name (narrowed to system roles
// exactly like identityaccess.IsLehrkraftSystemRole). Precedence admin >
// lehrkraft > staff, see the StaffRoleKind constants.
func (r *MessageableStaffRepository) StaffRoleKinds(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(accountIDs))
	for _, id := range accountIDs {
		out[id] = users.StaffRoleKindStaff
	}
	if len(accountIDs) == 0 {
		return out, nil
	}
	if r.identity.RoleClasses == nil {
		return nil, &modelBase.DatabaseError{Op: "resolve staff role kinds", Err: errors.New("role class query is required")}
	}

	rows, err := r.identity.RoleClasses(ctx, tenant.FromContext(ctx), accountIDs)
	if err != nil {
		return nil, fmt.Errorf("resolve staff role kinds: %w", err)
	}
	for _, row := range rows {
		switch {
		case row.IsAdmin:
			out[row.AccountID] = users.StaffRoleKindAdmin
		case row.IsLehrkraft:
			out[row.AccountID] = users.StaffRoleKindLehrkraft
		}
	}
	return out, nil
}
