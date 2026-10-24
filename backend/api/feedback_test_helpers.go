//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	feedbackCompose "github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// FeedbackTestModule is the Feedback owner the kiosk feedback routes drive;
// the feedback students come from NewRFIDTestModule.
type FeedbackTestModule struct {
	Feedback feedbackModule.Capability
}

// NewFeedbackTestModule composes the Feedback owner over db with its tenant
// settings bound, as the root binds them.
func NewFeedbackTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (FeedbackTestModule, error) {
	feedback, _, err := NewFeedbackTestModuleWithSettings(db, unit)
	return feedback, err
}

// NewFeedbackTestModuleWithSettings is NewFeedbackTestModule together with
// the tenant settings the owner reads, for suites that switch them.
func NewFeedbackTestModuleWithSettings(db transactionruntime.Database, unit tenant.UnitOfWork) (FeedbackTestModule, owners.SettingsTestModule, error) {
	settings, err := owners.NewSettingsTestModule(db, unit)
	if err != nil {
		return FeedbackTestModule{}, owners.SettingsTestModule{}, err
	}
	resolvers, err := feedbackCompose.NewBoundSettings(func(ctx context.Context) (bool, error) {
		return settings.Settings.ResolveBool(ctx, "feedback.enabled")
	}, func(ctx context.Context) (int, error) {
		return settings.Settings.ResolveInt(ctx, "feedback.data_retention_days")
	})
	if err != nil {
		return FeedbackTestModule{}, owners.SettingsTestModule{}, err
	}
	feedback, err := feedbackCompose.New(feedbackCompose.Dependencies{
		DB: db.Pool(), Settings: resolvers, Today: feedbackModule.Today, Observe: func(feedbackCompose.Observation) {},
	})
	if err != nil {
		return FeedbackTestModule{}, owners.SettingsTestModule{}, err
	}
	return FeedbackTestModule{Feedback: feedback}, settings, nil
}
