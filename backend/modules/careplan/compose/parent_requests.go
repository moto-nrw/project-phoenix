package compose

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Consumer-owned ports of the Stammdaten decision and the cross-kind parent
// request commands (#3354), re-exported so the composition root can
// implement them without reaching into the module.
type (
	MasterDataRecords        = ports.MasterDataRecords
	MasterDataFieldReviews   = ports.MasterDataFieldReviews
	MasterDataStudent        = ports.MasterDataStudent
	MasterDataPerson         = ports.MasterDataPerson
	MasterDataStudentChange  = ports.MasterDataStudentChange
	MasterDataStudentWrite   = ports.MasterDataStudentWrite
	MasterDataPersonChange   = ports.MasterDataPersonChange
	MasterDataWriteError     = ports.MasterDataWriteError
	MasterDataBroadcaster    = ports.MasterDataBroadcaster
	DepartureModes           = domain.DepartureModes
	ParentRequestRights      = ports.ParentRequestRights
	ParentRequestRightsCheck = ports.ParentRequestRightsResolver
	ParentRequestConflicts   = ports.ConflictPort
	ConflictCandidate        = ports.ConflictCandidate
	ConflictDecision         = ports.ConflictDecision
	StaffValueWrite          = ports.StaffValueWrite
)

// Steps of a People Directory write a MasterDataWriteError names.
const (
	MasterDataWriteLoad   = ports.MasterDataWriteLoad
	MasterDataWriteUpdate = ports.MasterDataWriteUpdate
	MasterDataWriteAudit  = ports.MasterDataWriteAudit
)

// MasterDataDecisions is the composed Stammdaten decision behind the
// masterdatarequests.Decisions contract.
type MasterDataDecisions = application.MasterDataDecisions

// MasterDataDecisionDependencies wires the Stammdaten decision. CarePlan,
// People, Records, Scope and Today are required; the effect ports may be nil
// and the decision then skips that effect.
type MasterDataDecisionDependencies struct {
	CarePlan    careplan.Capability
	People      MasterDataFieldReviews
	Records     MasterDataRecords
	Scope       ReviewScopeResolver
	Today       func() careplan.Date
	Logger      *slog.Logger
	Messenger   RequestMessenger
	Broadcaster MasterDataBroadcaster
	Ledger      RequestLedger
	Shares      ShareVisibility
}

// NewMasterDataDecisions composes the decision over Care Plan's own request
// rows and the tenant runtime's transaction hooks.
func NewMasterDataDecisions(deps MasterDataDecisionDependencies) (*MasterDataDecisions, error) {
	if deps.CarePlan == nil {
		return nil, errors.New("care plan master data decisions: care plan capability is required")
	}
	return application.NewMasterDataDecisions(application.MasterDataDecisionDependencies{
		Requests: deps.CarePlan, People: deps.People, Records: deps.Records, Scope: deps.Scope,
		Hooks: tenantHooks{}, Today: deps.Today, Logger: deps.Logger, Messenger: deps.Messenger,
		Broadcaster: deps.Broadcaster, Ledger: deps.Ledger, Shares: deps.Shares,
	})
}

// ParentRequestMasterData is the Stammdaten queue's part in the cross-kind
// commands: its bulk candidates, its request and student locks, its decision
// and its conflict payload. *MasterDataDecisions satisfies it.
type ParentRequestMasterData interface {
	ports.MasterDataBulkPort
	ports.ConflictPort
}

var _ ParentRequestMasterData = (*MasterDataDecisions)(nil)

// ParentRequestCoordinatorDependencies wires the cross-kind bulk approval and
// conflict resolution over the four request queues. Every queue is required:
// a missing one would answer conflict_kind_unsupported instead of deciding.
type ParentRequestCoordinatorDependencies struct {
	Rights     ParentRequestRightsCheck
	MasterData ParentRequestMasterData
	Excused    careplan.ExcusedAbsenceRequests
	// Care resolves both the weekly plan and the single-day pickup change.
	Care     ParentRequestConflicts
	Offering ParentRequestConflicts
	// Ledger records the staff-entered result of a resolve.
	Ledger RequestLedger
}

// NewParentRequestCoordinator composes the cross-kind commands.
func NewParentRequestCoordinator(deps ParentRequestCoordinatorDependencies) (parentrequests.Coordinator, error) {
	if deps.Rights == nil || deps.MasterData == nil || deps.Excused == nil || deps.Care == nil || deps.Offering == nil {
		return nil, errors.New("care plan parent request coordinator: rights and all four request queues are required")
	}
	excused := excusedParentRequests{requests: deps.Excused}
	return application.NewParentRequestCoordinator(application.ParentRequestCoordinatorDependencies{
		Rights: deps.Rights, Rollback: tenantHooks{}, MasterData: deps.MasterData, Excused: excused,
		MasterDataConflicts: deps.MasterData, ExcusedConflicts: excused,
		CareConflicts: deps.Care, OfferingConflicts: deps.Offering, Ledger: deps.Ledger,
	})
}

