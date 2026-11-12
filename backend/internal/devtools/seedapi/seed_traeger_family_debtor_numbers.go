package api

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// The debtor number of a family (#6692). The seed shows the three states of
// the confirmation and of Abrechnung → Familien with invented families:
//
//   - Petra Sommer registers only for the holiday care. The Träger-Büro
//     confirms her registration with seedFamilyDebtorNumber, so the family
//     stands under Familien as „Nur Ferien“ with that number although no
//     invoice exists yet. Her second registration is received and proposes
//     the number at the field.
//   - Jana Wechsel's first registration is confirmed with
//     seedConflictRegistrationNumber; then the Abrechnung changes the number
//     of the family in the list of families to seedConflictFamilyDebtorNumber.
//     The registration and the family now differ: changing the
//     registration's number, or confirming her second, received
//     registration with another number, opens the dialog „Welche
//     Debitorennummer gilt?“.
//
// The numbers have five digits, as the finance system of Bissendorf keeps
// them; after the siblings' confirmation the demo Träger allows only that
// format (#6675, seedDebtorNumberFormat). The siblings keep their number in
// the old format, which shows that a stored number stays as it is.
//
// A family is known to the list only once a registration of it is
// confirmed or a document names it, so the list sets a number after the
// confirmation.
const (
	seedFamilyDebtorNumber         = "20815"
	seedConflictRegistrationNumber = "30100"
	seedConflictFamilyDebtorNumber = "30101"
)

// seedSiblingsConfirmation confirms the siblings with their number. The
// family of a seeded parent may have another number from its care contract;
// the registration keeps its own and the family keeps that one (#6692).
var seedSiblingsConfirmation = map[string]any{"benefits_exemption": false, "debtor_number": seedHolidayDebtorNumber,
	"family_debtor_number": "keep"}

// seedHolidayFamilies adds the families after the first confirmation: a child
// the school knows, a sibling group and the remembered debtor numbers (#6692).
func seedHolidayFamilies(rt *Runtime, auth AuthRef, token string, periodID int64, registration map[string]any, siblings int64) error {
	if err := seedHolidayKnownChild(rt, auth, token, periodID, registration); err != nil {
		return err
	}
	if err := seedHolidaySiblingGroup(rt, auth, token, periodID, registration, siblings); err != nil {
		return err
	}
	return seedHolidayFamilyDebtorNumbers(rt, auth, token, periodID, registration)
}

// seedHolidayFamilyDebtorNumbers registers and confirms the families above in
// the seeded period. A server that refuses a registration (captcha, rate
// limit) keeps the period without them.
func seedHolidayFamilyDebtorNumbers(rt *Runtime, auth AuthRef, token string, periodID int64, registration map[string]any) error {
	first, refused, err := seedHolidayFamilyRegistration(rt, token, registration, "Petra", "Sommer", "petra.sommer@example.org", "Ida")
	if err != nil || refused {
		return err
	}
	confirm := fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, first)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{"benefits_exemption": false, "debtor_number": seedFamilyDebtorNumber}); err != nil {
		return fmt.Errorf("confirm the holiday family's first registration: %w", err)
	}
	if _, refused, err := seedHolidayFamilyRegistration(rt, token, registration, "Petra", "Sommer", "Petra.Sommer@example.org", "Ole"); err != nil || refused {
		return err
	}
	fmt.Println("  Ferienanmeldung: holiday family remembered with its debtor number, its next registration proposes it")

	conflict, refused, err := seedHolidayFamilyRegistration(rt, token, registration, "Jana", "Wechsel", "jana.wechsel@example.org", "Emil")
	if err != nil || refused {
		return err
	}
	confirm = fmt.Sprintf("/traeger/holiday-contributions/%d/registrations/%d/confirm", periodID, conflict)
	if _, err := rt.Client.PostWithAuth(auth, confirm, map[string]any{"benefits_exemption": false, "debtor_number": seedConflictRegistrationNumber}); err != nil {
		return fmt.Errorf("confirm the registration of the family with two numbers: %w", err)
	}
	if _, refused, err := seedHolidayFamilyRegistration(rt, token, registration, "Jana", "Wechsel", "jana.wechsel@example.org", "Mats"); err != nil || refused {
		return err
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/billing-families/debtor-number", map[string]any{
		"family_email": "jana.wechsel@example.org", "debtor_number": seedConflictFamilyDebtorNumber,
	}); err != nil {
		return fmt.Errorf("set the debtor number of a family in the list of families: %w", err)
	}
	fmt.Println("  Abrechnung: debtor number of a holiday family set in the list of families")
	return nil
}

