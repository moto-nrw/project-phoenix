package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// ParentPortalEnrollments serves the guardian portal's phase and application
// views of Care Plan from Enrollment. Enrollment sits above Care Plan
// (ADR 0058, #3417), so Care Plan reads them in its own vocabulary.
type ParentPortalEnrollments struct {
	Enrollment enrollment.Owner
}

func (p ParentPortalEnrollments) OpenPhaseCandidates(ctx context.Context) ([]*careplan.ParentEnrollmentPhase, error) {
	phases, err := p.Enrollment.OpenPhaseCandidates(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*careplan.ParentEnrollmentPhase, 0, len(phases))
	for _, phase := range phases {
		result = append(result, &careplan.ParentEnrollmentPhase{
			ID: phase.ID, TenantID: phase.TenantID, Name: phase.Name, NameTranslations: phase.PublicNameTranslations(),
			Kind: phase.Kind, ServiceStartDate: careplan.Date(phase.ServiceStartDate), ServiceEndDate: careplan.Date(phase.ServiceEndDate),
			EnrollmentOpenAt: phase.EnrollmentOpenAt, EnrollmentCloseAt: phase.EnrollmentCloseAt, Audience: phase.Audience,
		})
	}
	return result, nil
}

func (p ParentPortalEnrollments) AccountRequests(ctx context.Context, accountID int64, accountEmail string) ([]careplan.ParentEnrollmentRequest, error) {
	requests, err := p.Enrollment.AccountRequests(ctx, accountID, accountEmail)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.ParentEnrollmentRequest, 0, len(requests))
	for _, request := range requests {
		children := make([]careplan.ParentEnrollmentRequestChild, 0, len(request.Children))
		for _, child := range request.Children {
			children = append(children, careplan.ParentEnrollmentRequestChild(child))
		}
		result = append(result, careplan.ParentEnrollmentRequest{
			RequestID: request.RequestID, TenantID: request.TenantID, StatusToken: request.StatusToken,
			SubmittedAt: request.SubmittedAt, WithdrawnAt: request.WithdrawnAt, PhaseID: request.PhaseID, PhaseName: request.PhaseName,
			ServiceStartDate: careplan.Date(request.ServiceStartDate), ServiceEndDate: careplan.Date(request.ServiceEndDate),
			ShowStatusReasonToParent: request.ShowStatusReasonToParent, Children: children,
		})
	}
	return result, nil
}

func (p ParentPortalEnrollments) BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error) {
	return p.Enrollment.BackfillGuardianAccountID(ctx, accountID, email)
}
