package settings

import (
	"context"
	"fmt"

	configSvc "github.com/moto-nrw/project-phoenix/modules/settings/internal/application"
	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// EnrollmentIntakeSource is the resolver slice the enrollment intake values
// read through.
type EnrollmentIntakeSource interface {
	HasTenantOverride(ctx context.Context, key string) (bool, error)
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// EnrollmentIntakeValues are the settings of the enrollment intake. The
// switches use the resolved override or registry default. On read failures
// the intake stays closed and no admin is mailed. Edit permission and status
// token lifetime reads propagate errors.
type EnrollmentIntakeValues struct{ source EnrollmentIntakeSource }

// NewEnrollmentIntakeValues binds the values to the resolver.
func NewEnrollmentIntakeValues(source EnrollmentIntakeSource) EnrollmentIntakeValues {
	return EnrollmentIntakeValues{source: source}
}

// EnrollmentEnabled opens the public intake only while the school booked
// the module Online-Anmeldung and switched enrollment.enabled on (#4893): the
// school switch fine-tunes inside the module, it never opens a cancelled one.
func (v EnrollmentIntakeValues) EnrollmentEnabled(ctx context.Context) bool {
	return v.OnlineAnmeldungBooked(ctx) &&
		configSvc.ResolveBoolOrDefault(ctx, v.source, configModel.KeyEnrollmentEnabled, false, nil)
}

// OnlineAnmeldungBooked reports whether the school booked the module
// Online-Anmeldung; a read failure counts as not booked, so the intake and
// the status links stay closed.
func (v EnrollmentIntakeValues) OnlineAnmeldungBooked(ctx context.Context) bool {
	return configSvc.ResolveBoolOrDefault(ctx, v.source, configModel.KeyModuleOnlineAnmeldung, false, nil)
}

func (v EnrollmentIntakeValues) AllowSubmissionEdit(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyEnrollmentAllowSubmissionEdit)
}

func (v EnrollmentIntakeValues) StatusTokenTTLDays(ctx context.Context) (int, error) {
	return v.source.ResolveInt(ctx, configModel.KeyEnrollmentStatusTokenTTLDays)
}

func (v EnrollmentIntakeValues) AdminNotificationEmails(ctx context.Context) string {
	return configSvc.ResolveStringOrDefault(ctx, v.source, configModel.KeyEnrollmentNotificationEmails, "", nil)
}

func (v EnrollmentIntakeValues) DuplicateHandling(ctx context.Context) (string, error) {
	return v.source.ResolveString(ctx, configModel.KeyEnrollmentDuplicateHandling)
}

func (v EnrollmentIntakeValues) CollectGradeLevel(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyEnrollmentCollectGradeLevel)
}

func (v EnrollmentIntakeValues) CollectSchoolClass(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyEnrollmentCollectSchoolClass)
}

func (v EnrollmentIntakeValues) CareOfferingsEnabled(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
}

func (v EnrollmentIntakeValues) GradeLevelMax(ctx context.Context) (int, error) {
	return v.source.ResolveInt(ctx, configModel.KeyEnrollmentGradeLevelMax)
}

// ChangeRequestMailsEnabled fails closed: a read failure sends no change
// request mail.
func (v EnrollmentIntakeValues) ChangeRequestMailsEnabled(ctx context.Context) bool {
	enabled, err := v.source.ResolveBool(ctx, configModel.KeyEnrollmentChangeRequestEmailNotificationsEnabled)
	return err == nil && enabled
}

// EnrollmentLegal are the legal texts and block toggles of the intake form.
type EnrollmentLegal struct {
	AGB                 string
	AGBDocumentURL      string
	AGBDisplayMode      string
	DSGVO               string
	EmailContact        string
	Photo               string
	TermsEnabled        bool
	DSGVOEnabled        bool
	EmailContactEnabled bool
	PhotoEnabled        bool
}

