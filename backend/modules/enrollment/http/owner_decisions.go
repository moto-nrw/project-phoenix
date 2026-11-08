package enrollmenthttp

import (
	"context"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The decision flow as these routes speak it: the owner's decisions and
// their values.

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
	RosterImpact                = capability.RosterImpact
	// ChildOfferingsUpdate is the corrected child with the effect of the
	// correction on the child's Regeltermine (#5102).
	ChildOfferingsUpdate = capability.ChildOfferingsUpdate
	// DecideOutcome is the refreshed child plus the post-commit guardian
	// invitation an approval owes.
	DecideOutcome = capability.DecideOutcome
	// RequestSummary is the admin-list shape: one row per request with its
	// children.
	RequestSummary = capability.DecisionSummary
	// ExportRequestRow is one parent submission with its resolved children
	// and the co-guardians the parent submitted.
	ExportRequestRow = capability.ExportRequestRow
	// ExportChildRow is one child plus its care-offering selections.
	ExportChildRow = capability.ExportChildRow
)

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

// DecisionService backs the admin review UI over the owner's decision flow.
type DecisionService interface {
	// RequestReads is the caller's read state of the queue (#3778).
	capability.RequestReads
	// DecisionCommand is the owner's decision with the guardian invitation
	// an approval owes (#6093); its outcome carries the owner's child.
	capability.DecisionCommand
	List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error)
	ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error)
	Get(ctx context.Context, requestID int64) (*RequestSummary, error)
	Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error)
	RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error)
	UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*ChildOfferingsUpdate, error)
	ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*OfferingAdjustment, error)
	ListChildOfferings(ctx context.Context, requestID int64) (map[int64]ChildOfferingSet, error)
	ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error)
	ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error)
	RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *capability.Phase, format, statusFilter string, requestCount, childCount int) error
}

// NewDecisionService serves the owner's decision flow to these routes. A
// nil owner yields nil.
func NewDecisionService(owner capability.Decisions) DecisionService {
	if owner == nil {
		return nil
	}
	return decisionService{owner: owner, DecisionCommand: owner}
}

type decisionService struct {
	owner capability.Decisions
	capability.DecisionCommand
}

func (c decisionService) CountUnreadRequests(ctx context.Context, accountID int64) (int, error) {
	return c.owner.CountUnreadRequests(ctx, accountID)
}

func (c decisionService) UnreadRequestIDs(ctx context.Context, accountID int64, requestIDs []int64) ([]int64, error) {
	return c.owner.UnreadRequestIDs(ctx, accountID, requestIDs)
}

func (c decisionService) MarkRequestsRead(ctx context.Context, accountID int64, requestIDs []int64) error {
	return c.owner.MarkRequestsRead(ctx, accountID, requestIDs)
}

func (c decisionService) MarkAllRequestsRead(ctx context.Context, accountID int64) error {
	return c.owner.MarkAllRequestsRead(ctx, accountID)
}

func (c decisionService) MarkRequestUnread(ctx context.Context, accountID, requestID int64) error {
	return c.owner.MarkRequestUnread(ctx, accountID, requestID)
}

func (c decisionService) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	return c.owner.DecisionRequests(ctx, filters)
}

func (c decisionService) ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error) {
	return c.owner.StudentDecisionRequests(ctx, studentID)
}

func (c decisionService) Get(ctx context.Context, requestID int64) (*RequestSummary, error) {
	return c.owner.DecisionRequest(ctx, requestID)
}

func (c decisionService) Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error) {
	return c.owner.Decide(ctx, input)
}

func (c decisionService) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error) {
	return c.owner.RestoreWithdrawn(ctx, requestID, restoredBy)
}

func (c decisionService) UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*ChildOfferingsUpdate, error) {
	return c.owner.UpdateChildOfferings(ctx, input)
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
	return &PhaseExport{Phase: value.Phase, Schemas: value.Schemas, Rows: value.Rows}, nil
}

func (c decisionService) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error) {
	value, err := c.owner.ExportStudent(ctx, studentID, actorAccountID, actorRole, format)
	if err != nil {
		return nil, err
	}
	return &StudentEnrollmentExport{StudentID: value.StudentID, Schemas: value.Schemas, Phases: value.Phases, Rows: value.Rows}, nil
}

func (c decisionService) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *capability.Phase, format, statusFilter string, requestCount, childCount int) error {
	return c.owner.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount)
}
