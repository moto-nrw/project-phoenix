package studentpresencehttp

import (
	context "context"

	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// PrivacyConsentCapability is the Student Presence owner surface this
// resource needs for the per-child GDPR retention consent. Student Presence
// owns users.privacy_consents because the recorded window bounds how long
// presence data is kept (#3349).
type PrivacyConsentCapability interface {
	ListPrivacyConsents(context.Context, int64) ([]studentpresence.PrivacyConsent, error)
	RecordPrivacyConsent(context.Context, studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error)
	RevisePrivacyConsent(context.Context, studentpresence.PrivacyConsent) (studentpresence.PrivacyConsent, error)
}

// withinTenant runs fn in the transaction of tenantID. Inside a route that
// already runs under the tenant transaction middleware it joins that
// transaction, so an error returned from fn rolls the whole request back.
func withinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		tenant.ObserveMissingTenant(ctx, err)
		return err
	}
	return tenant.WithinTenant(ctx, id, fn)
}
