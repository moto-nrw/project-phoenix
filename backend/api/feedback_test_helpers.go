//go:build testhooks

package api

import (
	"context"

	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FeedbackTestModule is the Feedback owner with the settings it reads and the
// RFID graph the kiosk feedback routes run beside.
type FeedbackTestModule struct {
	RFIDTestModule
	SettingsTestModule
	Feedback *feedbackModule.Module
}

// NewFeedbackTestModule composes the Feedback owner over db with its tenant
// settings bound, as the root binds them.
func NewFeedbackTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (FeedbackTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return FeedbackTestModule{}, err
	}
	identity, err := NewRFIDTestModule(db)
	if err != nil {
		return FeedbackTestModule{}, err
	}
	resolvers, err := feedbackCompose.NewBoundSettings(func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, "feedback.enabled")
	}, func(ctx context.Context) (int, error) {
		return settings.Settings.ResolveInt(ctx, "feedback.data_retention_days")
	})
	if err != nil {
		return FeedbackTestModule{}, err
	}
	feedback, err := feedbackCompose.New(feedbackCompose.Dependencies{
		DB: db.Pool(), Settings: resolvers, Today: feedbackModule.Today, Observe: func(feedbackCompose.Observation) {},
	})
	if err != nil {
		return FeedbackTestModule{}, err
	}
	return FeedbackTestModule{RFIDTestModule: identity, SettingsTestModule: settings, Feedback: feedback}, nil
}
