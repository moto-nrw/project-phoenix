package enrollment

import (
	"context"
	"encoding/json"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The decision flow, the restore, the rollover and the deletions run in the
// Enrollment owner (modules/enrollment, #3564). The retained handlers, the
// request and change-request services and the scheduler still speak the
// decoded request and child values, so this facade decodes the owner's
// records for them until #3565 moves those flows into the owner too. It
// decides nothing itself.

// Sentinels of the decision flow, pointed at the owner values.
var (
	ErrDecisionRequestNotFound   = careplan.ErrBookingRequestNotFound
	ErrDecisionChildNotFound     = careplan.ErrBookingChildNotFound
	ErrDecisionStudentNotFound   = capability.ErrDecisionStudentNotFound
	ErrDecisionInvalidStatus     = capability.ErrDecisionInvalidStatus
	ErrDecisionAlreadyTerminal   = capability.ErrDecisionAlreadyTerminal
	ErrOfferingAdjustmentInvalid = careplan.ErrOfferingAdjustmentInvalid
	ErrDecisionInvalidData       = capability.ErrDecisionInvalidData
	ErrGuardianAccountMismatch   = capability.ErrGuardianAccountMismatch
	ErrWaitlistDisabled          = capability.ErrWaitlistDisabled
	ErrExportTooLarge            = capability.ErrExportTooLarge
	ErrRestoreNothingWithdrawn   = capability.ErrRestoreNothingWithdrawn
	ErrRestorePhaseInactive      = capability.ErrRestorePhaseInactive
	ErrRestoreDuplicateActive    = capability.ErrRestoreDuplicateActive
	// ErrCompleteWithdrawalConfirmationRequired points the enrollment name
	// at the Care Plan value.
	ErrCompleteWithdrawalConfirmationRequired = careplan.ErrCompleteWithdrawalConfirmationRequired
)

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

const (
	DecisionApproved    = capability.DecisionApproved
	DecisionWaitlisted  = capability.DecisionWaitlisted
	DecisionRejected    = capability.DecisionRejected
	DecisionUnderReview = capability.DecisionUnderReview
)

// DecisionLateInvites reads the late invite a request was submitted
// through.
type DecisionLateInvites interface {
	LateInviteByUsedRequestID(context.Context, int64) (*capability.LateInvite, error)
}

// DecideOutcome is what the admin handler gets back from Decide: the
// refreshed child plus the post-commit guardian invitation an approval owes.
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

// SyncApprovedChildDataInput carries a confirmed change request onto the
// student an approval created.
type SyncApprovedChildDataInput struct {
	RequestID                int64
	ChildID                  int64
	ActorAccountID           int64
	ReplaceTargetedData      bool
	PreviousSnapshot         map[string]any
	PreviousRequestGuardians []*capability.RequestGuardian
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

// NewDecisionService decodes the owner's decision flow for the retained
// consumers.
func NewDecisionService(owner capability.Decisions) DecisionService {
	return decisionContract{owner: owner}
}

type decisionContract struct {
	owner capability.Decisions
}

func (c decisionContract) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	values, err := c.owner.DecisionRequests(ctx, filters)
	if err != nil {
		return nil, err
	}
	return decisionSummaries(values)
}

func (c decisionContract) ListByStudent(ctx context.Context, studentID int64) ([]*RequestSummary, error) {
	values, err := c.owner.StudentDecisionRequests(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return decisionSummaries(values)
}

func (c decisionContract) Get(ctx context.Context, requestID int64) (*RequestSummary, error) {
	value, err := c.owner.DecisionRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return decisionSummary(value)
}

func (c decisionContract) Decide(ctx context.Context, input DecideInput) (*DecideOutcome, error) {
	value, err := c.owner.Decide(ctx, input)
	if err != nil {
		return nil, err
	}
	child, err := intakeChildValue(value.Child)
	if err != nil {
		return nil, err
	}
	return &DecideOutcome{Child: child, PendingInvite: value.PendingInvite}, nil
}

func (c decisionContract) RestoreWithdrawn(ctx context.Context, requestID, restoredBy int64) (*RestoreOutcome, error) {
	return c.owner.RestoreWithdrawn(ctx, requestID, restoredBy)
}

func (c decisionContract) UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error) {
	value, err := c.owner.UpdateChildOfferings(ctx, input)
	if err != nil {
		return nil, err
	}
	return intakeChildValue(value)
}

func (c decisionContract) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*OfferingAdjustment, error) {
	return c.owner.ListOfferingAdjustments(ctx, requestID, requestChildID)
}

