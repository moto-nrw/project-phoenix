package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentChildReads binds the owner public projection to the workflow port.
// Stored permission values are resolved by Security Runtime's shared policy.
func NewParentChildReads(source careplan.ParentPortalChildQueries) parentportal.ChildReads {
	if source == nil {
		return nil
	}
	return parentChildReads{source: source}
}

type parentChildReads struct {
	source careplan.ParentPortalChildQueries
}

func (r parentChildReads) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentportal.ChildSummary, error) {
	row, err := r.source.FindForAccount(ctx, accountID, studentID)
	return parentPortalChild(row), err
}

func (r parentChildReads) ListByAccount(ctx context.Context, accountID int64) ([]*parentportal.ChildSummary, error) {
	rows, err := r.source.ListByAccount(ctx, accountID)
	if rows == nil {
		return nil, err
	}
	out := make([]*parentportal.ChildSummary, len(rows))
	for i, row := range rows {
		out[i] = parentPortalChild(row)
	}
	return out, err
}

func parentPortalChild(row *careplan.ParentPortalChild) *parentportal.ChildSummary {
	if row == nil {
		return nil
	}
	out := parentportal.ChildSummary(*row)
	return &out
}

func NewParentPhaseReads(source careplan.ParentPortalPhaseQueries) parentportal.EnrollablePhaseReads {
	if source == nil {
		return nil
	}
	return parentPhaseReads{source: source}
}

type parentPhaseReads struct {
	source careplan.ParentPortalPhaseQueries
}

func (r parentPhaseReads) ListEnrollable(ctx context.Context, accountID int64) ([]*parentportal.EnrollablePhase, error) {
	rows, err := r.source.ListEnrollable(ctx, accountID)
	if rows == nil {
		return nil, err
	}
	out := make([]*parentportal.EnrollablePhase, len(rows))
	for i, row := range rows {
		if row != nil {
			value := parentportal.EnrollablePhase(*row)
			out[i] = &value
		}
	}
	return out, err
}

func (r parentPhaseReads) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentportal.GuardianSubmitStatus, error) {
	row, err := r.source.GuardianSubmitStatus(ctx, accountID, tenantID)
	if row == nil {
		return nil, err
	}
	value := parentportal.GuardianSubmitStatus(*row)
	return &value, err
}

func NewParentEnrollmentReads(source careplan.ParentPortalEnrollmentQueries) parentportal.EnrollmentRequestReads {
	if source == nil {
		return nil
	}
	return parentEnrollmentReads{source: source}
}

type parentEnrollmentReads struct {
	source careplan.ParentPortalEnrollmentQueries
}

func (r parentEnrollmentReads) ListByAccount(ctx context.Context, accountID int64) ([]*parentportal.EnrollmentRequestSummary, error) {
	rows, err := r.source.ListByAccount(ctx, accountID)
	if rows == nil {
		return nil, err
	}
	out := make([]*parentportal.EnrollmentRequestSummary, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		var children []parentportal.EnrollmentRequestChildSummary
		if row.Children != nil {
			children = make([]parentportal.EnrollmentRequestChildSummary, len(row.Children))
			for j, child := range row.Children {
				children[j] = parentportal.EnrollmentRequestChildSummary(child)
			}
		}
		out[i] = &parentportal.EnrollmentRequestSummary{
			RequestID: row.RequestID, TenantID: row.TenantID, StatusToken: row.StatusToken,
			SubmittedAt: row.SubmittedAt, WithdrawnAt: row.WithdrawnAt,
			PhaseID: row.PhaseID, PhaseName: row.PhaseName,
			ServiceStartDate: row.ServiceStartDate, ServiceEndDate: row.ServiceEndDate,
			ShowStatusReasonToParent: row.ShowStatusReasonToParent, SchoolName: row.SchoolName, SchoolSlug: row.SchoolSlug,
			Children: children,
		}
	}
	return out, err
}
