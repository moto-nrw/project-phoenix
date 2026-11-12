package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

var (
	// ErrChildNotLinked means the account is not a guardian of the
	// student. Handlers MUST map this to 403/404 and never leak whether
	// the student exists at another school.
	ErrChildNotLinked = errors.New("parent: child not linked to account")
	// ErrGuardianPermissionDenied means the account is linked to the child but
	// the relationship does not grant the requested parent-portal action.
	ErrGuardianPermissionDenied = errors.New("parent: guardian relationship lacks required permission")
	// ErrChildCareEnded means the child's care at this school has ended (#2487).
	// Every parent WRITE for that child is refused from the day after the last
	// care day; reading what happened before stays open, which is why this is
	// checked per write path and not in ResolvePermittedChild itself.
	ErrChildCareEnded = errors.New("parent: care for this child has ended")
)

// ResolveOwnedChild validates the account is a guardian of the student
// and returns the child's tenant id. The cross-tenant lookup runs under
// an admin tx; a nil child becomes ErrChildNotLinked so the caller never
// trusts a studentID it can't prove ownership of.
func (s *Service) ResolveOwnedChild(ctx context.Context, accountID, studentID int64) (*Child, error) {
	return s.ResolvePermittedChild(ctx, accountID, studentID, GuardianPermissionPortalAccess)
}

// RequireCareRunningForUpdate locks the child before a parent write so a
// concurrent care exit cannot turn an already-authorized operation into a
// post-exit write.
func (s *Service) RequireCareRunningForUpdate(ctx context.Context, studentID int64) error {
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}
	if student.CareEndedOn(s.todayDate()) {
		return ErrChildCareEnded
	}
	return nil
}

// ResolvePermittedChild validates the guardian link and, when
// requiredPermission is set, the relationship's parent-portal permission.
func (s *Service) ResolvePermittedChild(ctx context.Context, accountID, studentID int64, requiredPermission string) (*Child, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return nil, fmt.Errorf("parent: student_id must be positive")
	}

	var resolved *Child
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		var findErr error
		resolved, findErr = s.permittedChild(adminCtx, accountID, studentID, requiredPermission)
		return findErr
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

// WithPermittedChild is ResolvePermittedChild followed by a read of the
// child's school in ONE transaction, the one-way handoff of ADR 0161 (#5862).
// The guardian link and requiredPermission are checked exactly as in
// ResolvePermittedChild, in the administrative part and before any statement
// of the school. between then runs on the resolved child, still before the
// switch: it holds a check a read route runs today between the child lookup
// and its school unit; its error ends the transaction without a switch.
// read runs under the child's school's RLS. It is for read routes only;
// parent writes use WithPermittedChildForWrite.
func (s *Service) WithPermittedChild(
	ctx context.Context,
	accountID, studentID int64,
	requiredPermission string,
	between func(*Child) error,
	read func(ctx context.Context, child *Child) error,
) error {
	if accountID <= 0 {
		return fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return fmt.Errorf("parent: student_id must be positive")
	}
	if read == nil {
		return fmt.Errorf("parent: child read is required")
	}
	var resolved *Child
	return s.Runtime.WithinAdminThenTenant(ctx, func(adminCtx context.Context) (int64, error) {
		child, err := s.permittedChild(adminCtx, accountID, studentID, requiredPermission)
		if err != nil {
			return 0, err
		}
		if between != nil {
			if err := between(child); err != nil {
				return 0, err
			}
		}
		resolved = child
		return child.TenantID, nil
	}, func(txCtx context.Context) error {
		return read(txCtx, resolved)
	})
}

