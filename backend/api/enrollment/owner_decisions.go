package enrollment

import (
	"context"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The decision flow as these routes speak it: the owner's decisions with the
// requests and children decoded.

// Values of the decision flow the owner's contract carries unchanged.
type (
	DecisionStatus              = capability.DecisionStatus
	DecideInput                 = capability.DecideInput
	PendingGuardianInvite       = capability.PendingGuardianInvite
	RequestFilters              = capability.DecisionRequestFilters
	OfferingAdjustmentSelection = capability.OfferingAdjustmentSelection
	UpdateChildOfferingsInput   = capability.UpdateChildOfferingsInput
	ChildOfferingRow            = capability.ChildOfferingRow
	ChildOfferingSet            = capability.ChildOfferingSet
	RestoreOutcome              = capability.RestoreOutcome
	OfferingAdjustment          = capability.OfferingAdjustmentRecord
)

// DecideOutcome is the refreshed child plus the post-commit guardian
// invitation an approval owes.
type DecideOutcome struct {
	Child         *RequestChild
	PendingInvite *PendingGuardianInvite
}

// RequestSummary is the admin-list shape: one row per request with its
// children.
type RequestSummary struct {
	Request    *enrollmentModels.Request
	Phase      *capability.Phase
	Children   []*RequestChild
	Guardians  []*capability.RequestGuardian
	LateInvite *capability.LateInvite
}

// PhaseExport is the fully-assembled payload for the compact phase export.
type PhaseExport struct {
	Phase   *capability.Phase
	Schemas map[int64]*capability.FormSchema
	Rows    []ExportRequestRow
}

// Counts returns the number of requests (rows) and the total number of
// children across them.
func (e *PhaseExport) Counts() (requests, children int) {
	return exportRowCounts(e.Rows)
}

// StudentEnrollmentExport is the fully-assembled payload for exports from
// one student's kartei tab.
type StudentEnrollmentExport struct {
	StudentID int64
	Schemas   map[int64]*capability.FormSchema
	Phases    map[int64]*capability.Phase
	Rows      []ExportRequestRow
}

// Counts returns the number of requests (rows) and the total number of
// children across them.
func (e *StudentEnrollmentExport) Counts() (requests, children int) {
	return exportRowCounts(e.Rows)
}

func exportRowCounts(rows []ExportRequestRow) (requests, children int) {
	for _, row := range rows {
		children += len(row.Children)
	}
	return len(rows), children
}

// ExportRequestRow is one parent submission with its resolved children and
// the co-guardians the parent submitted.
type ExportRequestRow struct {
	Request   *enrollmentModels.Request
	Children  []ExportChildRow
	Guardians []*capability.RequestGuardian
}

// ExportChildRow is one child plus its care-offering selections.
type ExportChildRow struct {
	Child     *RequestChild
	Offerings []ChildOfferingRow
}

// DecisionService backs the admin review UI over the owner's decision flow.
type DecisionService interface {
	List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error)
	ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error)
	Get(ctx context.Context, requestID int64) (*RequestSummary, error)
	Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error)
	RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error)
	UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error)
	ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*OfferingAdjustment, error)
	ListChildOfferings(ctx context.Context, requestID int64) (map[int64]ChildOfferingSet, error)
	ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error)
	ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error)
	RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *capability.Phase, format, statusFilter string, requestCount, childCount int) error
}

// NewDecisionService decodes the owner's decision flow for these routes. A
// nil owner yields nil.
func NewDecisionService(owner capability.Decisions) DecisionService {
	if owner == nil {
		return nil
	}
	return decisionService{owner: owner}
}

type decisionService struct{ owner capability.Decisions }

func (c decisionService) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	return decisionSummaries(c.owner.DecisionRequests(ctx, filters))
}

func (c decisionService) ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error) {
	return decisionSummaries(c.owner.StudentDecisionRequests(ctx, studentID))
}

func (c decisionService) Get(ctx context.Context, requestID int64) (*RequestSummary, error) {
	value, err := c.owner.DecisionRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return decisionSummary(value)
}

func (c decisionService) Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error) {
	value, err := c.owner.Decide(ctx, input)
	if err != nil {
		return nil, err
	}
	child, err := childValue(value.Child)
	if err != nil {
		return nil, err
	}
	return &DecideOutcome{Child: child, PendingInvite: value.PendingInvite}, nil
}

func (c decisionService) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error) {
	return c.owner.RestoreWithdrawn(ctx, requestID, restoredBy)
}

func (c decisionService) UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error) {
	value, err := c.owner.UpdateChildOfferings(ctx, input)
	if err != nil {
		return nil, err
	}
	return childValue(value)
}

func (c decisionService) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*OfferingAdjustment, error) {
	return c.owner.ListOfferingAdjustments(ctx, requestID, requestChildID)
}

func (c decisionService) ListChildOfferings(ctx context.Context, requestID int64) (map[int64]ChildOfferingSet, error) {
	return c.owner.ListChildOfferings(ctx, requestID)
}

func (c decisionService) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error) {
	value, err := c.owner.ExportPhase(ctx, phaseID, actorAccountID, actorRole, format, childStatusFilter)
	if err != nil {
		return nil, err
	}
	rows, err := exportRows(value.Rows)
	if err != nil {
		return nil, err
	}
	return &PhaseExport{Phase: value.Phase, Schemas: value.Schemas, Rows: rows}, nil
}

func (c decisionService) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error) {
	value, err := c.owner.ExportStudent(ctx, studentID, actorAccountID, actorRole, format)
	if err != nil {
		return nil, err
	}
	rows, err := exportRows(value.Rows)
	if err != nil {
		return nil, err
	}
	return &StudentEnrollmentExport{StudentID: value.StudentID, Schemas: value.Schemas, Phases: value.Phases, Rows: rows}, nil
}

func (c decisionService) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *capability.Phase, format, statusFilter string, requestCount, childCount int) error {
	return c.owner.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount)
}

func decisionSummaries(values []*capability.DecisionSummary, err error) ([]*RequestSummary, error) {
	if err != nil {
		return nil, err
	}
	out := make([]*RequestSummary, 0, len(values))
	for _, value := range values {
		summary, err := decisionSummary(value)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func decisionSummary(value *capability.DecisionSummary) (*RequestSummary, error) {
	if value == nil {
		return nil, nil
	}
	request, err := requestValue(value.Request)
	if err != nil {
		return nil, err
	}
	children, err := childValues(value.Children)
	if err != nil {
		return nil, err
	}
	return &RequestSummary{
		Request: request, Phase: value.Phase, Children: children,
		Guardians: value.Guardians, LateInvite: value.LateInvite,
	}, nil
}

func exportRows(values []capability.ExportRequestRow) ([]ExportRequestRow, error) {
	rows := make([]ExportRequestRow, 0, len(values))
	for _, value := range values {
		request, err := requestValue(value.Request)
		if err != nil {
			return nil, err
		}
		children := make([]ExportChildRow, 0, len(value.Children))
		for _, childRow := range value.Children {
			child, err := childValue(childRow.Child)
			if err != nil {
				return nil, err
			}
			children = append(children, ExportChildRow{Child: child, Offerings: childRow.Offerings})
		}
		rows = append(rows, ExportRequestRow{Request: request, Children: children, Guardians: value.Guardians})
	}
	return rows, nil
}
