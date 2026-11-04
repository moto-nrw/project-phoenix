package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The debtor number of a holiday registration (#5621, ADR 0121 addendum):
// the number of the family in the finance system of the Träger, the
// reference to pay with. The Träger decides with a setting whether its
// office enters it at the confirmation; Enrollment keeps it at the
// registration. It changes until an issued invoice copies it (ADR 0119).

// HolidayConfirmationSettings reports whether a confirmation of the
// session's Träger needs the debtor number.
func (h *HolidayContributions) HolidayConfirmationSettings(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.HolidayConfirmationSettings, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayConfirmationSettings{}, errTraegerRequired
	}
	required, err := h.asksDebtorNumber(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.HolidayConfirmationSettings{}, err
	}
	bearers, err := h.store.ListCostBearers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.HolidayConfirmationSettings{}, fmt.Errorf("parent billing: list cost bearers: %w", err)
	}
	active := []parentbilling.CostBearerChoice{}
	for _, bearer := range bearers {
		if bearer.Active {
			active = append(active, parentbilling.CostBearerChoice{ID: bearer.ID, Name: bearer.Name})
		}
	}
	return parentbilling.HolidayConfirmationSettings{DebtorNumberRequired: required, CostBearers: active}, nil
}

// ChangeHolidayDebtorNumber changes the debtor number of a confirmed
// registration while no issued invoice bills it. It holds the invoices of
// the period first, so a release that runs at the same time copies either
// the number before or the number after, and the change sees an invoice
// the release committed. In a sibling group the other confirmed
// registrations take the number too, or lose it with a removal (#5628).
func (h *HolidayContributions) ChangeHolidayDebtorNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64, debtorNumber string) (parentbilling.HolidayDebtorNumber, error) {
	if reviewer.OrganizationID <= 0 || reviewer.AccountID <= 0 {
		return parentbilling.HolidayDebtorNumber{}, errTraegerRequired
	}
	if err := h.requireDebtorNumber(ctx, reviewer.OrganizationID, debtorNumber); err != nil {
		return parentbilling.HolidayDebtorNumber{}, err
	}
	groups, issued, err := h.lockSiblings(ctx, reviewer, periodID)
	if err != nil {
		return parentbilling.HolidayDebtorNumber{}, err
	}
	if issued[registrationID] {
		return parentbilling.HolidayDebtorNumber{}, parentbilling.ErrDebtorNumberLocked
	}
	registration, err := h.registrations.SetHolidayDebtorNumber(ctx, reviewer, periodID, registrationID, debtorNumber)
	if err != nil {
		return parentbilling.HolidayDebtorNumber{}, err
	}
	// A sibling group pays with one number (#5628): the other confirmed
	// registrations of the group that no issued invoice bills take it, or
	// lose it with this one.
	if group, grouped := groupOf(groups, registrationID); grouped {
		ids := slices.DeleteFunc(slices.Clone(group.RegistrationIDs), func(id int64) bool { return id == registrationID })
		others, err := h.readSiblings(ctx, reviewer, periodID, ids, nil)
		if err != nil {
			return parentbilling.HolidayDebtorNumber{}, err
		}
		if err := h.shareDebtorNumber(ctx, reviewer, periodID, others, registration.DebtorNumber, issued); err != nil {
			return parentbilling.HolidayDebtorNumber{}, err
		}
	}
	return parentbilling.HolidayDebtorNumber{RegistrationID: registration.ID, DebtorNumber: registration.DebtorNumber}, nil
}

// requireDebtorNumber refuses a blank debtor number when the Träger asks
// for it.
func (h *HolidayContributions) requireDebtorNumber(ctx context.Context, organizationID int64, debtorNumber string) error {
	if strings.TrimSpace(debtorNumber) != "" {
		return nil
	}
	required, err := h.asksDebtorNumber(ctx, organizationID)
	if err != nil {
		return err
	}
	if required {
		return validation.Errors{"debtor_number": errors.New("is required")}
	}
	return nil
}

func (h *HolidayContributions) asksDebtorNumber(ctx context.Context, organizationID int64) (bool, error) {
	required, err := h.settings.AsksDebtorNumber(ctx, organizationID)
	if err != nil {
		return false, fmt.Errorf("parent billing: read whether the Träger asks for the debtor number: %w", err)
	}
	return required, nil
}
