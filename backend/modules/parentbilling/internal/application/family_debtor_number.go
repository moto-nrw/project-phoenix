package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The debtor number of a family (#6692). A confirmation of a holiday
// registration, and a change of its number, also tell the family of the
// Träger: the parent who registered, how long a registration needs the
// number and the number itself. The family is the address of the
// registration (parentbilling.FamilyKey), in the Träger of the session only.
// The next registration of the family reads the number as a proposal. A
// number that differs from the family's needs the office's choice; it never
// replaces the family's number on its own.

// FamilyMemoryStore keeps the families of the holiday care in the Träger
// transaction and records every read and change in the family's trail.
type FamilyMemoryStore interface {
	// LockFamilies adds a row for each family without one and holds every
	// row until the transaction ends.
	LockFamilies(ctx context.Context, organizationID int64, families []parentbilling.FamilyMemory) ([]parentbilling.FamilyBilling, error)
	// RememberFamilies writes the parent, the last day of use and, where
	// given, the number of each family in one statement.
	RememberFamilies(ctx context.Context, organizationID, accountID int64, families []parentbilling.FamilyMemory) ([]parentbilling.FamilyBilling, error)
	// FamilyBillingOf reads the row of a family, nil without one.
	FamilyBillingOf(ctx context.Context, organizationID int64, familyEmail string) (*parentbilling.FamilyBilling, error)
	InsertFamilyBillingEvents(ctx context.Context, organizationID, accountID int64, action string, families []parentbilling.FamilyBilling) error
}

// debtorNumberOf checks a debtor number the office entered through the one
// rule of the Elternabrechnung (#6675 extends it) as a validation error on
// debtor_number.
func debtorNumberOf(raw string) (string, error) {
	number, err := parentbilling.NormalizeDebtorNumber(raw)
	if err != nil {
		return "", validation.Errors{"debtor_number": err}
	}
	return number, nil
}

// HolidayFamilyDebtorNumber reads the number the family of the registration
// has, through Enrollment's review of the registration, which refuses one
// of another Träger and records the read in the registration's trail. A
// number it shows is recorded in the family's trail.
func (h *HolidayContributions) HolidayFamilyDebtorNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64) (parentbilling.HolidayFamilyDebtorNumber, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	registration, err := h.registrations.HolidayRegistration(ctx, reviewer, periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	key := parentbilling.FamilyKey(registration.GuardianEmail)
	if key == "" {
		return parentbilling.HolidayFamilyDebtorNumber{}, nil
	}
	family, err := h.store.FamilyBillingOf(ctx, reviewer.OrganizationID, key)
	if err != nil || family == nil || family.DebtorNumber == "" {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	if err := h.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead,
		[]parentbilling.FamilyBilling{*family}); err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, fmt.Errorf("parent billing: record the read of the family's number: %w", err)
	}
	return parentbilling.HolidayFamilyDebtorNumber{DebtorNumber: family.DebtorNumber}, nil
}

// rememberFamilies tells the families of the registrations what a
// confirmation or a change of the number decided, once per family however
// many registrations it has: several registrations of one family confirmed
// together (#6674) and a sibling group (#5628) write the family once, with
// the same number. With a number set, a family whose number differs needs
// the office's choice: FamilyDebtorNumberKeep leaves the family's number,
// FamilyDebtorNumberReplace takes the new one, no choice is a
// FamilyDebtorNumberConflict. The rows are held first, so two confirmations
// of a family decide one after the other.
func (h *HolidayContributions) rememberFamilies(ctx context.Context, reviewer parentbilling.HolidayReviewer, registrations []parentbilling.HolidayRegistration,
	number *string, choice string,
) error {
	memories := familyMemories(registrations, number, calendar.TodayDate())
	if len(memories) == 0 {
		return nil
	}
	held, err := h.store.LockFamilies(ctx, reviewer.OrganizationID, memories)
	if err != nil {
		return err
	}
	if number != nil && *number != "" {
		for _, family := range held {
			if family.DebtorNumber == "" || family.DebtorNumber == *number {
				continue
			}
			switch choice {
			case parentbilling.FamilyDebtorNumberReplace:
			case parentbilling.FamilyDebtorNumberKeep:
				at := slices.IndexFunc(memories, func(memory parentbilling.FamilyMemory) bool { return memory.FamilyEmail == family.FamilyEmail })
				memories[at].DebtorNumber = nil
			default:
				return &parentbilling.FamilyDebtorNumberConflict{FamilyDebtorNumber: family.DebtorNumber, DebtorNumber: *number}
			}
		}
	}
	saved, err := h.store.RememberFamilies(ctx, reviewer.OrganizationID, reviewer.AccountID, memories)
	if err != nil {
		return err
	}
	if err := h.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, saved); err != nil {
		return fmt.Errorf("parent billing: record the family's number: %w", err)
	}
	return nil
}

