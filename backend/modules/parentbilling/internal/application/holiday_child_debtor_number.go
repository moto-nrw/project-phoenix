package application

import (
	"context"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// The debtor numbers of the children of a holiday registration (#7007).
// When the Träger keeps the number per child, each child of a registration
// is billed on an invoice of its own with its own number. The confirmation
// proposes the number each child has and keeps what the office enters as the
// child's number; the office changes them later the same way. The children
// are those of the registration as Enrollment reads it for the Träger, never
// names from the request.

// childDebtorNumberField is the field of a child's number in a validation
// error.
func childDebtorNumberField(childID int64) string {
	return fmt.Sprintf("child_debtor_number_%d", childID)
}

// holidayChildDebtorNumbers names every child of the registration with the
// number it has, recorded in the trail of each number it shows. The first
// child's number is the registration's proposal.
func (h *HolidayContributions) holidayChildDebtorNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	registration parentbilling.HolidayRegistration,
) (parentbilling.HolidayFamilyDebtorNumber, error) {
	result := parentbilling.HolidayFamilyDebtorNumber{Children: make([]parentbilling.HolidayChildDebtorNumber, 0, len(registration.Children))}
	if len(registration.Children) == 0 {
		return result, nil
	}
	numbers, err := h.store.ChildDebtorNumbers(ctx, reviewer.OrganizationID)
	if err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	byKey := make(map[string]parentbilling.ChildDebtorNumber, len(numbers))
	for _, number := range numbers {
		if number.DebtorNumber != "" {
			byKey[number.Key()] = number
		}
	}
	var read []parentbilling.ChildDebtorNumber
	for _, child := range registration.Children {
		shown := parentbilling.HolidayChildDebtorNumber{ChildID: child.ID, FirstName: child.FirstName, LastName: child.LastName}
		if number, ok := byKey[holidayChildKey(registration, child)]; ok {
			shown.DebtorNumber = number.DebtorNumber
			read = append(read, number)
		}
		result.Children = append(result.Children, shown)
	}
	result.DebtorNumber = result.Children[0].DebtorNumber
	if err := h.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingRead, read); err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, fmt.Errorf("parent billing: record the read of the children's numbers: %w", err)
	}
	return result, nil
}

// holidayChildKey is the key of a child of the registration within the
// Träger; empty when the registration has no family or the child no name.
func holidayChildKey(registration parentbilling.HolidayRegistration, child parentbilling.HolidayChild) string {
	birth := child.BirthDate
	key := parentbilling.ChildKey(child.FirstName, child.LastName, &birth)
	if key == "" || parentbilling.FamilyKey(registration.GuardianEmail) == "" {
		return ""
	}
	return parentbilling.ChildDebtorNumberKey(registration.GuardianEmail, key)
}

// confirmsChildNumbers says whether a confirmation takes the numbers of the
// children: when the office enters them and the Träger keeps the number per
// child. Otherwise the registration's number applies, which the Träger may
// ask for.
func (h *HolidayContributions) confirmsChildNumbers(ctx context.Context, organizationID int64, debtorNumber string,
	childNumbers []parentbilling.HolidayChildDebtorNumberInput,
) (bool, error) {
	if childNumbers != nil {
		perChild, err := h.perChild(ctx, organizationID)
		if err != nil || perChild {
			return perChild, err
		}
	}
	return false, h.requireDebtorNumber(ctx, organizationID, debtorNumber)
}

// SetHolidayChildDebtorNumbers changes the numbers of the children of a
// confirmed registration (#7007).
func (h *HolidayContributions) SetHolidayChildDebtorNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer, periodID, registrationID int64,
	numbers []parentbilling.HolidayChildDebtorNumberInput,
) (parentbilling.HolidayFamilyDebtorNumber, error) {
	if err := validReviewer(reviewer); err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	registration, err := h.registrations.HolidayRegistration(ctx, reviewer, periodID, registrationID)
	if err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	if registration.Status != parentbilling.HolidayRegistrationConfirmed {
		return parentbilling.HolidayFamilyDebtorNumber{}, parentbilling.ErrHolidayRegistrationStateConflict
	}
	entered, err := h.enteredChildNumbers(ctx, reviewer.OrganizationID, registration, numbers)
	if err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	return h.keepChildNumbers(ctx, reviewer, registration, entered)
}

// confirmChildNumbers keeps the numbers of the children a confirmation
// takes (confirmsChildNumbers).
func (h *HolidayContributions) confirmChildNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	registration parentbilling.HolidayRegistration, byChild bool, numbers []parentbilling.HolidayChildDebtorNumberInput,
) error {
	if !byChild {
		return nil
	}
	entered, err := h.enteredChildNumbers(ctx, reviewer.OrganizationID, registration, numbers)
	if err != nil {
		return err
	}
	_, err = h.keepChildNumbers(ctx, reviewer, registration, entered)
	return err
}