// seedHolidayFamilyRegistration sends a registration of one child, booked
// for the first week, by the given parent. refused is true when the server
// refuses seeded registrations.
func seedHolidayFamilyRegistration(rt *Runtime, token string, registration map[string]any, firstName, lastName, email, child string) (int64, bool, error) {
	body := map[string]any{}
	maps.Copy(body, registration)
	delete(body, "second_guardian")
	body["guardian"] = map[string]any{"first_name": firstName, "last_name": lastName, "email": email, "phone": "0151 0000077"}
	body["guardian_address"] = map[string]any{"street": "Ferienweg 3", "postal_code": "49143", "city": "Bissendorf"}
	body["declaration_name"] = firstName + " " + lastName
	children := registration["children"].([]map[string]any)
	bookings := children[0]["bookings"].([]map[string]any)
	body["children"] = []map[string]any{{
		"first_name": child, "last_name": lastName, "birth_date": "2019-05-14", "home_school_id": children[0]["home_school_id"],
		"bookings": bookings[:1], "custom_data": children[0]["custom_data"],
	}}
	raw, err := rt.Client.PostPublic("/traeger/holiday-registrations/"+token, body)
	var refused *APIError
	if errors.As(err, &refused) && slices.Contains(seedHolidayRefusals, refused.Code) {
		fmt.Println("  Ferienanmeldung: the server refuses the holiday family, no remembered debtor number")
		return 0, true, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("register the holiday family %s: %w", lastName, err)
	}
	id, err := decodeSeedEntityID(raw)
	if err != nil {
		return 0, false, fmt.Errorf("read the holiday family's registration: %w", err)
	}
	return id, false, nil
}

// seedAskForDebtorNumber asks for the debtor number at the confirmation and
// accepts any number until seedDebtorNumberFormat switches the format on. A
// second seed run on the same Träger finds the format still on, so it is
// switched off here first: the siblings keep their old-format number.
func seedAskForDebtorNumber(rt *Runtime, auth AuthRef) error {
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+seedDebtorNumberSettingKey, map[string]any{"value": true}); err != nil {
		return fmt.Errorf("ask for the debtor number at the confirmation: %w", err)
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+seedDebtorNumberDigitsOnlyKey, map[string]any{"value": false}); err != nil {
		return fmt.Errorf("accept every debtor number before the format: %w", err)
	}
	return nil
}

// seedDebtorNumberFormat switches the demo Träger to the format of
// Bissendorf (#6675): debtor numbers of exactly five digits. It comes after
// the siblings' confirmation, so their number "10 4711" stays in the old
// format and shows that stored numbers are never changed.
func seedDebtorNumberFormat(rt *Runtime, auth AuthRef) error {
	for _, setting := range []struct {
		key   string
		value any
	}{
		{seedDebtorNumberDigitsOnlyKey, true}, {seedDebtorNumberMinDigitsKey, 5}, {seedDebtorNumberMaxDigitsKey, 5},
	} {
		if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+setting.key, map[string]any{"value": setting.value}); err != nil {
			return fmt.Errorf("set the format of the debtor number: %w", err)
		}
	}
	fmt.Println("  Träger-Büro: debtor numbers have five digits from now on")
	return nil
}

// The settings of the format of a debtor number (#6675).
const (
	seedDebtorNumberDigitsOnlyKey = "parent_billing.debtor_number_digits_only"
	seedDebtorNumberMinDigitsKey  = "parent_billing.debtor_number_min_digits"
	seedDebtorNumberMaxDigitsKey  = "parent_billing.debtor_number_max_digits"
)
