package parent

import (
	"context"
	"fmt"
	"slices"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// EnrollablePhaseRepository implements parentModels.EnrollablePhaseRepository.
type EnrollablePhaseRepository struct {
	runtime   Runtime
	phases    PhaseQueries
	students  StudentDirectory
	guardians GuardianDirectory
}

// BindStudentDirectory installs the People Directory the enrolled-student
// eligibility is resolved through (#2662).
func (r *EnrollablePhaseRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

// BindGuardianDirectory installs the People Directory the account's guardian
// links are read from (#2663).
func (r *EnrollablePhaseRepository) BindGuardianDirectory(guardians GuardianDirectory) {
	r.guardians = guardians
}

// enrolledSubmitPersonIDs maps the submit-permitted guardian links of one
// school to the persons of the children that are still ACTIVE or PENDING at
// that school. The students belong to the People Directory (#2662); the
// former join carried the same status and tenant predicates.
func (r *EnrollablePhaseRepository) enrolledSubmitPersonIDs(students map[int64]DirectoryStudent, tenantID int64, studentIDs []int64) []int64 {
	personIDs := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, studentID := range studentIDs {
		student, found := students[studentID]
		if !found || student.TenantID != tenantID {
			continue
		}
		if student.Status != careplan.StudentStatusActive && student.Status != careplan.StudentStatusPending {
			continue
		}
		if _, dup := seen[student.PersonID]; dup {
			continue
		}
		seen[student.PersonID] = struct{}{}
		personIDs = append(personIDs, student.PersonID)
	}
	return personIDs
}

// NewEnrollablePhaseRepository wires a fresh repository.
type PhaseQueries interface {
	OpenPhaseCandidates(context.Context) ([]*enrollment.Phase, error)
}

func NewEnrollablePhaseRepository(runtime Runtime, phases PhaseQueries) parentModels.EnrollablePhaseRepository {
	if phases == nil {
		panic("parent: enrollment phase queries are required")
	}
	return &EnrollablePhaseRepository{runtime: requireRuntime(runtime), phases: phases}
}

// guardianGuard is the per-school guardian evidence the picker and the
// submit gate decide on: whether the account holds any guardian link there
// (backed by an ACTIVE mapping), whether one of them grants
// parent_portal.enrollment.submit, and the children behind the permitted
// links.
type guardianGuard struct {
	hasFamilyLink       bool
	hasSubmitPermission bool
	submitStudentIDs    []int64
}

// guardianGuards resolves the guard per tenant from the account's links at
// the schools where it holds an ACTIVE auth.account_tenants mapping.
func (r *EnrollablePhaseRepository) guardianGuards(ctx context.Context, accountID int64, activeTenants map[int64]struct{}) (map[int64]*guardianGuard, error) {
	links, err := guardianLinksByAccount(ctx, r.guardians, accountID)
	if err != nil {
		return nil, err
	}
	guards := make(map[int64]*guardianGuard)
	seen := make(map[int64]map[int64]struct{})
	for _, link := range links {
		if _, active := activeTenants[link.TenantID]; !active {
			continue
		}
		guard := guards[link.TenantID]
		if guard == nil {
			guard = &guardianGuard{}
			guards[link.TenantID] = guard
			seen[link.TenantID] = make(map[int64]struct{})
		}
		guard.hasFamilyLink = true
		if !link.HasPermission(careplan.GuardianPermissionEnrollmentSubmit) {
			continue
		}
		guard.hasSubmitPermission = true
		if _, dup := seen[link.TenantID][link.StudentID]; dup {
			continue
		}
		seen[link.TenantID][link.StudentID] = struct{}{}
		guard.submitStudentIDs = append(guard.submitStudentIDs, link.StudentID)
	}
	return guards, nil
}

// ListEnrollable returns one row per (school, active+open phase) pair
// the account is eligible for.
//
// "Open" = phase.is_active AND now BETWEEN enrollment_open_at and
// enrollment_close_at (treating NULL bounds as open-ended). Enrollment supplies
// the candidates; active account memberships determine the already_linked flag.
//
// Hidden schools (platform.schools.hidden) are excluded from cross-school
// discovery the same way the public tenant listing excludes them: an
// unlinked parent must not learn a hidden school's name or phase details
// through this picker. A hidden school stays visible only to an account that
// holds an actual FAMILY link there — a guardian_profile with at least one
// students_guardians row, backed by an ACTIVE auth.account_tenants mapping
// (HasFamilyLink) — so existing families keep seeing their own school's
// re-enrollment phases.
//
// The membership mapping alone (at.account_id / the already_linked display
// flag) is deliberately NOT the visibility key: auth.account_tenants also
// carries staff, admins, and any other role at that tenant, so keying on it
// leaked a hidden school's name, phase names, and service dates to accounts
// with no guardian relationship at all. already_linked keeps its membership
// meaning — it only orders and labels rows that already passed this gate.
//
// Eligibility (#1663): a phase whose audience is linked_parents OR
// existing_students is listed only when the account holds a guardian
// relationship at the school — backed by an ACTIVE auth.account_tenants
// mapping — that grants parent_portal.enrollment.submit. The active-mapping
// conjunction stops a former guardian, whose historical guardian rows linger
// after the mapping was deactivated, from still submitting those phases.
//
// existing_students needs the SAME permission requirement as linked_parents,
// including for an account with no guardian relationship at that school at
// all: such an account can never complete one of those phases. A child it
// does not already have fails the enrolled-student gate (ErrChildNotEnrolled),
// and a child it does have pins a matched student the account holds no
// per-student submit permission on (ErrChildEnrollmentNotPermitted). Listing
// it would only advertise a guaranteed dead end.
//
// existing_students goes one step further and requires the permission-granting
// relationship to point at a student that is still ACTIVE or PENDING (its
// person not soft-deleted) — the same "enrolled" scope
// ExistsEnrolledByNameAndBirthday / FindEnrolledStudentIDByNameAndBirthday use
// on submit. An account whose only submit-permitted relationship is to an
// inactive (or otherwise un-enrolled) child would otherwise see the phase, find
// that child filtered out of the form, and hit ErrChildNotEnrolled on submit:
// another guaranteed dead end. linked_parents keeps the looser probe on
// purpose — it lets a parent enroll a genuinely NEW sibling, so an inactive
// child still legitimately grants access there.
//
// The denial stays deliberately scoped to those two audiences: a pickup-only
// relationship on one child must NOT hide open / new_students phases the same
// account can bootstrap a genuinely new child into and submit via a direct URL
// — the authenticated submit path applies no such account-wide denial
// (per-child authorization happens inside Submit), so the picker must not
// either. Genuinely new-school applicants (no guardian rows at all) still see
// open phases.
//
// Guardian rows belong to the People Directory (#2663). Phase candidates,
// memberships, and guardian evidence share the caller's admin transaction.
//
// Cross-tenant query — must run inside tenant.WithAdminTx.
func (r *EnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	rows, err := r.phases.OpenPhaseCandidates(ctx)
	if err != nil {
		return nil, err
	}
	activeTenants, err := activeMappingTenants(ctx, r.runtime, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollable phases: %w", err)
	}
	slices.SortStableFunc(rows, func(a, b *enrollment.Phase) int {
		_, aLinked := activeTenants[a.TenantID]
		_, bLinked := activeTenants[b.TenantID]
		if aLinked && !bLinked {
			return -1
		}
		if !aLinked && bLinked {
			return 1
		}
		return 0
	})

	guards, err := r.guardianGuards(ctx, accountID, activeTenants)
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollable phases: %w", err)
	}

	studentIDs := make([]int64, 0)
	for _, guard := range guards {
		studentIDs = append(studentIDs, guard.submitStudentIDs...)
	}
	students, err := studentsByID(ctx, r.students, studentIDs)
	if err != nil {
		return nil, err
	}

	out := make([]*parentModels.EnrollablePhase, 0, len(rows))
	for _, rr := range rows {
		_, alreadyLinked := activeTenants[rr.TenantID]
		guard := guards[rr.TenantID]
		if guard == nil {
			guard = &guardianGuard{}
		}
		if rr.Audience == "linked_parents" && !guard.hasSubmitPermission {
			continue
		}
		out = append(out, &parentModels.EnrollablePhase{
			SchoolID:          rr.TenantID,
			PhaseID:           rr.ID,
			PhaseName:         rr.Name,
			PhaseKind:         rr.Kind,
			ServiceStartDate:  careplan.Date(rr.ServiceStartDate),
			ServiceEndDate:    careplan.Date(rr.ServiceEndDate),
			EnrollmentOpenAt:  rr.EnrollmentOpenAt,
			EnrollmentCloseAt: rr.EnrollmentCloseAt,
			AlreadyLinked:     alreadyLinked,
			Audience:          rr.Audience,
			HasFamilyLink:     guard.hasFamilyLink,

			EnrolledSubmitPersonIDs: r.enrolledSubmitPersonIDs(students, rr.TenantID, guard.submitStudentIDs),
		})
	}
	return out, nil
}

// GuardianSubmitStatus resolves the (account, school) facts the parent
// enrollment submit and form-load paths gate on (#1663). HasSubmitPermission
// additionally requires an ACTIVE auth.account_tenants mapping so a
// deactivated guardian's lingering guardian rows cannot report submit
// authority; HasGuardianLink deliberately does not, it reports the
// relationship as such.
//
// EnrolledSubmitPersonIDs is the existing_students counterpart and MUST stay
// identical to the guard ListEnrollable applies: it additionally requires the
// permission-granting relationship to point at an ACTIVE or PENDING student
// whose person is not soft-deleted. The picker and the authenticated form
// gate have to agree — a form that loads for a phase the picker hides is a
// dead end whose submit always fails. Cross-tenant — must run inside
// tenant.WithAdminTx.
func (r *EnrollablePhaseRepository) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentModels.GuardianSubmitStatus, error) {
	if accountID <= 0 || tenantID <= 0 {
		return nil, fmt.Errorf("parent: account_id and tenant_id must be positive")
	}

	tenants, err := activeMappingTenants(ctx, r.runtime, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: guardian submit status: %w", err)
	}
	_, linked := tenants[tenantID]

	links, err := guardianLinksByAccount(ctx, r.guardians, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: guardian submit status: %w", err)
	}
	status := &parentModels.GuardianSubmitStatus{Linked: linked}
	submitStudentIDs := make([]int64, 0)
	seen := make(map[int64]struct{})
	for _, link := range links {
		if link.TenantID != tenantID {
			continue
		}
		status.HasGuardianLink = true
		if !linked || !link.HasPermission(careplan.GuardianPermissionEnrollmentSubmit) {
			continue
		}
		status.HasSubmitPermission = true
		if _, dup := seen[link.StudentID]; dup {
			continue
		}
		seen[link.StudentID] = struct{}{}
		submitStudentIDs = append(submitStudentIDs, link.StudentID)
	}

	students, err := studentsByID(ctx, r.students, submitStudentIDs)
	if err != nil {
		return nil, err
	}
	status.EnrolledSubmitPersonIDs = r.enrolledSubmitPersonIDs(students, tenantID, submitStudentIDs)
	return status, nil
}