// keepChildNumbers gives each child named the number enteredChildNumbers
// checked, an empty one removes it, and makes the family known to the list
// of families. The answer is the children named with their numbers.
func (h *HolidayContributions) keepChildNumbers(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	registration parentbilling.HolidayRegistration, entered map[int64]string,
) (parentbilling.HolidayFamilyDebtorNumber, error) {
	if err := h.knowFamilies(ctx, reviewer, []parentbilling.HolidayRegistration{registration}); err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	result := parentbilling.HolidayFamilyDebtorNumber{Children: make([]parentbilling.HolidayChildDebtorNumber, 0, len(registration.Children))}
	var save []parentbilling.ChildDebtorNumber
	for _, child := range registration.Children {
		number, named := entered[child.ID]
		if !named {
			// A child the office did not name keeps its number.
			continue
		}
		result.Children = append(result.Children, parentbilling.HolidayChildDebtorNumber{ChildID: child.ID, FirstName: child.FirstName,
			LastName: child.LastName, DebtorNumber: number})
		if number == "" {
			if err := h.removeChildNumber(ctx, reviewer, registration, child); err != nil {
				return parentbilling.HolidayFamilyDebtorNumber{}, err
			}
			continue
		}
		birth := child.BirthDate
		save = append(save, parentbilling.ChildDebtorNumber{FamilyEmail: registration.GuardianEmail, ChildFirstName: truncateName(child.FirstName),
			ChildLastName: truncateName(child.LastName), ChildBirthDate: &birth, DebtorNumber: number})
	}
	if len(save) == 0 {
		return result, nil
	}
	saved, err := h.store.SaveChildDebtorNumbers(ctx, reviewer.OrganizationID, reviewer.AccountID, save, h.today(), false)
	if err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, err
	}
	if err := h.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingSave, saved); err != nil {
		return parentbilling.HolidayFamilyDebtorNumber{}, fmt.Errorf("parent billing: record the children's numbers: %w", err)
	}
	return result, nil
}

// enteredChildNumbers checks the entered numbers: only children of the
// registration, each in the Träger's format, and, when the Träger asks for
// a number, one for every child, entered or stored. The errors name the
// field of each child.
func (h *HolidayContributions) enteredChildNumbers(ctx context.Context, organizationID int64, registration parentbilling.HolidayRegistration,
	inputs []parentbilling.HolidayChildDebtorNumberInput,
) (map[int64]string, error) {
	if parentbilling.FamilyKey(registration.GuardianEmail) == "" {
		return nil, parentbilling.ErrChildNotFound
	}
	children := make(map[int64]bool, len(registration.Children))
	for _, child := range registration.Children {
		if holidayChildKey(registration, child) == "" {
			return nil, parentbilling.ErrChildNotFound
		}
		children[child.ID] = true
	}
	required, err := h.asksDebtorNumber(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	entered := make(map[int64]string, len(inputs))
	errs := validation.Errors{}
	for _, input := range inputs {
		if !children[input.ChildID] {
			return nil, parentbilling.ErrChildNotFound
		}
		number, err := h.debtorNumberOf(ctx, organizationID, input.DebtorNumber)
		if err != nil {
			errs[childDebtorNumberField(input.ChildID)] = fieldError(err)
			continue
		}
		entered[input.ChildID] = number
	}
	if required {
		if err := h.requireChildNumbers(ctx, organizationID, registration, entered, errs); err != nil {
			return nil, err
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return entered, nil
}

// requireChildNumbers refuses each child without a number: one the office
// empties, or one it does not name that has none stored.
func (h *HolidayContributions) requireChildNumbers(ctx context.Context, organizationID int64, registration parentbilling.HolidayRegistration,
	entered map[int64]string, errs validation.Errors,
) error {
	numbers, err := h.store.ChildDebtorNumbers(ctx, organizationID)
	if err != nil {
		return err
	}
	stored := make(map[string]bool, len(numbers))
	for _, number := range numbers {
		stored[number.Key()] = number.DebtorNumber != ""
	}
	for _, child := range registration.Children {
		field := childDebtorNumberField(child.ID)
		if _, refused := errs[field]; refused {
			continue
		}
		number, named := entered[child.ID]
		if (named && number == "") || (!named && !stored[holidayChildKey(registration, child)]) {
			errs[field] = errors.New("is required")
		}
	}
	return nil
}

// fieldError is the refusal of a number without the field debtorNumberOf
// names, so it can stand at the field of a child.
func fieldError(err error) error {
	var fields validation.Errors
	if errors.As(err, &fields) {
		for _, refusal := range fields {
			return refusal
		}
	}
	return err
}

// removeChildNumber empties the number of a child that has one and records
// the removal in its trail.
func (h *HolidayContributions) removeChildNumber(ctx context.Context, reviewer parentbilling.HolidayReviewer,
	registration parentbilling.HolidayRegistration, child parentbilling.HolidayChild,
) error {
	birth := child.BirthDate
	key := parentbilling.ChildKey(child.FirstName, child.LastName, &birth)
	removed, err := h.store.DeleteChildDebtorNumber(ctx, reviewer.OrganizationID, reviewer.AccountID,
		parentbilling.FamilyKey(registration.GuardianEmail), key)
	if err != nil || removed == nil {
		return err
	}
	if err := h.store.InsertChildDebtorNumberEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, familyBillingDelete,
		[]parentbilling.ChildDebtorNumber{*removed}); err != nil {
		return fmt.Errorf("parent billing: record the removal of the child's number: %w", err)
	}
	return nil
}
