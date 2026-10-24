package compose

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// Consumer-owned ports of the Stammdaten decision and the cross-kind parent
// request commands (#3354), re-exported so the composition root can
// implement them without reaching into the module.
type (
	MasterDataRecords        = ports.MasterDataRecords
	MasterDataFieldReviews   = careplan.MasterDataFieldReviews
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
		Hooks: TenantHooks{}, Today: deps.Today, Logger: deps.Logger, Messenger: deps.Messenger,
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
	excused := application.ExcusedParentRequests{Requests: deps.Excused}
	return application.NewParentRequestCoordinator(application.ParentRequestCoordinatorDependencies{
		Rights: deps.Rights, Rollback: TenantHooks{}, MasterData: deps.MasterData, Excused: excused,
		MasterDataConflicts: deps.MasterData, ExcusedConflicts: excused,
		CareConflicts: deps.Care, OfferingConflicts: deps.Offering, Ledger: deps.Ledger,
	})
}