// familyMemories is one memory per family of the registrations, by key,
// with the parent of its first registration and the later of today and the
// last booked day of its registrations. An empty number removes none: it
// tells the family nothing about its number.
func familyMemories(registrations []parentbilling.HolidayRegistration, number *string, today calendar.Date) []parentbilling.FamilyMemory {
	if number != nil && *number == "" {
		number = nil
	}
	var memories []parentbilling.FamilyMemory
	for _, registration := range registrations {
		key := parentbilling.FamilyKey(registration.GuardianEmail)
		if key == "" {
			continue
		}
		used := lastBookedDay(registration, today)
		at := slices.IndexFunc(memories, func(memory parentbilling.FamilyMemory) bool { return memory.FamilyEmail == key })
		if at >= 0 {
			if used.After(memories[at].UsedOn) {
				memories[at].UsedOn = used
			}
			continue
		}
		memories = append(memories, parentbilling.FamilyMemory{FamilyEmail: key, FirstName: truncateName(registration.GuardianFirstName),
			LastName: truncateName(registration.GuardianLastName), UsedOn: used, DebtorNumber: number})
	}
	slices.SortFunc(memories, func(a, b parentbilling.FamilyMemory) int {
		switch {
		case a.FamilyEmail < b.FamilyEmail:
			return -1
		case a.FamilyEmail > b.FamilyEmail:
			return 1
		}
		return 0
	})
	return memories
}

// lastBookedDay is the later of today and the last day of a week the
// registration booked.
func lastBookedDay(registration parentbilling.HolidayRegistration, today calendar.Date) calendar.Date {
	last := today
	for _, child := range registration.Children {
		for _, booking := range child.Bookings {
			if booking.WeekEndsOn.After(last) {
				last = booking.WeekEndsOn
			}
		}
	}
	return last
}

// truncateName keeps a name within the 100 characters of the family row.
func truncateName(name string) string {
	runes := []rune(name)
	if len(runes) > 100 {
		return string(runes[:100])
	}
	return name
}

// confirmedSharers are the other members of a sibling group that hold the
// number of the group after a confirmation or a change: confirmed and not
// billed by an issued invoice (shareDebtorNumber).
func confirmedSharers(members []siblingMember, issued map[int64]bool) []parentbilling.HolidayRegistration {
	var sharers []parentbilling.HolidayRegistration
	for _, member := range members {
		if member.registration.Status == parentbilling.HolidayRegistrationConfirmed && !issued[member.id()] {
			sharers = append(sharers, member.registration)
		}
	}
	return sharers
}

// SetFamilyDebtorNumber sets the number of a family of the Träger by its
// key from the list of families (#6692): a family with a row or a document
// of the Träger, also one without a care contract. The change is recorded
// in the family's trail.
func (f *BillingFamilies) SetFamilyDebtorNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer, input parentbilling.FamilyDebtorNumberInput) (parentbilling.FamilyDebtorNumber, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.FamilyDebtorNumber{}, err
	}
	number, err := debtorNumberOf(input.DebtorNumber)
	if err != nil {
		return parentbilling.FamilyDebtorNumber{}, err
	}
	key := parentbilling.FamilyKey(input.FamilyEmail)
	if key == "" {
		return parentbilling.FamilyDebtorNumber{}, parentbilling.ErrFamilyNotFound
	}
	family, err := f.store.FamilyBillingOf(ctx, reviewer.OrganizationID, key)
	if err != nil {
		return parentbilling.FamilyDebtorNumber{}, err
	}
	if family == nil {
		known, err := f.store.FamilyHasDocuments(ctx, reviewer.OrganizationID, key)
		if err != nil {
			return parentbilling.FamilyDebtorNumber{}, err
		}
		if !known {
			return parentbilling.FamilyDebtorNumber{}, parentbilling.ErrFamilyNotFound
		}
	}
	saved, err := f.store.SetFamilyDebtorNumber(ctx, reviewer.OrganizationID, reviewer.AccountID, key, number, calendar.TodayDate())
	if err != nil {
		return parentbilling.FamilyDebtorNumber{}, err
	}
	if err := f.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave,
		[]parentbilling.FamilyBilling{saved}); err != nil {
		return parentbilling.FamilyDebtorNumber{}, fmt.Errorf("parent billing: record the family's number: %w", err)
	}
	return parentbilling.FamilyDebtorNumber{FamilyEmail: saved.FamilyEmail, DebtorNumber: saved.DebtorNumber}, nil
}

// errFamilyChoice refuses a choice that is neither keep nor replace.
var errFamilyChoice = errors.New("family_debtor_number must be keep or replace")

// validFamilyChoice accepts no choice, keep or replace.
func validFamilyChoice(choice string) error {
	switch choice {
	case "", parentbilling.FamilyDebtorNumberKeep, parentbilling.FamilyDebtorNumberReplace:
		return nil
	}
	return validation.Errors{"family_debtor_number": errFamilyChoice}
}
