package api

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-chi/chi/v5"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryRuntime "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	parentAPI "github.com/moto-nrw/project-phoenix/workflows/parentportal/http"
)

// mountTraegerRoutes serves the Träger-Portal at /traeger and the public
// holiday registration (#4661, ADR 0110) at /traeger/holiday-registrations: the one
// link of a published holiday period, without an account. Turnstile is
// always required there.
func mountTraegerRoutes(root chi.Router, limiters authRateLimiters, identity TraegerPortalIdentity, organizations organizationtenancy.Capability,
	persons TraegerPersons, staff TraegerStaff, audit TraegerAudit, settings settingsHTTP.TraegerOperations, delivery deliveryRuntime.Capability, keys TurnstileKeys, logger *slog.Logger) error {
	if err := mountTraegerPortalRoutes(root, limiters, identity, organizations, persons, staff, audit, settings); err != nil {
		return err
	}
	root.Mount("/traeger/holiday-registrations", NewHolidayRegistrationHTTP(organizations, delivery, keys, "", logger).Router())
	return nil
}

// HolidayOrganizations is Organisation & Tenancy as the holiday registration
// reads the Träger of a link.
type HolidayOrganizations interface {
	FindOrganization(context.Context, int64) (organizationtenancy.Organization, error)
	ListSchoolsByOrganization(context.Context, int64) ([]organizationtenancy.School, error)
}

// NewHolidayRegistrationHTTP binds the public holiday registration to
// Organisation & Tenancy, Delivery and Turnstile. An empty verifyURL uses
// Turnstile's endpoint; tests point it at a local provider.
func NewHolidayRegistrationHTTP(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, keys TurnstileKeys, verifyURL string, logger *slog.Logger) *enrollmentAPI.HolidayRegistrationResource {
	registrations := enrollmentCompose.NewHolidayRegistrations(holidayTraegerDirectory{organizations: organizations},
		holidayTraegerMail{delivery: delivery}, logger.With("service", "holiday-registrations"))
	captcha := enrollmentCompose.NewCaptcha(holidayCaptchaSettings(keys), verifyURL, logger.With("service", "holiday-registration-captcha"))
	return enrollmentAPI.NewHolidayRegistrationResource(registrations, captcha)
}

// holidayCaptchaSettings always require the captcha with the platform's
// Turnstile keys: a Träger has no setting to switch it off yet (ADR 0110).
type holidayCaptchaSettings TurnstileKeys

func (s holidayCaptchaSettings) CaptchaRequired(context.Context) (bool, error) { return true, nil }

func (s holidayCaptchaSettings) CaptchaSecretKey(context.Context) (string, error) {
	return s.SecretKey, nil
}

func (s holidayCaptchaSettings) CaptchaSiteKey(context.Context) (string, error) {
	return s.SiteKey, nil
}

// holidayTraegerDirectory reads the Träger of a link and its undeleted
// schools. A deactivated, deleted or unknown Träger has no open link.
type holidayTraegerDirectory struct {
	organizations HolidayOrganizations
}

func (d holidayTraegerDirectory) HolidayTraeger(ctx context.Context, organizationID int64) (enrollmentOwner.HolidayTraeger, error) {
	organization, err := d.organizations.FindOrganization(ctx, organizationID)
	if errors.Is(err, organizationtenancy.ErrOrganizationNotFound) || (err == nil && (!organization.Active || organization.IsDeleted())) {
		return enrollmentOwner.HolidayTraeger{}, enrollmentOwner.ErrHolidayRegistrationNotFound
	}
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	schools, err := d.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return enrollmentOwner.HolidayTraeger{}, err
	}
	traeger := enrollmentOwner.HolidayTraeger{Name: organization.Name, Schools: []enrollmentOwner.HolidaySchool{}}
	for _, school := range schools {
		if !school.IsDeleted() {
			traeger.Schools = append(traeger.Schools, enrollmentOwner.HolidaySchool{ID: school.ID, Name: school.Name})
		}
	}
	return traeger, nil
}

// holidayTraegerMail enqueues the receipt of a holiday registration as a
// mail of the Träger in the caller's Träger transaction.
type holidayTraegerMail struct {
	delivery deliveryRuntime.Capability
}

func (m holidayTraegerMail) EnqueueTraegerMail(ctx context.Context, mail enrollmentOwner.TraegerMail) error {
	_, err := m.delivery.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		OrganizationID: mail.OrganizationID, Template: mail.Kind, Recipient: deliveryContract.EmailRecipient{Address: mail.Recipient},
		Payload: mail.Payload, IdempotencyKey: mail.IdempotencyKey,
		Related: deliveryContract.RelatedEntity{Type: mail.RelatedType, ID: mail.RelatedID},
	})
	return err
}

// parentHolidayRegistrationLinks resolves the link of a holiday period for
// the parents portal's prefill (#4662, ADR 0113): the Träger and its
// undeleted schools, in the caller's administrative transaction.
type parentHolidayRegistrationLinks struct {
	registrations enrollmentOwner.HolidayRegistrations
}

// newParentHolidayRegistrationLinks binds the parents portal to Enrollment's
// link resolution over the same Träger directory as the public link.
func newParentHolidayRegistrationLinks(organizations HolidayOrganizations, delivery deliveryRuntime.Capability, logger *slog.Logger) parentAPI.HolidayRegistrationLinks {
	return parentHolidayRegistrationLinks{registrations: enrollmentCompose.NewHolidayRegistrations(holidayTraegerDirectory{organizations: organizations},
		holidayTraegerMail{delivery: delivery}, logger.With("service", "parent-holiday-registrations"))}
}

func (l parentHolidayRegistrationLinks) ResolveHolidayRegistrationSchools(ctx context.Context, token string) (int64, []int64, error) {
	link, err := l.registrations.ResolveHolidayRegistrationLink(ctx, token)
	if errors.Is(err, enrollmentOwner.ErrHolidayRegistrationNotFound) {
		return 0, nil, parentAPI.ErrHolidayRegistrationLinkNotFound
	}
	if err != nil {
		return 0, nil, err
	}
	schoolIDs := make([]int64, 0, len(link.Traeger.Schools))
	for _, school := range link.Traeger.Schools {
		schoolIDs = append(schoolIDs, school.ID)
	}
	return link.OrganizationID, schoolIDs, nil
}