// MarkRollback asks the ambient tenant transaction to roll back.
func (tenantHooks) MarkRollback(ctx context.Context) { tenant.MarkRollback(ctx) }

// excusedParentRequests presents the excused-absence workflow to the
// cross-kind commands in the coordinator's vocabulary.
type excusedParentRequests struct {
	requests careplan.ExcusedAbsenceRequests
}

func (p excusedParentRequests) GetExcusedBulkCandidate(ctx context.Context, requestID int64) (*ports.ExcusedBulkCandidate, error) {
	candidate, err := p.requests.GetExcusedBulkCandidate(ctx, requestID)
	if err != nil || candidate == nil {
		return nil, excusedParentRequestError(err)
	}
	return &ports.ExcusedBulkCandidate{ID: candidate.ID, StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt, Eligible: candidate.Eligible}, nil
}

func (p excusedParentRequests) LockExcusedBulkRequest(ctx context.Context, requestID int64) error {
	return excusedParentRequestError(p.requests.LockExcusedBulkRequest(ctx, requestID))
}

func (p excusedParentRequests) ApproveExcusedBulk(ctx context.Context, requestID int64, reason string, reviewerID int64, expectedVersion string) error {
	return excusedParentRequestError(p.requests.ApproveExcusedBulk(ctx, requestID, reason, reviewerID, expectedVersion))
}

func (p excusedParentRequests) ConflictCandidate(ctx context.Context, requestID int64) (*ports.ConflictCandidate, error) {
	candidate, err := p.requests.ConflictCandidate(ctx, requestID)
	if err != nil {
		return nil, excusedParentRequestError(err)
	}
	return &ports.ConflictCandidate{StudentID: candidate.StudentID, UpdatedAt: candidate.UpdatedAt}, nil
}

func (p excusedParentRequests) LockConflictRequest(ctx context.Context, requestID int64) error {
	return excusedParentRequestError(p.requests.LockConflictRequest(ctx, requestID))
}

func (p excusedParentRequests) DecideConflictRequest(ctx context.Context, decision ports.ConflictDecision) error {
	return excusedParentRequestError(p.requests.DecideConflictRequest(ctx, careplan.ExcusedConflictDecision{
		RequestID: decision.RequestID, Approve: decision.Approve, Reason: decision.Reason,
		ReviewerID: decision.ReviewerID, ExpectedVersion: decision.ExpectedVersion,
	}))
}

// WriteStaffValue reads {"value": "<status>"} from the resolve payload;
// anything else is the workflow's invalid-status sentinel, never a 500.
func (p excusedParentRequests) WriteStaffValue(ctx context.Context, write ports.StaffValueWrite) error {
	status, ok := write.Value["value"].(string)
	if !ok {
		return careplan.ErrAbsenceRequestInvalidStatus
	}
	return excusedParentRequestError(p.requests.WriteStaffValue(ctx, careplan.ExcusedStaffValueWrite{
		StudentID: write.StudentID, RequestIDs: write.RequestIDs, Reason: write.Reason, Status: status,
	}))
}

// excusedParentRequestError translates the workflow's sentinels into the ones
// the cross-kind commands and their routes match on. Domain-specific
// sentinels pass through unchanged.
func excusedParentRequestError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, careplan.ErrExcusedRequestNotFound):
		return absencerecords.ErrExcusedRequestNotFound
	case errors.Is(err, careplan.ErrExcusedRequestNotPending):
		return absencerecords.ErrExcusedRequestNotPending
	case errors.Is(err, careplan.ErrExcusedRequestNotDecided):
		return absencerecords.ErrExcusedRequestNotDecided
	default:
		return parentRequestLifecycleError(err)
	}
}

// parentRequestLifecycleError maps Care Plan's cross-kind lifecycle
// sentinels onto the parent-request contract.
func parentRequestLifecycleError(err error) error {
	for _, pair := range [][2]error{
		{careplan.ErrParentRequestStale, parentrequests.ErrStale},
		{careplan.ErrParentRequestDecisionRace, parentrequests.ErrDecisionRace},
		{careplan.ErrParentRequestReasonRequired, parentrequests.ErrReasonRequired},
		{careplan.ErrParentRequestPast, parentrequests.ErrPast},
		{careplan.ErrParentRequestNotPast, parentrequests.ErrNotPast},
		{careplan.ErrParentRequestNotDecided, parentrequests.ErrNotDecided},
		{careplan.ErrParentRequestCorrectionUnsupported, parentrequests.ErrCorrectionUnsupported},
		{careplan.ErrStaffValueUnsupported, parentrequests.ErrStaffValueUnsupported},
	} {
		if errors.Is(err, pair[0]) {
			return pair[1]
		}
	}
	return err
}
