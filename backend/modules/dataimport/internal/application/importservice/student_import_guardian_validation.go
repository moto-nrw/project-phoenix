package importpkg

import (
	"fmt"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// validateGuardian validates a single guardian's data
func (c *StudentImportConfig) validateGuardian(num int, guardian importModels.GuardianImportData) []importModels.ValidationError {
	errors := []importModels.ValidationError{}
	fieldPrefix := fmt.Sprintf("guardian_%d", num)

	// Check contact methods: either legacy fields or new PhoneNumbers array
	hasLegacyContact := guardian.Email != "" || guardian.Phone != "" || guardian.MobilePhone != ""
	hasNewPhoneNumbers := len(guardian.PhoneNumbers) > 0

	// At least one contact method required
	if !hasLegacyContact && !hasNewPhoneNumbers {
		errors = append(errors, importModels.ValidationError{
			Field:    fieldPrefix,
			Message:  fmt.Sprintf("Erziehungsberechtigter %d benötigt mindestens eine Kontaktmethode (Email, Telefon oder Mobil)", num),
			Code:     "guardian_contact_required",
			Severity: importModels.ErrorSeverityError,
		})
		return errors // Return early if no contact info
	}

	// Validate email, legacy phones, and new phone numbers
	errors = append(errors, validateGuardianEmail(num, guardian.Email, fieldPrefix)...)
	errors = append(errors, validateGuardianLegacyPhones(num, guardian, fieldPrefix)...)
	errors = append(errors, validateGuardianPhoneNumbers(num, guardian.PhoneNumbers, fieldPrefix)...)

	// Validate language preference (warning for unrecognized codes)
	errors = append(errors, validateGuardianLanguage(num, guardian.LanguagePreference, fieldPrefix)...)

	if _, ok := importModels.MapGuardianRole(guardian.GuardianRole); !ok {
		errors = append(errors, importModels.ValidationError{
			Field:       fmt.Sprintf("%s_role", fieldPrefix),
			Message:     fmt.Sprintf("Unbekannte Rolle '%s' für Erziehungsberechtigten %d. Erlaubt: Hauptsorgeberechtigt, Sorgeberechtigt, Mitsorgeberechtigt, Notfallkontakt, Nur Abholung, Sozialarbeit.", guardian.GuardianRole, num),
			Code:        "invalid_guardian_role",
			Severity:    importModels.ErrorSeverityError,
			ActualValue: guardian.GuardianRole,
		})
	}
	if guardian.EmergencyPriority < 0 {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_emergency_priority", fieldPrefix),
			Message:  fmt.Sprintf("Notfallpriorität für Erziehungsberechtigten %d muss 1 oder größer sein.", num),
			Code:     "invalid_emergency_priority",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// validateGuardianLanguage warns about unrecognized language codes
func validateGuardianLanguage(num int, lang, fieldPrefix string) []importModels.ValidationError {
	if lang == "" {
		return nil // Empty is fine — backend defaults to "de"
	}
	normalized := strings.ToLower(strings.TrimSpace(lang))
	if !supportedLanguages[normalized] {
		return []importModels.ValidationError{{
			Field:    fmt.Sprintf("%s_language", fieldPrefix),
			Message:  fmt.Sprintf("Unbekannter Sprachcode '%s' für Erziehungsberechtigten %d. Gängige Codes: de, en, tr, ar, ru, pl", normalized, num),
			Code:     "unknown_language",
			Severity: importModels.ErrorSeverityWarning,
		}}
	}
	return nil
}

// validateGuardianEmail validates email format
func validateGuardianEmail(num int, email, fieldPrefix string) []importModels.ValidationError {
	if email != "" && !ports.IsValidEmailFormat(email) {
		return []importModels.ValidationError{{
			Field:    fmt.Sprintf("%s_email", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Email-Format für Erziehungsberechtigten %d: %s", num, email),
			Code:     "invalid_email",
			Severity: importModels.ErrorSeverityError,
		}}
	}
	return nil
}

// validateGuardianLegacyPhones validates legacy phone and mobile_phone fields
func validateGuardianLegacyPhones(num int, guardian importModels.GuardianImportData, fieldPrefix string) []importModels.ValidationError {
	var errors []importModels.ValidationError

	if guardian.Phone != "" && ports.ValidateOptionalPhone(guardian.Phone) != nil {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_phone", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Telefon-Format für Erziehungsberechtigten %d: %s", num, guardian.Phone),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if guardian.MobilePhone != "" && ports.ValidateOptionalPhone(guardian.MobilePhone) != nil {
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_mobile", fieldPrefix),
			Message:  fmt.Sprintf("Ungültiges Mobiltelefon-Format für Erziehungsberechtigten %d: %s", num, guardian.MobilePhone),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// validateGuardianPhoneNumbers validates phone numbers from the new flexible PhoneNumbers array
func validateGuardianPhoneNumbers(num int, phones []importModels.PhoneImportData, fieldPrefix string) []importModels.ValidationError {
	var errors []importModels.ValidationError

	for i, phone := range phones {
		if phone.PhoneNumber == "" || ports.ValidateOptionalPhone(phone.PhoneNumber) == nil {
			continue
		}
		label := phone.Label
		if label == "" {
			label = phone.PhoneType
		}
		errors = append(errors, importModels.ValidationError{
			Field:    fmt.Sprintf("%s_phone_%d", fieldPrefix, i+1),
			Message:  fmt.Sprintf("Ungültiges Telefon-Format für Erziehungsberechtigten %d (%s): %s", num, label, phone.PhoneNumber),
			Code:     "invalid_phone",
			Severity: importModels.ErrorSeverityError,
		})
	}

	return errors
}

// createGuardianPhoneNumbers creates phone numbers for a guardian from import data
// mapPhoneType converts import phone type string to users.PhoneType enum
func mapPhoneType(importType string) string {
	switch strings.ToLower(importType) {
	case "mobile":
		return ports.PhoneTypeMobile
	case "home":
		return ports.PhoneTypeHome
	case "work":
		return ports.PhoneTypeWork
	default:
		return ports.PhoneTypeOther
	}
}

// Update patches an existing student (#2600). Empty cells never clear a
// stored value; only what the row carries is written. Guardians are merged
// (matched by e-mail, new ones linked), schedules are replaced per weekday
// given, privacy consent is only created when none exists yet.
// importGuardianPhones collects every phone number of the row in normalized
// form, including the legacy single-column fields.
func importGuardianPhones(data importModels.GuardianImportData) map[string]struct{} {
	phones := make(map[string]struct{}, len(data.PhoneNumbers)+2)
	add := func(raw string) {
		if normalized := normalizeImportPhone(raw); normalized != "" {
			phones[normalized] = struct{}{}
		}
	}
	for _, phone := range data.PhoneNumbers {
		add(phone.PhoneNumber)
	}
	add(data.Phone)
	add(data.MobilePhone)
	return phones
}

// normalizeImportPhone reduces a phone number to its digits (a leading "+" is
// kept) so that "0221 / 123-45" and "0221 12345" compare equal.
func normalizeImportPhone(raw string) string {
	var b strings.Builder
	for i, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// guardianLanguagePreference returns a language preference, defaulting to "de"
func guardianLanguagePreference(val string) string {
	if val == "" {
		return "de"
	}
	return strings.ToLower(strings.TrimSpace(val))
}