func (c decisionContract) ListChildOfferings(ctx context.Context, requestID int64) (map[int64]ChildOfferingSet, error) {
	return c.owner.ListChildOfferings(ctx, requestID)
}

func (c decisionContract) ExportPhase(ctx context.Context, phaseID, actorAccountID int64, actorRole, format, childStatusFilter string) (*PhaseExport, error) {
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

func (c decisionContract) ExportStudent(ctx context.Context, studentID, actorAccountID int64, actorRole, format string) (*StudentEnrollmentExport, error) {
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

func (c decisionContract) RecordPhaseExportAudit(ctx context.Context, actorAccountID int64, actorRole string, phase *capability.Phase, format, statusFilter string, requestCount, childCount int) error {
	return c.owner.RecordPhaseExportAudit(ctx, actorAccountID, actorRole, phase, format, statusFilter, requestCount, childCount)
}

func decisionSummaries(values []*capability.DecisionSummary) ([]*RequestSummary, error) {
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
	request, err := intakeRequestValue(value.Request)
	if err != nil {
		return nil, err
	}
	children, err := intakeChildValues(value.Children)
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
		request, err := intakeRequestValue(value.Request)
		if err != nil {
			return nil, err
		}
		children := make([]ExportChildRow, 0, len(value.Children))
		for _, childRow := range value.Children {
			child, err := intakeChildValue(childRow.Child)
			if err != nil {
				return nil, err
			}
			children = append(children, ExportChildRow{Child: child, Offerings: childRow.Offerings})
		}
		rows = append(rows, ExportRequestRow{Request: request, Children: children, Guardians: value.Guardians})
	}
	return rows, nil
}

// CareBookingGates are the Care Plan gates a change-request approval takes
// around its booking- or offering-derived writes.
type CareBookingGates interface {
	LockOfferingDerivedWrites(ctx context.Context) error
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error
}

// NewChangeRequestDecisionApplier applies approved change requests through
// the owner's decision flow and Care Plan's booking gates. A nil bookings
// capability skips the gates.
func NewChangeRequestDecisionApplier(owner capability.ApprovedChildChanges, bookings CareBookingGates) ChangeRequestDecisionApplier {
	return changeRequestDecisions{owner: owner, bookings: bookings}
}

type changeRequestDecisions struct {
	owner    capability.ApprovedChildChanges
	bookings CareBookingGates
}

func (c changeRequestDecisions) LockOfferingDerivedWrites(ctx context.Context) error {
	if c.bookings == nil {
		return nil
	}
	return c.bookings.LockOfferingDerivedWrites(ctx)
}

func (c changeRequestDecisions) ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error {
	if c.bookings == nil {
		return nil
	}
	return c.bookings.ReconcileOfferingPickupForStudents(ctx, studentIDs)
}

func (c changeRequestDecisions) applyApprovedChangeRequestOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error) {
	value, err := c.owner.ApplyChangeRequestOfferings(ctx, input)
	if err != nil {
		return nil, err
	}
	return intakeChildValue(value)
}

func (c changeRequestDecisions) SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*RequestChild, error) {
	snapshot, err := json.Marshal(input.PreviousSnapshot)
	if err != nil {
		return nil, err
	}
	value, err := c.owner.SyncApprovedChildData(ctx, capability.ApprovedChildSync{
		RequestID: input.RequestID, ChildID: input.ChildID, ActorAccountID: input.ActorAccountID,
		ReplaceTargetedData: input.ReplaceTargetedData, PreviousSnapshot: snapshot,
		PreviousRequestGuardians: input.PreviousRequestGuardians,
	})
	if err != nil {
		return nil, err
	}
	return intakeChildValue(value)
}
