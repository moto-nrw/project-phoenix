package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	return s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
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
	err := tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		child, findErr := s.ChildRepo.FindForAccount(adminCtx, accountID, studentID)
		if findErr != nil {
			return findErr
		}
		if child == nil {
			return ErrChildNotLinked
		}
		if requiredPermission != "" && !childHasPermission(child, requiredPermission) {
			return ErrGuardianPermissionDenied
		}
		resolved = &Child{
			TenantID:            child.TenantID,
			GuardianProfileID:   child.GuardianProfileID,
			GuardianPermissions: child.GuardianPermissions,
			StudentName:         strings.TrimSpace(child.FirstName + " " + child.LastName),
			SchoolName:          child.SchoolName,
			CareEnded:           child.CareEnded(careplan.Date(s.todayDate().String())),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func childHasPermission(child *parentModels.ChildSummary, permission string) bool {
	if child == nil {
		return false
	}
	return authorize.StudentGuardianHasPermission(&usersModels.StudentGuardian{
		Permissions: child.GuardianPermissions,
	}, permission)
}

// Child is the minimal resolved context a per-child write needs.
type Child struct {
	TenantID            int64
	GuardianProfileID   int64
	GuardianPermissions map[string]interface{}
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
	return authorize.StudentGuardianHasPermission(&usersModels.StudentGuardian{
		Permissions: c.GuardianPermissions,
	}, permission)
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
	policy, err := s.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyParentRequestReasonPolicy)
	if err != nil {
		s.Logger.Warn("parent: resolve reason policy failed, requiring a reason",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return usersSvc.ReasonRequiredFor(policy, false)
}
