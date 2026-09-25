package enrollment

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
)

// The enrollment flow suites of the external test package (#3565) seed and
// read the owner's rows in the decoded values these routes speak, and
// compose the owner's phases, notifications and projections through its test
// support, which only this package's internal tests may import.

// Owner ports and values the flow suites bind their doubles to.
type (
	TestPhaseDependencies          = enrollmentTest.PhaseDependencies
	TestPhaseRecords               = enrollmentTest.PhaseRecords
	TestPhaseExpirySnapshots       = enrollmentTest.PhaseExpirySnapshots
	TestOfferingStudent            = enrollmentTest.OfferingStudent
	TestApprovedOfferingProjection = enrollmentTest.ApprovedOfferingProjection
	TestFormSchemaRecords          = enrollmentTest.FormSchemaRecords
	TestNotificationModePin        = enrollmentTest.NotificationModePin
	TestNotificationSettings       = enrollmentTest.NotificationSettings
	TestDecisions                  = enrollmentTest.Decisions
	TestDecisionPhases             = enrollmentTest.DecisionPhases
	TestDecisionSchemas            = enrollmentTest.DecisionSchemas
	TestCareWithdrawalReconciler   = enrollmentTest.CareWithdrawalReconciler
	TestWeeklyPickupHooks          = enrollmentTest.WeeklyPickupHooks
	TestRolloverCatalogCloner      = enrollmentTest.RolloverCatalogCloner
	TestDirectoryGuardian          = enrollmentTest.DirectoryGuardian
)

// Owner compositions the flow suites drive.
var (
	NewTestModule                     = enrollmentTest.New
	NewTestPhases                     = enrollmentTest.NewPhases
	NewTestPhaseExpirySnapshots       = enrollmentTest.NewPhaseExpirySnapshots
	NewTestPhaseExpiryWarnings        = enrollmentTest.NewPhaseExpiryWarnings
	NewTestApprovedOfferingProjection = enrollmentTest.NewApprovedOfferingProjection
)

// NewTestFlowNotifications composes the parent mails like
// NewTestNotifications; without an outbox the mails are not recorded.
func NewTestFlowNotifications(modes enrollmentTest.NotificationModePin, settings enrollmentTest.NotificationSettings, outbox platformModels.OutboxEnqueuer, schools capability.SchoolDirectory) capability.Notifications {
	deps := enrollmentTest.NotificationDependencies{Modes: modes, Settings: settings, Schools: schools}
	if outbox != nil {
		deps.Outbox = testMailOutbox{outbox: outbox}
	}
	return enrollmentTest.NewNotifications(deps)
}

// InsertOwnerChildForTest persists a decoded child fixture through the owner
// and copies the generated metadata back.
func InsertOwnerChildForTest(ctx context.Context, owner interface {
	InsertChild(context.Context, *capability.RequestChild) error
}, child *RequestChild) error {
	value, err := fixtureChildInput(child)
	if err != nil {
		return err
	}
	if err := owner.InsertChild(ctx, value); err != nil {
		return err
	}
	stored, err := childValue(value)
	if err != nil {
		return err
	}
	*child = *stored
	return nil
}

// ReadOwnerRequestChildrenForTest loads the decoded children of a request.
func ReadOwnerRequestChildrenForTest(ctx context.Context, owner interface {
	ChildrenForRequest(context.Context, int64, bool) ([]*capability.RequestChild, error)
}, requestID int64) ([]*RequestChild, error) {
	values, err := owner.ChildrenForRequest(ctx, requestID, false)
	if err != nil {
		return nil, err
	}
	return childValues(values)
}

// ReadOwnerChildForTest loads a decoded child through the owner query.
func ReadOwnerChildForTest(ctx context.Context, owner interface {
	ChildByID(context.Context, int64) (*capability.RequestChild, error)
}, id int64) (*RequestChild, error) {
	value, err := owner.ChildByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return childValue(value)
}

// UpdateOwnerChildForTest writes decoded child data through the owner.
func UpdateOwnerChildForTest(ctx context.Context, owner interface {
	UpdateChildData(context.Context, *capability.RequestChild) error
}, child *RequestChild) error {
	value, err := fixtureChildInput(child)
	if err != nil {
		return err
	}
	return owner.UpdateChildData(ctx, value)
}

// InsertOwnerRequestForTest persists a request fixture through the owner and
// copies the generated metadata back.
func InsertOwnerRequestForTest(ctx context.Context, owner interface {
	InsertRequest(context.Context, *capability.Request) error
}, request *enrollmentModels.Request) error {
	value, err := fixtureRequestInput(request)
	if err != nil {
		return err
	}
	if err := owner.InsertRequest(ctx, value); err != nil {
		return err
	}
	stored, err := requestValue(value)
	if err != nil {
		return err
	}
	*request = *stored
	return nil
}

