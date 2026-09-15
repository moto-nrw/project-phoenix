// Package parent contains repositories for the cross-tenant
// guardian-portal endpoints. Every method MUST be invoked from inside
// a tenant.WithAdminTx — the queries intentionally span tenant_id
// boundaries scoped only by auth.account_tenants membership, which
// requires BYPASSRLS to traverse.
package parent

import (
	"context"
	"fmt"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// ChildRepository implements parentModels.ChildRepository.
type ChildRepository struct {
	runtime   Runtime
	students  StudentDirectory
	guardians GuardianDirectory
}

// NewChildRepository wires a fresh repository.
func NewChildRepository(runtime Runtime) parentModels.ChildRepository {
	return &ChildRepository{runtime: requireRuntime(runtime)}
}

// BindStudentDirectory installs the People Directory the guardian links are
// resolved to children through (#2662).
func (r *ChildRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

// BindGuardianDirectory installs the People Directory the account's guardian
// links are read from (#2663).
func (r *ChildRepository) BindGuardianDirectory(guardians GuardianDirectory) {
	r.guardians = guardians
}

// guardianLink is one users.students_guardians row the account reaches
// through an ACTIVE auth.account_tenants mapping.
type guardianLink struct {
	StudentID         int64
	TenantID          int64
	GuardianProfileID int64
	Permissions       map[string]any
}

// ListByAccount lists the children a parent can see: the account's active
// auth.account_tenants memberships name the schools, the People Directory
// answers which guardian links the account holds there (#2663) and which
// students stand behind them (#2662), all inside the same admin transaction.
// The only safe filter for "students this parent can see" is the
// account_tenants membership, and that is a membership read, not a tenant
// context.
//
// Soft-deleted person rows are filtered by the composition layer.
// Inactive account_tenants are excluded so a parent who lost access to
// a school doesn't continue seeing its children.
//
// Alumni are excluded for the same reason. Graduation is a soft delete:
// the student row survives but the child has left the OGS, and every
// staff-facing read path already filters them out. Leaving them visible
// here would let a guardian keep filing sick notes, care exceptions and
// notes for a child the school no longer cares for (#405 review).
//
// Result ordering: tenant, then student id; the composition layer orders
// by name afterwards so the dashboard can render with stable grouping.
func (r *ChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	links, err := r.portalLinks(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: list children: %w", err)
	}
	return r.resolveChildren(ctx, links)
}

// FindForAccount resolves a single child the account is a guardian of.
// Same membership and directory reads as ListByAccount, narrowed to one
// student id. Returns nil, nil when the student is not linked to the
// account so the caller can map "not yours" to a 403/404 without leaking
// existence.
//
// This is THE authorization gate for every per-child parent write
// (services/parent resolvePermittedChild), so the alumnus exclusion that
// hides graduates from the dashboard also stops the writes: a guardian
// cannot submit a future-dated sick note or care exception for a child
// who has left the school (#405 review).
//
// MUST run inside a tenant.WithAdminTx — the reads span tenant_id
// boundaries scoped only by auth.account_tenants membership.
func (r *ChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return nil, fmt.Errorf("parent: student_id must be positive")
	}
	links, err := r.portalLinks(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: find child for account: %w", err)
	}
	matching := make([]guardianLink, 0, 1)
	for _, link := range links {
		if link.StudentID == studentID {
			matching = append(matching, link)
		}
	}
	children, err := r.resolveChildren(ctx, matching)
	if err != nil {
		return nil, err
	}
	if len(children) == 0 {
		return nil, nil
	}
	return children[0], nil
}

// portalLinks returns the account's guardian links at the schools it holds
// an ACTIVE mapping at, keeping only the links that grant portal access.
func (r *ChildRepository) portalLinks(ctx context.Context, accountID int64) ([]guardianLink, error) {
	links, err := activeGuardianLinks(ctx, r.runtime, r.guardians, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]guardianLink, 0, len(links))
	for _, link := range links {
		if !link.HasPermission(careplan.GuardianPermissionPortalAccess) {
			continue
		}
		out = append(out, guardianLink{
			StudentID: link.StudentID, TenantID: link.TenantID, GuardianProfileID: link.GuardianProfileID,
			Permissions: link.PermissionMap(),
		})
	}
	return out, nil
}

// resolveChildren joins the guardian links with the directory rows: a link
// stays only when its student still exists in the link's tenant and has not
// graduated, which is what the former inner join on users.students did.
func (r *ChildRepository) resolveChildren(ctx context.Context, links []guardianLink) ([]*parentModels.ChildSummary, error) {
	out := make([]*parentModels.ChildSummary, 0, len(links))
	if len(links) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.StudentID)
	}
	students, err := studentsByID(ctx, r.students, ids)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		student, found := students[link.StudentID]
		if !found || student.TenantID != link.TenantID || student.Status == careplan.StudentStatusAlumnus {
			continue
		}
		out = append(out, &parentModels.ChildSummary{
			StudentID:           student.ID,
			TenantID:            link.TenantID,
			PersonID:            student.PersonID,
			GuardianProfileID:   link.GuardianProfileID,
			SchoolClass:         student.SchoolClass,
			Status:              student.Status,
			EnrolledFrom:        parseDirectoryDate(student.EnrolledFrom),
			EnrolledUntil:       parseDirectoryDate(student.EnrolledUntil),
			GuardianPermissions: link.Permissions,
		})
	}
	return out, nil
}
