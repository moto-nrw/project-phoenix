package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The debtor number of a family (#6692). A confirmation of a holiday
// registration, and a change of its number, also tell the family of the
// Träger: the parent who registered, how long a registration needs the
// number and the number itself. The family is the address of the
// registration (parentbilling.FamilyKey), in the Träger of the session only.
// The next registration of the family reads the number as a proposal. A
// number that differs from the family's needs the office's choice; it never
// replaces the family's number on its own, unless the family has the number
// from the very registration that changes it.

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
	// ChildNumberStore keeps the numbers of the children when the Träger
	// keeps one per child (#7007).
	ChildNumberStore
}

// debtorNumberOf checks a debtor number the office entered through the one
// rule of the Elternabrechnung, in the format of the Träger (#6675), as a
// validation error on debtor_number. A number in another format stays a
// DebtorNumberFormatError inside it, so the refusal names the format.
func debtorNumberOf(raw string, format parentbilling.DebtorNumberFormat) (string, error) {
	number, err := parentbilling.NormalizeDebtorNumber(raw, format)
	if err != nil {
		return "", validation.Errors{"debtor_number": err}
	}
	return number, nil
}

// debtorNumberFormatOf reads the format of the Träger's debtor numbers.
func debtorNumberFormatOf(ctx context.Context, formats parentbilling.DebtorNumberFormats, organizationID int64) (parentbilling.DebtorNumberFormat, error) {
	format, err := formats.DebtorNumberFormat(ctx, organizationID)
	if err != nil {
		return parentbilling.DebtorNumberFormat{}, fmt.Errorf("parent billing: read the format of the debtor number: %w", err)
	}
	return format, nil
}

// debtorNumberOf checks a number entered at a holiday registration in the
// format of the reviewer's Träger.
func (h *HolidayContributions) debtorNumberOf(ctx context.Context, organizationID int64, raw string) (string, error) {
	format, err := debtorNumberFormatOf(ctx, h.settings, organizationID)
	if err != nil {
		return "", err
	}
	return debtorNumberOf(raw, format)
}

