package enrollmenthttp_test

import (
	"context"
	"log/slog"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
)

// The router and flow suites (#3564, #3565) compose Enrollment's phases,
// form schemas, decisions, notifications and projections through the owner's
// test support and bind their doubles to its ports.

// Owner ports and values the suites bind their doubles to.
type (
	TestPhaseDependencies          = enrollmentTest.PhaseDependencies
	TestPhaseRecords               = enrollmentTest.PhaseRecords
	TestPhaseExpirySnapshots       = enrollmentTest.PhaseExpirySnapshots
	TestOfferingStudent            = enrollmentTest.OfferingStudent
	TestApprovedOfferingProjection = enrollmentTest.ApprovedOfferingProjection
	TestFormSchemaRecords          = enrollmentTest.FormSchemaRecords
	TestNotificationModePin        = enrollmentTest.NotificationModePin
	TestDecisions                  = enrollmentTest.Decisions
	TestDecisionPhases             = enrollmentTest.DecisionPhases
	TestDecisionSchemas            = enrollmentTest.DecisionSchemas
	TestCareWithdrawalReconciler   = enrollmentTest.CareWithdrawalReconciler
	TestWeeklyPickupHooks          = enrollmentTest.WeeklyPickupHooks
	TestRolloverCatalogCloner      = enrollmentTest.RolloverCatalogCloner
	TestDirectoryGuardian          = enrollmentTest.DirectoryGuardian
	TestDecisionChildren           = enrollmentTest.DecisionChildren
	TestDecisionGuardianAccess     = enrollmentTest.DecisionGuardianAccess
	TestDecisionStudentEnrollment  = enrollmentTest.DecisionStudentEnrollment
	TestDecisionBookings           = enrollmentTest.DecisionBookings
)

// Owner compositions the suites drive.
var (
	NewTestModule                     = enrollmentTest.New
	NewTestPhases                     = enrollmentTest.NewPhases
	NewTestPhaseExpirySnapshots       = enrollmentTest.NewPhaseExpirySnapshots
	NewTestPhaseExpiryWarnings        = enrollmentTest.NewPhaseExpiryWarnings
	NewTestApprovedOfferingProjection = enrollmentTest.NewApprovedOfferingProjection
)

// NewTestFormSchemas composes form-schema publishing over the owner's
// records.
func NewTestFormSchemas(records enrollmentTest.FormSchemaRecords) capability.FormSchemaAdministration {
	return enrollmentTest.NewFormSchemas(records, nil, slog.Default())
}

// NewTestNotifications composes the parent mails the way the root does: the
// owner pins the mode, the suite's settings choose it, the suite's outbox
// records the mails and the school directory brands them.
func NewTestNotifications(modes enrollmentTest.NotificationModePin, settings enrollmentTest.NotificationSettings, outbox platformModels.OutboxEnqueuer, schools capability.SchoolDirectory) capability.Notifications {
	return enrollmentTest.NewNotifications(enrollmentTest.NotificationDependencies{
		Modes:    modes,
		Settings: settings,
		Outbox:   testMailOutbox{outbox: outbox},
		Schools:  schools,
	})
}

// NewTestFlowNotifications composes the parent mails like
// NewTestNotifications; without an outbox the mails are not recorded.
func NewTestFlowNotifications(modes enrollmentTest.NotificationModePin, settings enrollmentTest.NotificationSettings, outbox platformModels.OutboxEnqueuer, schools capability.SchoolDirectory) capability.Notifications {
	deps := enrollmentTest.NotificationDependencies{Modes: modes, Settings: settings, Schools: schools}
	if outbox != nil {
		deps.Outbox = testMailOutbox{outbox: outbox}
	}
	return enrollmentTest.NewNotifications(deps)
}

type testMailOutbox struct{ outbox platformModels.OutboxEnqueuer }

func (o testMailOutbox) EnqueueMail(ctx context.Context, mail enrollmentTest.Mail) error {
	return o.outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind: mail.Kind, Payload: mail.Payload, RelatedEntityType: mail.RelatedEntityType,
		RelatedEntityID: mail.RelatedEntityID, IdempotencyKey: mail.IdempotencyKey,
	})
}