// enrollmentLegalKeys are read in one snapshot when the resolver supports it.
var enrollmentLegalKeys = []string{
	configModel.KeyEnrollmentLegalAGBText,
	configModel.KeyEnrollmentLegalAGBDocumentURL,
	configModel.KeyEnrollmentLegalAGBDisplayMode,
	configModel.KeyEnrollmentLegalDSGVOText,
	configModel.KeyEnrollmentLegalEmailContactText,
	configModel.KeyEnrollmentLegalPhotoText,
	configModel.KeyEnrollmentLegalTermsEnabled,
	configModel.KeyEnrollmentLegalDSGVOEnabled,
	configModel.KeyEnrollmentLegalEmailContactEnabled,
	configModel.KeyEnrollmentLegalPhotoEnabled,
}

// Legal resolves the legal texts and toggles. No env fallback: a plain
// resolve (tenant override, then the registry default "") is correct, and a
// failure is propagated because the texts drive legal blocks.
func (v EnrollmentIntakeValues) Legal(ctx context.Context) (EnrollmentLegal, error) {
	if batch, ok := v.source.(interface {
		ResolveMany(context.Context, []string) (*configModel.SettingsSnapshot, error)
	}); ok {
		snapshot, err := batch.ResolveMany(ctx, enrollmentLegalKeys)
		if err != nil {
			return EnrollmentLegal{}, fmt.Errorf("resolve legal settings: %w", err)
		}
		if snapshot != nil {
			ctx = configModel.WithSettingsSnapshot(ctx, snapshot)
		}
	}
	var out EnrollmentLegal
	for _, text := range []struct {
		key   string
		label string
		into  *string
	}{
		{configModel.KeyEnrollmentLegalAGBText, "AGB legal text", &out.AGB},
		{configModel.KeyEnrollmentLegalAGBDocumentURL, "AGB legal document", &out.AGBDocumentURL},
		{configModel.KeyEnrollmentLegalAGBDisplayMode, "AGB legal display mode", &out.AGBDisplayMode},
		{configModel.KeyEnrollmentLegalDSGVOText, "DSGVO legal text", &out.DSGVO},
		{configModel.KeyEnrollmentLegalEmailContactText, "email contact legal text", &out.EmailContact},
		{configModel.KeyEnrollmentLegalPhotoText, "photo legal text", &out.Photo},
	} {
		value, err := v.source.ResolveString(ctx, text.key)
		if err != nil {
			return EnrollmentLegal{}, fmt.Errorf("resolve %s: %w", text.label, err)
		}
		*text.into = value
	}
	return v.legalToggles(ctx, out)
}

// legalToggles resolves the legal block toggles. A toggle without a tenant
// override is off; a failure fails closed, because the toggle decides whether
// a required legal block is rendered and enforced.
func (v EnrollmentIntakeValues) legalToggles(ctx context.Context, out EnrollmentLegal) (EnrollmentLegal, error) {
	for _, toggle := range []struct {
		key   string
		label string
		into  *bool
	}{
		{configModel.KeyEnrollmentLegalTermsEnabled, "AGB terms", &out.TermsEnabled},
		{configModel.KeyEnrollmentLegalDSGVOEnabled, "DSGVO legal block", &out.DSGVOEnabled},
		{configModel.KeyEnrollmentLegalEmailContactEnabled, "email contact legal block", &out.EmailContactEnabled},
		{configModel.KeyEnrollmentLegalPhotoEnabled, "photo legal block", &out.PhotoEnabled},
	} {
		has, err := v.source.HasTenantOverride(ctx, toggle.key)
		if err != nil {
			return EnrollmentLegal{}, fmt.Errorf("check %s setting override: %w", toggle.label, err)
		}
		if !has {
			continue
		}
		value, err := v.source.ResolveBool(ctx, toggle.key)
		if err != nil {
			return EnrollmentLegal{}, fmt.Errorf("resolve %s setting: %w", toggle.label, err)
		}
		*toggle.into = value
	}
	return out, nil
}
