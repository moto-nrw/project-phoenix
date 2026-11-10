package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The debtor number of a holiday registration (#5621, ADR 0121 addendum):
// the number of the family in the finance system of the Träger, entered by
// the Träger-Büro when it confirms and changed afterwards on a confirmed
// registration. Enrollment keeps it in the form of a reference to pay with:
// trimmed, at most enrollment.HolidayDebtorNumberMaxLength characters, no
// line break, no format. Whether the Träger asks for it and whether an
// issued invoice still lets it change is the Elternabrechnung's rule.

// ConfirmHolidayRegistration confirms a received registration, frozen or
// not, with the debtor number when one is given. The confirmation and the
// number are one entry in the audit trail.
func (h *HolidayRegistrationReview) ConfirmHolidayRegistration(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, debtorNumber string) (enrollment.HolidayRegistrationDetail, error) {
	number, err := normalizeHolidayDebtorNumber(debtorNumber)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	return h.change(ctx, reviewer, periodID, registrationID, func(period enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		if current.Status != enrollment.HolidayRegistrationReceived {
			return "", nil, enrollment.ErrHolidayRegistrationStateConflict
		}
		if err := h.refusePendingAllocation(ctx, reviewer.OrganizationID, period, current.ID); err != nil {
			return "", nil, err
		}
		if err := h.store.SetHolidayRegistrationStatus(ctx, reviewer.OrganizationID, current.ID, enrollment.HolidayRegistrationConfirmed); err != nil {
			return "", nil, err
		}
		details := enrollment.HolidayRegistrationEventDetails{From: current.Status}
		if number != current.DebtorNumber {
			if err := h.store.SetHolidayDebtorNumber(ctx, reviewer.OrganizationID, current.ID, number); err != nil {
				return "", nil, err
			}
			details.DebtorNumber = &enrollment.HolidayDebtorNumberChange{Before: current.DebtorNumber, After: number}
		}
		return enrollment.HolidayAuditConfirm, details, nil
	})
}

// SetHolidayDebtorNumber changes the debtor number of a confirmed
// registration, frozen or not: freezing locks cancellations and rebookings,
// not the reference to pay with. The audit trail keeps the number before
// and after; an unchanged number records only the read of the answer.
func (h *HolidayRegistrationReview) SetHolidayDebtorNumber(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID, registrationID int64, debtorNumber string) (enrollment.HolidayRegistrationDetail, error) {
	number, err := normalizeHolidayDebtorNumber(debtorNumber)
	if err != nil {
		return enrollment.HolidayRegistrationDetail{}, err
	}
	return h.change(ctx, reviewer, periodID, registrationID, func(_ enrollment.HolidayPeriod, current holidayRegistrationState) (string, any, error) {
		if current.Status != enrollment.HolidayRegistrationConfirmed {
			return "", nil, enrollment.ErrHolidayRegistrationStateConflict
		}
		if number == current.DebtorNumber {
			return enrollment.HolidayAuditRead, nil, nil
		}
		if err := h.store.SetHolidayDebtorNumber(ctx, reviewer.OrganizationID, current.ID, number); err != nil {
			return "", nil, err
		}
		return enrollment.HolidayAuditDebtorNumber, enrollment.HolidayRegistrationEventDetails{
			DebtorNumber: &enrollment.HolidayDebtorNumberChange{Before: current.DebtorNumber, After: number},
		}, nil
	})
}

// normalizeHolidayDebtorNumber trims a debtor number and checks its length
// and that it has no line break. An empty result means none.
func normalizeHolidayDebtorNumber(raw string) (string, error) {
	number := strings.TrimSpace(raw)
	switch {
	case strings.ContainsAny(number, "\r\n\v\f\u0085  "):
		return "", validation.Errors{"debtor_number": errors.New("must be one line")}
	case utf8.RuneCountInString(number) > enrollment.HolidayDebtorNumberMaxLength:
		return "", validation.Errors{"debtor_number": fmt.Errorf("must be at most %d characters", enrollment.HolidayDebtorNumberMaxLength)}
	}
	return number, nil
}
