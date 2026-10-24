package enrollmenthttp_test

import (
	"context"
	"log/slog"

	api "github.com/moto-nrw/project-phoenix/roots/owners"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
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
	TestDecisionBookings           = enrollmentTest.DecisionBookings
	TestMail                       = enrollmentTest.Mail
)

// Owner compositions the suites drive.
var (
	NewTestModule                     = enrollmentCompose.New
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
func NewTestNotifications(modes enrollmentTest.NotificationModePin, settings enrollmentTest.NotificationSettings, outbox testMailOutbox, schools capability.SchoolDirectory) capability.Notifications {
	return enrollmentTest.NewNotifications(enrollmentTest.NotificationDependencies{Branding: api.EnrollmentMailBranding,
		Modes:    modes,
		Settings: settings,
		Outbox:   outbox,
		Schools:  schools,
	})
}

// NewTestFlowNotifications composes the parent mails like
// NewTestNotifications; without an outbox the mails are not recorded.
func NewTestFlowNotifications(modes enrollmentTest.NotificationModePin, settings enrollmentTest.NotificationSettings, outbox testMailOutbox, schools capability.SchoolDirectory) capability.Notifications {
	deps := enrollmentTest.NotificationDependencies{Branding: api.EnrollmentMailBranding, Modes: modes, Settings: settings, Schools: schools}
	if outbox != nil {
		deps.Outbox = outbox
	}
	return enrollmentTest.NewNotifications(deps)
}

// testMailOutbox records or fails Enrollment's parent mails in a suite.
type testMailOutbox interface {
	EnqueueMail(ctx context.Context, mail TestMail) error
}
