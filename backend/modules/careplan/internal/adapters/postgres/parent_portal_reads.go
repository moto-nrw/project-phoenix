package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewParentChildQueries adapts retained query records to public projections.
// It resolves stored permission values with the policy
// supplied by the root, never by guessing truthiness in the projection.
func NewParentChildQueries(source parentModels.ChildRepository, granted func(any) bool) careplan.ParentPortalChildQueries {
	if granted == nil {
		panic("care plan: guardian permission policy is required")
	}
	return parentChildQueries{source: source, granted: granted}
}

type parentChildQueries struct {
	source  parentModels.ChildRepository
	granted func(any) bool
}

func (q parentChildQueries) FindForAccount(ctx context.Context, accountID, studentID int64) (*careplan.ParentPortalChild, error) {
	row, err := q.source.FindForAccount(ctx, accountID, studentID)
	return q.child(row), err
}
func (q parentChildQueries) ListByAccount(ctx context.Context, accountID int64) ([]*careplan.ParentPortalChild, error) {
	rows, err := q.source.ListByAccount(ctx, accountID)
	if rows == nil {
		return nil, err
	}
	out := make([]*careplan.ParentPortalChild, len(rows))
	for i, row := range rows {
		out[i] = q.child(row)
	}
	return out, err
}
func (q parentChildQueries) child(row *parentModels.ChildSummary) *careplan.ParentPortalChild {
	if row == nil {
		return nil
	}
	var permissions map[string]bool
	if row.GuardianPermissions != nil {
		permissions = make(map[string]bool, len(row.GuardianPermissions))
		for key, value := range row.GuardianPermissions {
			permissions[key] = q.granted(value)
		}
	}
	return &careplan.ParentPortalChild{
		StudentID:           row.StudentID,
		TenantID:            row.TenantID,
		PersonID:            row.PersonID,
		FirstName:           row.FirstName,
		LastName:            row.LastName,
		SchoolClass:         row.SchoolClass,
		Status:              row.Status,
		EnrolledFrom:        parentPortalDatePointer(row.EnrolledFrom),
		EnrolledUntil:       parentPortalDatePointer(row.EnrolledUntil),
		SchoolName:          row.SchoolName,
		SchoolSlug:          row.SchoolSlug,
		GuardianPermissions: permissions,
		GuardianProfileID:   row.GuardianProfileID,
	}
}
func parentPortalDatePointer(value *careplan.Date) *calendar.Date {
	if value == nil {
		return nil
	}
	day := calendar.Date(*value)
	return &day
}
func NewParentPhaseQueries(source parentModels.EnrollablePhaseRepository) careplan.ParentPortalPhaseQueries {
	return parentPhaseQueries{source: source}
}

type parentPhaseQueries struct {
	source parentModels.EnrollablePhaseRepository
}

func (q parentPhaseQueries) ListEnrollable(ctx context.Context, accountID int64) ([]*careplan.ParentPortalPhase, error) {
	rows, err := q.source.ListEnrollable(ctx, accountID)
	if rows == nil {
		return nil, err
	}
	out := make([]*careplan.ParentPortalPhase, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		out[i] = &careplan.ParentPortalPhase{
			SchoolID:                row.SchoolID,
			SchoolName:              row.SchoolName,
			SchoolSlug:              row.SchoolSlug,
			SchoolSubdomain:         row.SchoolSubdomain,
			PhaseID:                 row.PhaseID,
			PhaseName:               row.PhaseName,
			PhaseNameTranslations:   row.PhaseNameTranslations,
			PhaseKind:               row.PhaseKind,
			ServiceStartDate:        calendar.Date(row.ServiceStartDate),
			ServiceEndDate:          calendar.Date(row.ServiceEndDate),
			EnrollmentOpenAt:        row.EnrollmentOpenAt,
			EnrollmentCloseAt:       row.EnrollmentCloseAt,
			AlreadyLinked:           row.AlreadyLinked,
			Audience:                row.Audience,
			HasFamilyLink:           row.HasFamilyLink,
			EnrolledSubmitPersonIDs: row.EnrolledSubmitPersonIDs,
		}
	}
	return out, err
}
func (q parentPhaseQueries) GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*careplan.ParentPortalSubmitStatus, error) {
	row, err := q.source.GuardianSubmitStatus(ctx, accountID, tenantID)
	if row == nil {
		return nil, err
	}
	out := careplan.ParentPortalSubmitStatus(*row)
	return &out, err
}
func NewParentEnrollmentQueries(source parentModels.EnrollmentRequestRepository) careplan.ParentPortalEnrollmentQueries {
	return parentEnrollmentQueries{source: source}
}

type parentEnrollmentQueries struct {
	source parentModels.EnrollmentRequestRepository
}

func (q parentEnrollmentQueries) ListByAccount(ctx context.Context, accountID int64) ([]*careplan.ParentPortalEnrollment, error) {
	rows, err := q.source.ListByAccount(ctx, accountID)
	if rows == nil {
		return nil, err
	}
	out := make([]*careplan.ParentPortalEnrollment, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		var children []careplan.ParentPortalEnrollmentChild
		if row.Children != nil {
			children = make([]careplan.ParentPortalEnrollmentChild, len(row.Children))
			for j, child := range row.Children {
				children[j] = careplan.ParentPortalEnrollmentChild(child)
			}
		}
		out[i] = &careplan.ParentPortalEnrollment{
			RequestID:                row.RequestID,
			TenantID:                 row.TenantID,
			StatusToken:              row.StatusToken,
			SubmittedAt:              row.SubmittedAt,
			WithdrawnAt:              row.WithdrawnAt,
			PhaseID:                  row.PhaseID,
			PhaseName:                row.PhaseName,
			ServiceStartDate:         calendar.Date(row.ServiceStartDate),
			ServiceEndDate:           calendar.Date(row.ServiceEndDate),
			ShowStatusReasonToParent: row.ShowStatusReasonToParent,
			SchoolName:               row.SchoolName,
			SchoolSlug:               row.SchoolSlug,
			Children:                 children,
		}
	}
	return out, err
}
