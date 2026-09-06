package parent

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// EnrollmentRequestRepository implements
// parentModels.EnrollmentRequestRepository.
type EnrollmentRequestRepository struct {
	runtime   Runtime
	guardians GuardianDirectory
	commands  EnrollmentCommands
}

// NewEnrollmentRequestRepository wires a fresh repository.
func NewEnrollmentRequestRepository(runtime Runtime, commands EnrollmentCommands) parentModels.EnrollmentRequestRepository {
	if commands == nil {
		panic("parent repository: enrollment commands are required")
	}
	return &EnrollmentRequestRepository{runtime: requireRuntime(runtime), commands: commands}
}

// BindGuardianDirectory installs the People Directory the account's guardian
// links are read from (#2663).
func (r *EnrollmentRequestRepository) BindGuardianDirectory(guardians GuardianDirectory) {
	r.guardians = guardians
}

// ListByAccount composes Enrollment-owned applications with the account identity
// and relationship-level permissions needed by the parent dashboard.
//
// Cross-tenant — caller must wrap in tenant.WithAdminTx.
func (r *EnrollmentRequestRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	var accountEmail string
	err := runtimeDB(ctx, r.runtime).NewRaw("SELECT email FROM auth.accounts WHERE id = ?", accountID).Scan(ctx, &accountEmail)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("parent: load enrollment account identity: %w", err)
	}
	rows, err := r.commands.AccountRequests(ctx, accountID, accountEmail)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []*parentModels.EnrollmentRequestSummary{}, nil
	}

	// A request whose approved child became a student is only listed while
	// the account still holds parent_portal.enrollments.view on EVERY such
	// child at the request's school. The links belong to the People
	// Directory (#2663) and are read inside the same admin transaction.
	links, err := guardianLinksByAccount(ctx, r.guardians, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollment requests: %w", err)
	}
	viewable := make(map[[2]int64]struct{}, len(links))
	for _, link := range links {
		if link.HasPermission(careplan.GuardianPermissionEnrollmentsView) {
			viewable[[2]int64{link.TenantID, link.StudentID}] = struct{}{}
		}
	}

	hidden := make(map[int64]struct{})
	childrenByRequest := make(map[int64][]parentModels.EnrollmentRequestChildSummary, len(rows))
	for _, rr := range rows {
		for _, c := range rr.Children {
			if c.CreatedStudentID != nil {
				if _, ok := viewable[[2]int64{rr.TenantID, *c.CreatedStudentID}]; !ok {
					hidden[rr.RequestID] = struct{}{}
				}
			}
			childrenByRequest[rr.RequestID] = append(childrenByRequest[rr.RequestID], parentModels.EnrollmentRequestChildSummary{ChildID: c.ChildID, FirstName: c.FirstName, LastName: c.LastName, Status: c.Status, StatusReason: c.StatusReason})
		}
	}

	out := make([]*parentModels.EnrollmentRequestSummary, 0, len(rows))
	for _, rr := range rows {
		if _, skip := hidden[rr.RequestID]; skip {
			continue
		}
		out = append(out, &parentModels.EnrollmentRequestSummary{
			RequestID:                rr.RequestID,
			TenantID:                 rr.TenantID,
			StatusToken:              rr.StatusToken,
			SubmittedAt:              rr.SubmittedAt,
			WithdrawnAt:              rr.WithdrawnAt,
			PhaseID:                  rr.PhaseID,
			PhaseName:                rr.PhaseName,
			ServiceStartDate:         careplan.Date(rr.ServiceStartDate),
			ServiceEndDate:           careplan.Date(rr.ServiceEndDate),
			ShowStatusReasonToParent: rr.ShowStatusReasonToParent,
			Children:                 childrenByRequest[rr.RequestID],
		})
	}
	return out, nil
}

// BackfillGuardianAccountID claims every guardian-less enrollment
// request that matches the given email (case-insensitive) for the
// given account. Used by the guardian-invite-accept flow so requests
// submitted before the parent had an account show up in /me/enrollments
// after acceptance. Cross-tenant — caller wraps in WithAdminTx.
func (r *EnrollmentRequestRepository) BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error) {
	return r.commands.BackfillGuardianAccountID(ctx, accountID, email)
}

// EnrollmentCommands is Parent's port for reading and attaching pre-account submissions.
type EnrollmentCommands interface {
	AccountRequests(context.Context, int64, string) ([]enrollment.AccountRequest, error)
	BackfillGuardianAccountID(context.Context, int64, string) (int, error)
}
