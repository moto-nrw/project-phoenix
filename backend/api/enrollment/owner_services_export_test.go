package enrollment

import (
	"context"
	"log/slog"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
)

// The router suites of the external test package build Enrollment's form
// schemas and parent notifications through these helpers: only this
// package's internal tests may compose the owner through its test support.

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

type testMailOutbox struct{ outbox platformModels.OutboxEnqueuer }

func (o testMailOutbox) EnqueueMail(ctx context.Context, mail enrollmentTest.Mail) error {
	return o.outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind: mail.Kind, Payload: mail.Payload, RelatedEntityType: mail.RelatedEntityType,
		RelatedEntityID: mail.RelatedEntityID, IdempotencyKey: mail.IdempotencyKey,
	})
}

// The decision-flow ports the external router suites bind their doubles to
// (#3564).
type (
	TestDecisionChildren          = enrollmentTest.DecisionChildren
	TestDecisionGuardianAccess    = enrollmentTest.DecisionGuardianAccess
	TestDecisionStudentEnrollment = enrollmentTest.DecisionStudentEnrollment
	TestDecisionBookings          = enrollmentTest.DecisionBookings
)
