//go:build testhooks

package api

import (
	"context"

	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FeedbackTestModule is the Feedback owner the kiosk feedback routes drive;
// the feedback students come from NewRFIDTestModule.
type FeedbackTestModule struct {
	Feedback *feedbackModule.Module
}

// NewFeedbackTestModule composes the Feedback owner over db with its tenant
// settings bound, as the root binds them.
func NewFeedbackTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (FeedbackTestModule, error) {
	feedback, _, err := NewFeedbackTestModuleWithSettings(db, unit)
	return feedback, err
}

// NewFeedbackTestModuleWithSettings is NewFeedbackTestModule together with
// the tenant settings the owner reads, for suites that switch them.
func NewFeedbackTestModuleWithSettings(db transactionruntime.Database, unit tenant.UnitOfWork) (FeedbackTestModule, SettingsTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return FeedbackTestModule{}, SettingsTestModule{}, err
	}
	resolvers := feedbackCompose.NewSettings()
	resolvers.Bind(func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, "feedback.enabled")
	}, func(ctx context.Context) (int, error) {
		return settings.Settings.ResolveInt(ctx, "feedback.data_retention_days")
	})
	feedback, err := feedbackCompose.New(feedbackCompose.Dependencies{
		DB: db.Pool(), Settings: resolvers, Today: feedbackModule.Today, Observe: func(feedbackCompose.Observation) {},
	})
	if err != nil {
		return FeedbackTestModule{}, SettingsTestModule{}, err
	}
	return FeedbackTestModule{Feedback: feedback}, settings, nil
}