// DebtorNumberFormat answers the format of the Träger's debtor numbers, so
// the list of families and the form of a family can name it (#6675).
func (f *BillingFamilies) DebtorNumberFormat(ctx context.Context, reviewer parentbilling.HolidayReviewer) (parentbilling.DebtorNumberFormat, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.DebtorNumberFormat{}, err
	}
	return debtorNumberFormatOf(ctx, f.formats, reviewer.OrganizationID)
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
	perChild, err := h.perChild(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	if perChild {
		return h.holidayChildDebtorNumbers(ctx, reviewer, registration)
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

// rememberFamilies tells the families of the registrations the number a
// confirmation or a change of the number decided, once per family however
// many registrations it has: several registrations of one family confirmed
// together (#6674) and a sibling group (#5628) write the family once, with
// the same number. Without a number it tells no family anything and writes
// no row (data minimisation). A family whose number differs needs the
// office's choice: FamilyDebtorNumberKeep leaves the family's number,
// FamilyDebtorNumberReplace takes the new one, no choice is a
// FamilyDebtorNumberConflict. Only a family that has its number from the
// registration the office confirms or changes (correctingID), takes a
// correction without a choice (#6692.3). A merge passes zero and never
// corrects an existing number; one that has it from another
// registration, a sibling's too, or from the list of families does not. The rows are held first, so two
// confirmations of a family decide one after the other.
//
// When the Träger keeps a number per child (#7007), the children learn it
// instead (rememberChildren).
func (h *HolidayContributions) rememberFamilies(ctx context.Context, reviewer parentbilling.HolidayReviewer, registrations []parentbilling.HolidayRegistration,
	number *string, choice string, correctingID int64,
) error {
	perChild, err := h.perChild(ctx, reviewer.OrganizationID)
	if err != nil {
		return err
	}
	if perChild {
		return h.rememberChildren(ctx, reviewer, registrations, number)
	}
	return h.rememberPayers(ctx, reviewer, registrations, number, choice, correctingID)
}

// rememberPayers tells the families of the registrations the number, as
// rememberFamilies describes it, when the Träger keeps it per payer.
func (h *HolidayContributions) rememberPayers(ctx context.Context, reviewer parentbilling.HolidayReviewer, registrations []parentbilling.HolidayRegistration,
	number *string, choice string, correctingID int64,
) error {
	if number == nil || *number == "" {
		return h.touchFamilies(ctx, reviewer, registrations)
	}
	memories := familyMemories(registrations, *number, h.today())
	if len(memories) == 0 {
		return nil
	}
	held, err := h.store.LockFamilies(ctx, reviewer.OrganizationID, memories)
	if err != nil {
		return err
	}
	var conflicts []parentbilling.FamilyDebtorNumber
	for _, family := range held {
		if family.DebtorNumber == "" || family.DebtorNumber == *number || (correctingID > 0 && family.DebtorNumberRegistrationID == correctingID) {
			continue
		}
		switch choice {
		case parentbilling.FamilyDebtorNumberReplace:
		case parentbilling.FamilyDebtorNumberKeep:
			at := slices.IndexFunc(memories, func(memory parentbilling.FamilyMemory) bool { return memory.FamilyEmail == family.FamilyEmail })
			// LockFamilies returns only the requested family keys.
			memories[at].DebtorNumber = nil
		default:
			conflicts = append(conflicts, parentbilling.FamilyDebtorNumber{FamilyEmail: family.FamilyEmail, DebtorNumber: family.DebtorNumber})
		}
	}
	if len(conflicts) > 0 {
		return &parentbilling.FamilyDebtorNumberConflict{FamilyEmail: conflicts[0].FamilyEmail, FamilyDebtorNumber: conflicts[0].DebtorNumber,
			DebtorNumber: *number, Families: conflicts}
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

// touchFamilies refreshes when a family that already has a number last used
// it: an invoice of a registration without its own number takes the
// family's (#6677), so the number's retention must not end before that
// invoice. It updates existing rows only, writes no number and records no
// event: nothing the office decided changed (#6692).
func (h *HolidayContributions) touchFamilies(ctx context.Context, reviewer parentbilling.HolidayReviewer, registrations []parentbilling.HolidayRegistration) error {
	memories := familyMemories(registrations, "", h.today())
	for i := range memories {
		memories[i].DebtorNumber = nil
	}
	if _, err := h.store.RememberFamilies(ctx, reviewer.OrganizationID, reviewer.AccountID, memories); err != nil {
		return fmt.Errorf("parent billing: refresh the use of the family's number: %w", err)
	}
	return nil
}

// familyMemories is one memory per family of the registrations, by key,
// with the parent and the ID of its first registration and the later of
// today and the last booked day of its registrations.
func familyMemories(registrations []parentbilling.HolidayRegistration, number string, today calendar.Date) []parentbilling.FamilyMemory {
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
			LastName: truncateName(registration.GuardianLastName), UsedOn: used, RegistrationID: registration.ID, DebtorNumber: &number})
	}
	slices.SortFunc(memories, func(a, b parentbilling.FamilyMemory) int { return strings.Compare(a.FamilyEmail, b.FamilyEmail) })
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

// truncateName keeps a name within the characters of the family row.
func truncateName(name string) string {
	runes := []rune(name)
	if len(runes) > parentbilling.MaxBillingNameLength {
		return string(runes[:parentbilling.MaxBillingNameLength])
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
	format, err := debtorNumberFormatOf(ctx, f.formats, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.FamilyDebtorNumber{}, err
	}
	number, err := debtorNumberOf(input.DebtorNumber, format)
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
	saved, err := f.store.SetFamilyDebtorNumber(ctx, reviewer.OrganizationID, reviewer.AccountID, key, number, f.today())
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

// The debtor number of a child at a holiday registration (#7007). When the
// Träger keeps a number per child, the confirmation proposes the number of
// the first child of the registration, the number its invoice falls back
// to, and a confirmation or change with a number gives it to the only child
// of a registration that has none yet. A registration of several children
// teaches no child a number: one number cannot name several children. A
// child that has a number keeps it; the office changes it in the list of
// families.

// perChild tells whether the Träger keeps a number per child.
func (h *HolidayContributions) perChild(ctx context.Context, organizationID int64) (bool, error) {
	scope, err := debtorNumberScopeOf(ctx, h.settings, organizationID)
	return scope == parentbilling.DebtorNumberScopeChild, err
}

// rememberChildren gives the number to the only child of each registration
// that has none yet, and records it in the trail of the number.
func (h *HolidayContributions) rememberChildren(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	registrations []parentbilling.HolidayRegistration, number *string,
) error {
	if number == nil || *number == "" {
		return nil
	}
	// The family is known to the list of families from the confirmation on,
	// as with a number per payer (#6692), but keeps no number of its own.
	if err := h.knowFamilies(ctx, reviewer, registrations); err != nil {
		return err
	}
	var numbers []parentbilling.ChildDebtorNumber
	for _, registration := range registrations {
		if len(registration.Children) != 1 || parentbilling.FamilyKey(registration.GuardianEmail) == "" {
			continue
		}
		child := registration.Children[0]
		birth := child.BirthDate
		if parentbilling.ChildKey(child.FirstName, child.LastName, &birth) == "" {
			continue
		}
		numbers = append(numbers, parentbilling.ChildDebtorNumber{FamilyEmail: registration.GuardianEmail,
			ChildFirstName: truncateName(child.FirstName), ChildLastName: truncateName(child.LastName), ChildBirthDate: &birth,
			DebtorNumber: *number})
	}
	if len(numbers) == 0 {
		return nil
	}
	saved, err := h.store.SaveChildDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, numbers, h.today(), true)
	if err != nil {
		return err
	}
	if err := h.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, saved); err != nil {
		return fmt.Errorf("parent billing: record the child's number: %w", err)
	}
	return nil
}

// knowFamilies adds a row without a contract and without a number for each
// family of the registrations that has none, with the parent who registered
// and how long the registration needs it. A row the family has keeps its
// number and contract; a row without a contract takes the parent and the
// later day, as RememberFamilies writes them. The trail of every row without
// a number and a contract it writes records it.
func (h *HolidayContributions) knowFamilies(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	registrations []parentbilling.HolidayRegistration,
) error {
	memories := familyMemories(registrations, "", h.today())
	for i := range memories {
		memories[i].DebtorNumber = nil
	}
	if _, err := h.store.LockFamilies(ctx, reviewer.OrganizationID, memories); err != nil {
		return err
	}
	// A row the family had keeps the latest day a registration needs it.
	written, err := h.store.RememberFamilies(ctx, reviewer.OrganizationID, reviewer.AccountID, memories)
	if err != nil {
		return err
	}
	known := slices.DeleteFunc(written, func(family parentbilling.FamilyBilling) bool {
		return family.DebtorNumber != "" || family.ContractID != 0
	})
	if err := h.store.InsertFamilyBillingEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, known); err != nil {
		return fmt.Errorf("parent billing: record the family of the holiday care: %w", err)
	}
	return nil
}
