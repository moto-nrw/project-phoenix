package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The status link of a holiday registration (#6676): the token is resolved
// in the administrative transaction, the registration is read in the
// Träger transaction of its Träger.

// holidayStatusTokenMaxLength bounds a token before it reaches the store;
// the token of a status link has 43 characters.
const holidayStatusTokenMaxLength = 128

// newStatusToken draws the token of a new registration's status link from
// the bound random source; the composition always binds it.
func (h *HolidayRegistrations) newStatusToken() (string, error) {
	if h.deps.Random == nil {
		return "", errors.New("enrollment: the random source of the holiday registration status link is not bound")
	}
	token, err := enrollment.NewStatusToken(h.deps.Random)
	if err != nil {
		return "", fmt.Errorf("enrollment: draw holiday registration status token: %w", err)
	}
	return token, nil
}

// ResolveHolidayRegistrationStatus names the registration and the Träger of
// a status link. An empty, oversized or unknown token, and a registration of
// a Träger that is no longer active, are not found alike.
func (h *HolidayRegistrations) ResolveHolidayRegistrationStatus(ctx context.Context, token string) (enrollment.HolidayRegistrationStatusLink, error) {
	token = strings.TrimSpace(token)
	if token == "" || len(token) > holidayStatusTokenMaxLength {
		return enrollment.HolidayRegistrationStatusLink{}, enrollment.ErrHolidayRegistrationStatusNotFound
	}
	reference, found, err := h.deps.Registrations.FindHolidayRegistrationStatus(ctx, token)
	if err != nil {
		return enrollment.HolidayRegistrationStatusLink{}, fmt.Errorf("enrollment: resolve holiday registration status link: %w", err)
	}
	if !found {
		return enrollment.HolidayRegistrationStatusLink{}, enrollment.ErrHolidayRegistrationStatusNotFound
	}
	traeger, err := h.deps.Traeger.HolidayTraeger(ctx, reference.OrganizationID)
	if err != nil {
		if errors.Is(err, enrollment.ErrHolidayRegistrationNotFound) {
			return enrollment.HolidayRegistrationStatusLink{}, enrollment.ErrHolidayRegistrationStatusNotFound
		}
		return enrollment.HolidayRegistrationStatusLink{}, err
	}
	return enrollment.HolidayRegistrationStatusLink{Reference: reference, Traeger: traeger}, nil
}

// HolidayRegistrationStatus reads what the status link shows: the state, the
// period and per child the first name with its booked weeks and their host
// schools.
func (h *HolidayRegistrations) HolidayRegistrationStatus(ctx context.Context, link enrollment.HolidayRegistrationStatusLink) (enrollment.HolidayRegistrationStatus, error) {
	if link.Reference.OrganizationID <= 0 || link.Reference.RegistrationID <= 0 {
		return enrollment.HolidayRegistrationStatus{}, enrollment.ErrHolidayRegistrationStatusNotFound
	}
	schools := make(map[int64]string, len(link.Traeger.Schools))
	for _, school := range link.Traeger.Schools {
		schools[school.ID] = school.Name
	}
	status, found, err := h.deps.Registrations.HolidayRegistrationStatus(ctx, link.Reference, schools)
	if err != nil {
		return enrollment.HolidayRegistrationStatus{}, fmt.Errorf("enrollment: read holiday registration status: %w", err)
	}
	if !found {
		return enrollment.HolidayRegistrationStatus{}, enrollment.ErrHolidayRegistrationStatusNotFound
	}
	status.TraegerName = link.Traeger.Name
	return status, nil
}