// ReadOwnerRequestForTest loads a decoded request through the owner query.
func ReadOwnerRequestForTest(ctx context.Context, owner interface {
	RequestByID(context.Context, int64, bool) (*capability.Request, error)
}, id int64) (*enrollmentModels.Request, error) {
	value, err := owner.RequestByID(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return requestValue(value)
}

// UpdateOwnerRequestGuardianForTest writes a request fixture's guardian
// fields through the owner.
func UpdateOwnerRequestGuardianForTest(ctx context.Context, owner interface {
	UpdateRequestGuardian(context.Context, *capability.Request, bool) error
}, request *enrollmentModels.Request, includeEmail bool) error {
	value, err := fixtureRequestInput(request)
	if err != nil {
		return err
	}
	return owner.UpdateRequestGuardian(ctx, value, includeEmail)
}

// InsertOwnerPhaseForTest persists a phase fixture and returns generated
// metadata.
func InsertOwnerPhaseForTest(ctx context.Context, owner interface {
	InsertPhase(context.Context, *capability.Phase) error
}, phase *capability.Phase) error {
	return owner.InsertPhase(ctx, phase)
}

// OwnerPhaseForTest copies an owner phase for isolated fixture updates.
func OwnerPhaseForTest(phase *capability.Phase) *capability.Phase {
	if phase == nil {
		return nil
	}
	value := *phase
	return &value
}

// WriteSelectionDateForTest is the clamp the offering reads and writes
// share: a day before the service window reads its first day, a day after
// it its last day (#2185).
func WriteSelectionDateForTest(phase *capability.Phase, today timezone.Date) timezone.Date {
	if phase == nil {
		return today
	}
	if today.Before(timezone.Date(phase.ServiceStartDate)) {
		return timezone.Date(phase.ServiceStartDate)
	}
	if today.After(timezone.Date(phase.ServiceEndDate)) {
		return timezone.Date(phase.ServiceEndDate)
	}
	return today
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

// ChangeRequestDecisionApplier is the admin correction's sync of an
// approved child onto its student, decoded.
type ChangeRequestDecisionApplier interface {
	SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*RequestChild, error)
}

// ChangeRequestApplierForTest syncs approved children through the owner's
// decision flow, the way the change requests do.
func ChangeRequestApplierForTest(owner capability.ApprovedChildChanges) ChangeRequestDecisionApplier {
	return changeRequestApplier{owner: owner}
}

type changeRequestApplier struct {
	owner capability.ApprovedChildChanges
}

func (a changeRequestApplier) SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*RequestChild, error) {
	snapshot, err := json.Marshal(input.PreviousSnapshot)
	if err != nil {
		return nil, err
	}
	value, err := a.owner.SyncApprovedChildData(ctx, capability.ApprovedChildSync{
		RequestID: input.RequestID, ChildID: input.ChildID, ActorAccountID: input.ActorAccountID,
		ReplaceTargetedData: input.ReplaceTargetedData, PreviousSnapshot: snapshot,
		PreviousRequestGuardians: input.PreviousRequestGuardians,
	})
	if err != nil {
		return nil, err
	}
	return childValue(value)
}

// fixtureChildInput encodes a decoded child the way the retained service
// stored its fixtures: an absent answer map is stored as JSON null.
func fixtureChildInput(child *RequestChild) (*capability.RequestChild, error) {
	value, err := childInput(child)
	if err != nil || value == nil {
		return value, err
	}
	if value.CustomData == nil {
		value.CustomData = json.RawMessage("null")
	}
	return value, nil
}

// fixtureRequestInput encodes every JSON column of a request fixture.
func fixtureRequestInput(r *enrollmentModels.Request) (*capability.Request, error) {
	if r == nil {
		return nil, nil
	}
	status, err := requestStatusInput(r, nil)
	if err != nil {
		return nil, err
	}
	value := status.Request
	for _, field := range []struct {
		from any
		into *json.RawMessage
	}{
		{r.ConsentFlags, &value.ConsentFlags},
		{r.LegalBlocksSnapshot, &value.LegalBlocksSnapshot},
		{r.CustomData, &value.CustomData},
		{r.SourceMetadata, &value.SourceMetadata},
	} {
		encoded, err := json.Marshal(field.from)
		if err != nil {
			return nil, err
		}
		*field.into = encoded
	}
	return value, nil
}
