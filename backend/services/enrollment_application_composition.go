package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/services/config"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// The bindings below connect Enrollment's application services (#3562) to
// the Settings Platform, the Delivery outbox and Care Plan's offering rows.

// enrollmentSettingsReads is the slice of the settings service the
// Enrollment bindings resolve through.
type enrollmentSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	LockClassCollectionPair(ctx context.Context) error
}

// enrollmentCollectionSettings resolves the grade and class collection
// settings the phase and schema guards check.
type enrollmentCollectionSettings struct{ settings enrollmentSettingsReads }

func (s enrollmentCollectionSettings) CollectGradeLevel(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectGradeLevel)
}

func (s enrollmentCollectionSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectSchoolClass)
}

func (s enrollmentCollectionSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

func (s enrollmentCollectionSettings) LockClassCollectionPair(ctx context.Context) error {
	return s.settings.LockClassCollectionPair(ctx)
}

// enrollmentNotificationSettings resolves the decision notification mode.
type enrollmentNotificationSettings struct {
	settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

// newEnrollmentNotifications composes Enrollment's mail branding and decision
// notifications over the Delivery outbox.
func newEnrollmentNotifications(modes enrollmentCompose.NotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, outbox platformModels.OutboxEnqueuer, schools enrollmentOwner.SchoolDirectory) enrollmentOwner.Notifications {
	return enrollmentCompose.NewNotifications(enrollmentCompose.NotificationDependencies{
		Modes:       modes,
		Settings:    enrollmentNotificationSettings{settings: settings},
		Outbox:      enrollmentMailOutbox{outbox: outbox},
		Schools:     schools,
		Fingerprint: securityruntime.Fingerprint,
	})
}

func (s enrollmentNotificationSettings) NotifyPerDecision(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModels.KeyEnrollmentNotifyPerDecision)
}

// enrollmentCaptchaSettings resolves the captcha settings: the tenant
// override when one exists, else the deployment's captcha configuration.
type enrollmentCaptchaSettings struct {
	settings       enrollmentSettingsReads
	requireCaptcha bool
	secretKey      string
	siteKey        string
}

func (s enrollmentCaptchaSettings) CaptchaRequired(ctx context.Context) bool {
	return config.ResolveBoolOrDefault(ctx, s.settings, configModels.KeyEnrollmentRequireCaptcha, s.requireCaptcha, nil)
}

func (s enrollmentCaptchaSettings) CaptchaSecretKey(ctx context.Context) string {
	return config.ResolveStringOrDefault(ctx, s.settings, configModels.KeyEnrollmentCaptchaSecretKey, s.secretKey, nil)
}

func (s enrollmentCaptchaSettings) CaptchaSiteKey(ctx context.Context) string {
	return config.ResolveStringOrDefault(ctx, s.settings, configModels.KeyEnrollmentCaptchaSiteKey, s.siteKey, nil)
}

// enrollmentPhaseOfferings reads the Care Plan offerings of a phase.
type enrollmentPhaseOfferings struct {
	carePlan interface {
		ListCareOfferings(context.Context, careplan.CareOfferingFilter) ([]careplan.CareOffering, error)
		CountCareOfferingsByPhase(context.Context, int64) (int, error)
	}
}

func (o enrollmentPhaseOfferings) OfferingIDsForPhase(ctx context.Context, phaseID int64) ([]int64, error) {
	offerings, err := o.carePlan.ListCareOfferings(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog})
	if err != nil {
		return nil, fmt.Errorf("failed to list care offerings by phase: %w", err)
	}
	ids := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		ids = append(ids, offering.ID)
	}
	return ids, nil
}

func (o enrollmentPhaseOfferings) CountOfferingsForPhase(ctx context.Context, phaseID int64) (int, error) {
	count, err := o.carePlan.CountCareOfferingsByPhase(ctx, phaseID)
	if err != nil {
		return 0, fmt.Errorf("failed to count care offerings by phase: %w", err)
	}
	return count, nil
}

// parentCarePeriods answers the parent portal's care periods from
// Enrollment.
type parentCarePeriods struct {
	owner interface {
		StudentCarePeriods(context.Context, int64) ([]*enrollmentOwner.StudentCarePeriod, error)
	}
}