// WithPermittedChildForWrite is the write counterpart of WithPermittedChild
// (ADR 0161 Nachtrag zu Schreibrouten, #5862): the guardian link and
// requiredPermission are checked as admin, between runs the route's Go checks
// that sit today between the child check and its school unit, and write runs
// under the child's school's RLS, all in ONE transaction. Nothing of the
// administrative part locks a row: the school part runs guardChildWrite with
// requiredPermission before write, which then takes the same locks in the
// same order as in its own school unit (#6529).
//
// A caller inside an open unit, or with a context already bound to a school,
// takes the way before ADR 0161, chosen from the context before anything
// starts: ResolvePermittedChild, between, then write in its own school unit.
// The handoff would refuse such a context before the child check (ADR 0161
// point 7). A unit that cannot start, switch roles or commit fails with its
// error once: no retry and no second way after a failure.
func (s *Service) WithPermittedChildForWrite(
	ctx context.Context,
	accountID, studentID int64,
	requiredPermission string,
	between func(*Child) error,
	write func(ctx context.Context, child *Child) error,
	adjust ...func(*ChildWriteAccess),
) error {
	access := childWriteNeeds(requiredPermission)
	for _, change := range adjust {
		change(&access)
	}
	if write == nil {
		return fmt.Errorf("parent: child write is required")
	}
	if unitOpen, bound := s.Runtime.AmbientScope(ctx); unitOpen || bound != 0 {
		return s.permittedChildWriteUnbundled(ctx, accountID, studentID, requiredPermission, access, between, write)
	}
	if accountID <= 0 {
		return fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return fmt.Errorf("parent: student_id must be positive")
	}
	var resolved *Child
	return s.Runtime.WithinAdminThenTenant(ctx, func(adminCtx context.Context) (int64, error) {
		child, err := s.permittedChild(adminCtx, accountID, studentID, requiredPermission)
		if err != nil {
			return 0, err
		}
		if between != nil {
			if err := between(child); err != nil {
				return 0, err
			}
		}
		resolved = child
		return child.TenantID, nil
	}, func(txCtx context.Context) error {
		if err := s.guardChildWrite(txCtx, accountID, studentID, resolved, access); err != nil {
			return err
		}
		return write(txCtx, resolved)
	})
}

// permittedChildWriteUnbundled is the way before ADR 0161 for an ambient
// caller: the child check in its administrative unit, between, then write in
// the child's own school unit.
func (s *Service) permittedChildWriteUnbundled(
	ctx context.Context,
	accountID, studentID int64,
	requiredPermission string,
	access ChildWriteAccess,
	between func(*Child) error,
	write func(ctx context.Context, child *Child) error,
) error {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, requiredPermission)
	if err != nil {
		return err
	}
	if between != nil {
		if err := between(child); err != nil {
			return err
		}
	}
	return s.WithChildWrite(ctx, accountID, studentID, child, access, func(txCtx context.Context) error {
		return write(txCtx, child)
	})
}

// permittedChild is the relationship check of ResolvePermittedChild inside
// the caller's administrative transaction.
func (s *Service) permittedChild(adminCtx context.Context, accountID, studentID int64, requiredPermission string) (*Child, error) {
	child, err := s.ChildRepo.FindForAccount(adminCtx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if child == nil {
		return nil, ErrChildNotLinked
	}
	if requiredPermission != "" && !childHasPermission(child, requiredPermission) {
		return nil, ErrGuardianPermissionDenied
	}
	return &Child{
		TenantID:            child.TenantID,
		GuardianProfileID:   child.GuardianProfileID,
		GuardianPermissions: child.GuardianPermissions,
		StudentName:         strings.TrimSpace(child.FirstName + " " + child.LastName),
		SchoolName:          child.SchoolName,
		CareEnded:           child.CareEnded(s.todayDate()),
	}, nil
}

func childHasPermission(child *ChildSummary, permission string) bool {
	if child == nil {
		return false
	}
	return child.GuardianPermissions[permission]
}

// Child is the minimal resolved context a per-child write needs.
type Child struct {
	TenantID            int64
	GuardianProfileID   int64
	GuardianPermissions map[string]bool
	// StudentName / SchoolName feed the OGS messaging views (thread counterpart
	// + child label); resolved once here from the cross-tenant child lookup.
	StudentName string
	SchoolName  string
	// CareEnded mirrors the child's enrollment interval as of today (#2487).
	CareEnded bool
}

// RequireCareRunning refuses a write for a child whose care has ended. Reads
// deliberately do not call it: a family keeps access to what happened while
// their child was here.
func (c *Child) RequireCareRunning() error {
	if c == nil {
		return ErrChildNotLinked
	}
	if c.CareEnded {
		return ErrChildCareEnded
	}
	return nil
}

// HasPermission reports whether the guardian relationship grants permission.
func (c *Child) HasPermission(permission string) bool {
	if c == nil {
		return false
	}
	return c.GuardianPermissions[permission]
}

// GuardianReasonRequired resolves operations.parent_request_reason_policy for
// the child's school and answers whether the SUBMITTING family must state a
// reason (#2267, story 28). A read failure falls back to the strictest
// reading: asking for a reason that was not required is a nuisance, dropping
// one that was required loses information nobody can recover later.
func (s *Service) GuardianReasonRequired(ctx context.Context, tenantID int64) bool {
	if s.Settings == nil {
		return true
	}
	policy, err := s.Settings.ResolveStringForTenant(ctx, tenantID, KeyParentRequestReasonPolicy)
	if err != nil {
		s.Logger.Warn("parent: resolve reason policy failed, requiring a reason",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return parentrequests.ReasonRequired(policy, parentrequests.ReasonSideGuardian)
}
