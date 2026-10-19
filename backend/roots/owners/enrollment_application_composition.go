package owners

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// enrollmentSettingsReads is the slice of the settings service the
// Enrollment bindings resolve through.
type EnrollmentSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
	ResolveString(ctx context.Context, key string) (string, error)
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	LockClassCollectionPair(ctx context.Context) error
}

// enrollmentCollectionSettings resolves the grade and class collection
// settings the phase and schema guards check.
type EnrollmentCollectionSettings struct{ Settings EnrollmentSettingsReads }

func (s EnrollmentCollectionSettings) CollectGradeLevel(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectGradeLevel)
}

func (s EnrollmentCollectionSettings) CollectSchoolClass(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyEnrollmentCollectSchoolClass)
}

func (s EnrollmentCollectionSettings) GradeLevelMax(ctx context.Context) (int, error) {
	return s.Settings.ResolveInt(ctx, configModels.KeyEnrollmentGradeLevelMax)
}

func (s EnrollmentCollectionSettings) LockClassCollectionPair(ctx context.Context) error {
	return s.Settings.LockClassCollectionPair(ctx)
}

// enrollmentNotificationSettings resolves the decision notification mode.
type enrollmentNotificationSettings struct {
	settings interface {
		ResolveString(ctx context.Context, key string) (string, error)
	}
}

// NewEnrollmentNotifications composes Enrollment's mail branding and decision
// notifications over the Delivery outbox.
func NewEnrollmentNotifications(modes enrollmentCompose.NotificationModePin, settings interface {
	ResolveString(ctx context.Context, key string) (string, error)
}, outbox MailOutbox, schools enrollmentOwner.SchoolDirectory) enrollmentOwner.Notifications {
	return enrollmentCompose.NewNotifications(enrollmentCompose.NotificationDependencies{
		Modes:       modes,
		Settings:    enrollmentNotificationSettings{settings: settings},
		Outbox:      enrollmentMailOutbox{outbox: outbox},
		Schools:     schools,
		Fingerprint: securityruntime.Fingerprint,
		Branding:    EnrollmentMailBranding,
	})
}

func (s enrollmentNotificationSettings) NotifyPerDecision(ctx context.Context) (string, error) {
	return s.settings.ResolveString(ctx, configModels.KeyEnrollmentNotifyPerDecision)
}

// enrollmentCaptchaSettings resolves tenant overrides or registry defaults.
type EnrollmentCaptchaSettings struct {
	Settings EnrollmentSettingsReads
}

func (s EnrollmentCaptchaSettings) CaptchaRequired(ctx context.Context) (bool, error) {
	return s.Settings.ResolveBool(ctx, configModels.KeyEnrollmentRequireCaptcha)
}

func (s EnrollmentCaptchaSettings) CaptchaSecretKey(ctx context.Context) (string, error) {
	return s.Settings.ResolveString(ctx, configModels.KeyEnrollmentCaptchaSecretKey)
}

func (s EnrollmentCaptchaSettings) CaptchaSiteKey(ctx context.Context) (string, error) {
	return s.Settings.ResolveString(ctx, configModels.KeyEnrollmentCaptchaSiteKey)
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

func (p parentCarePeriods) CarePeriods(ctx context.Context, studentID int64) ([]*parentportal.CarePeriod, error) {
	periods, err := enrollmentOwner.StudentCarePeriodRecords(ctx, p.owner, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]*parentportal.CarePeriod, 0, len(periods))
	for _, period := range periods {
		result = append(result, &parentportal.CarePeriod{
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

func (p parentOfferingHistory) OfferingHistory(ctx context.Context, requestChildID int64) ([]*parentportal.OfferingBooking, error) {
	links, err := enrollmentOwner.OfferingHistoryRecords(ctx, p.owner, requestChildID)
	if err != nil {
		return nil, err
	}
	result := make([]*parentportal.OfferingBooking, 0, len(links))
	for _, link := range links {
		if link == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &parentportal.OfferingBooking{
			CareOfferingID: link.CareOfferingID, SelectedDays: link.SelectedDays,
			ValidFrom: link.ValidFrom, ValidUntil: link.ValidUntil,
		})
	}
	return result, nil
}

// enrollmentMailOutbox puts Enrollment's mails on Delivery's outbox.
type enrollmentMailOutbox struct {
	outbox MailOutbox
}

func (o enrollmentMailOutbox) EnqueueMail(ctx context.Context, mail enrollmentCompose.Mail) error {
	return o.outbox.EnqueueOutbox(ctx, deliveryCompose.MailEnqueueRequest{
		Kind: mail.Kind, Payload: mail.Payload, RelatedEntityType: mail.RelatedEntityType,
		RelatedEntityID: mail.RelatedEntityID, IdempotencyKey: mail.IdempotencyKey,
	})
}

// enrollmentMailRenderers registers Enrollment's renderers by outbox kind.
// Delivery chooses the sender from the mail's school name.
func enrollmentMailRenderers(renderers enrollmentOwner.MailRenderers, defaultFrom email.Email) map[string]deliveryCompose.MailRenderer {
	render := func(renderer enrollmentOwner.MailRenderer) deliveryCompose.MailRenderer {
		return deliveryCompose.MailRendererFunc(func(ctx context.Context, intent *deliveryCompose.MailIntent) (*email.Message, error) {
			payload, err := json.Marshal(intent.Payload)
			if err != nil {
				return nil, fmt.Errorf("%s payload: %w", intent.Kind, err)
			}
			mail, err := renderer(ctx, intent.Kind, payload)
			if err != nil {
				return nil, err
			}
			return &email.Message{
				From: email.NewEmail(deliveryModule.SchoolSender(mail.SenderName, defaultFrom.Name, defaultFrom.Address)), To: email.NewEmail("", mail.Recipient),
				Subject: mail.Subject, Template: mail.Template, Content: mail.Content,
			}, nil
		})
	}
	return map[string]deliveryCompose.MailRenderer{
		enrollmentOwner.MailKindSubmitted:                render(renderers.Submitted),
		enrollmentOwner.MailKindAdminNotification:        render(renderers.AdminNotification),
		enrollmentOwner.MailKindApproved:                 render(renderers.Approved),
		enrollmentOwner.MailKindWaitlisted:               render(renderers.Waitlisted),
		enrollmentOwner.MailKindRejected:                 render(renderers.Rejected),
		enrollmentOwner.MailKindDecisionDigest:           render(renderers.DecisionDigest),
		enrollmentOwner.MailKindChangeRequestSubmitted:   render(renderers.ChangeRequestSubmitted),
		enrollmentOwner.MailKindChangeRequestQuestion:    render(renderers.ChangeRequestQuestion),
		enrollmentOwner.MailKindChangeRequestParentReply: render(renderers.ChangeRequestParentReply),
		enrollmentOwner.MailKindChangeRequestApproved:    render(renderers.ChangeRequestApproved),
		enrollmentOwner.MailKindChangeRequestRejected:    render(renderers.ChangeRequestRejected),
		enrollmentOwner.MailKindRolloverOptIn:            render(renderers.RolloverOptIn),
		enrollmentOwner.MailKindRolloverOptOut:           render(renderers.RolloverOptOut),
	}
}

// EnrollmentMailBranding binds Enrollment's logo port to Delivery.
func EnrollmentMailBranding(baseURL, schoolImageURL string) (string, string) {
	return deliveryCompose.SchoolLogoURL(baseURL, schoolImageURL), deliveryCompose.MotoLogoURL(baseURL)
}