func (p parentCarePeriods) CarePeriods(ctx context.Context, studentID int64) ([]*parentportalcompose.CarePeriod, error) {
	periods, err := enrollmentOwner.StudentCarePeriodRecords(ctx, p.owner, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]*parentportalcompose.CarePeriod, 0, len(periods))
	for _, period := range periods {
		result = append(result, &parentportalcompose.CarePeriod{
			RequestChildID: period.RequestChildID, RequestID: period.RequestID, PhaseID: period.PhaseID,
			PhaseName: period.PhaseName, ServiceStartDate: period.ServiceStartDate, ServiceEndDate: period.ServiceEndDate,
		})
	}
	return result, nil
}

// parentOfferingHistory answers the parent portal's booked offerings from
// Enrollment.
type parentOfferingHistory struct {
	owner interface {
		RequestChildOfferingHistory(context.Context, int64) ([]*enrollmentOwner.RequestChildOffering, error)
	}
}

func (p parentOfferingHistory) OfferingHistory(ctx context.Context, requestChildID int64) ([]*parentportalcompose.OfferingBooking, error) {
	links, err := enrollmentOwner.OfferingHistoryRecords(ctx, p.owner, requestChildID)
	if err != nil {
		return nil, err
	}
	result := make([]*parentportalcompose.OfferingBooking, 0, len(links))
	for _, link := range links {
		if link == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &parentportalcompose.OfferingBooking{
			CareOfferingID: link.CareOfferingID, SelectedDays: link.SelectedDays,
			ValidFrom: link.ValidFrom, ValidUntil: link.ValidUntil,
		})
	}
	return result, nil
}

// enrollmentMailOutbox puts Enrollment's decision mails on the platform
// outbox the retained enrollment flows enqueue through.
type enrollmentMailOutbox struct {
	outbox platformModels.OutboxEnqueuer
}

func (o enrollmentMailOutbox) EnqueueMail(ctx context.Context, mail enrollmentCompose.Mail) error {
	return o.outbox.EnqueueOutbox(ctx, platformModels.OutboxEnqueueRequest{
		Kind: mail.Kind, Payload: mail.Payload, RelatedEntityType: mail.RelatedEntityType,
		RelatedEntityID: mail.RelatedEntityID, IdempotencyKey: mail.IdempotencyKey,
	})
}

// enrollmentMailRenderers registers Enrollment's renderers by outbox kind.
// A mail without a school name keeps the default sender; otherwise it goes
// out under the school's name from the default address.
func enrollmentMailRenderers(renderers enrollmentOwner.MailRenderers, defaultFrom email.Email) map[string]emailoutbox.Renderer {
	render := func(renderer enrollmentOwner.MailRenderer) emailoutbox.Renderer {
		return emailoutbox.RendererFunc(func(ctx context.Context, intent *emailoutbox.Intent) (*email.Message, error) {
			payload, err := json.Marshal(intent.Payload)
			if err != nil {
				return nil, fmt.Errorf("%s payload: %w", intent.Kind, err)
			}
			mail, err := renderer(ctx, intent.Kind, payload)
			if err != nil {
				return nil, err
			}
			from := defaultFrom
			if mail.SenderName != "" {
				from = email.NewEmail(mail.SenderName, defaultFrom.Address)
			}
			return &email.Message{
				From: from, To: email.NewEmail("", mail.Recipient),
				Subject: mail.Subject, Template: mail.Template, Content: mail.Content,
			}, nil
		})
	}
	return map[string]emailoutbox.Renderer{
		platformModels.EmailKindEnrollmentSubmitted:                render(renderers.Submitted),
		platformModels.EmailKindEnrollmentAdminNotify:              render(renderers.AdminNotification),
		platformModels.EmailKindEnrollmentApproved:                 render(renderers.Approved),
		platformModels.EmailKindEnrollmentWaitlisted:               render(renderers.Waitlisted),
		platformModels.EmailKindEnrollmentRejected:                 render(renderers.Rejected),
		platformModels.EmailKindEnrollmentDecisionDigest:           render(renderers.DecisionDigest),
		platformModels.EmailKindEnrollmentChangeRequestSubmitted:   render(renderers.ChangeRequestSubmitted),
		platformModels.EmailKindEnrollmentChangeRequestQuestion:    render(renderers.ChangeRequestQuestion),
		platformModels.EmailKindEnrollmentChangeRequestParentReply: render(renderers.ChangeRequestParentReply),
		platformModels.EmailKindEnrollmentChangeRequestApproved:    render(renderers.ChangeRequestApproved),
		platformModels.EmailKindEnrollmentChangeRequestRejected:    render(renderers.ChangeRequestRejected),
		platformModels.EmailKindEnrollmentRolloverOptIn:            render(renderers.RolloverOptIn),
		platformModels.EmailKindEnrollmentRolloverOptOut:           render(renderers.RolloverOptOut),
	}
}
