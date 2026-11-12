package enrollment

import (
	"context"
	"fmt"
	"time"
)

// The declaration of a holiday registration (#5626). Parents tick the fixed
// consents, confirm the custody and type their name to give the
// declarations. It is no signature and no written form: the name only
// documents who gave the declarations and when. Enrollment holds the German
// texts of each version; the form shows them, the receipt mail repeats them
// word for word, and the Träger-Büro reads them beside the registration.

// HolidayConsentVersionCurrent is the version of the consent texts every new
// registration agrees to. Version 3 keeps the texts of version 2 and adds
// the custody and the name as the declaration. Version 4 names the retention
// of the Träger instead of the fixed three months and says when the health
// information goes (#6691). It is at least HolidayConsentVersionHostHealth,
// so the hosting OGS sees the health information of a registration under it.
// Version 5 adds that the Träger keeps the customer number of the Gemeinde
// for later registrations and when it goes (#6692).
const HolidayConsentVersionCurrent = HolidayConsentVersionDebtorNumber

// HolidayConsentVersionDebtorNumber is the first version whose privacy text
// says that the Träger keeps the debtor number of the family (#6692).
const HolidayConsentVersionDebtorNumber = 5

// HolidayConsentVersionRetention is the first version whose privacy text
// names the retention of the registration in days (#6691). The wording of a
// registration under it is this version with the days the registration
// keeps: another retention is another wording.
const HolidayConsentVersionRetention = 4

// Retention of a holiday registration (#6691, ADR 0118 addendum).
const (
	// HolidayLegacyRetentionDays is what the texts before
	// HolidayConsentVersionRetention promise: three months after the last
	// week of the period (spec #4611 user story 121).
	HolidayLegacyRetentionDays = 92
	// HolidayHealthRetentionDays is how long the health information of a
	// registration stays after the last week of its period, whatever the
	// registration itself keeps (Art. 9 GDPR): it serves the care in the
	// holiday weeks and nothing after them.
	HolidayHealthRetentionDays = 92
	// HolidayRetentionDaysMin and HolidayRetentionDaysMax bound the
	// retention a Träger may set; the setting's default is a year.
	HolidayRetentionDaysMin = 92
	HolidayRetentionDaysMax = 1095
)

// HolidayDeclarationNameMaxLength bounds the name a parent types as the
// declaration, like the name of the parent.
const HolidayDeclarationNameMaxLength = 100

// HolidayRetentionSettings reads the Träger's
// holiday_registration.retention_days in the caller's Träger transaction
// (#6691). The root binds the Träger level of the settings system.
type HolidayRetentionSettings interface {
	HolidayRegistrationRetentionDays(ctx context.Context, organizationID int64) (int, error)
}

// HolidayConsentTexts are the German texts of one version of the fixed
// declarations. A text a version does not have is empty.
type HolidayConsentTexts struct {
	Version int `json:"version"`
	// RetentionDays are the days after the last week of the period the
	// privacy text promises (#6691): HolidayLegacyRetentionDays for the
	// versions with the fixed three months.
	RetentionDays    int    `json:"retention_days"`
	PrivacyConsent   string `json:"privacy_consent"`
	HealthConsent    string `json:"health_consent"`
	BindingConfirmed string `json:"binding_confirmed"`
	CustodyConfirmed string `json:"custody_confirmed"`
	DeclarationName  string `json:"declaration_name"`
}

const (
	holidayPrivacyConsentText = "Ich bin einverstanden, dass der Träger die Angaben für die Ferienbetreuung nutzt. " +
		"Er löscht sie drei Monate nach den Ferien. Die OGS, die mein Kind in einer Ferienwoche betreut, " +
		"sieht Name und Telefon der zweiten sorgeberechtigten Person. Sie sieht auch die Antworten, " +
		"die der Träger für sie freigibt. Gesundheitsangaben sieht sie nur, wenn ich zustimme. " +
		"Die OGS sieht das schon vor der Ferienwoche. So kann sie sich vorbereiten. " +
		"Die Liste der Kinder sieht sie bis zum letzten Tag der Woche. " +
		"Name, Telefon und Antworten kann sie danach höchstens 7 Tage lesen. Dann werden sie gelöscht."
	// holidayRetentionPrivacyConsentText is the privacy text from
	// HolidayConsentVersionRetention on; its two numbers are the retention of
	// the registration and of the health information in days.
	holidayRetentionPrivacyConsentText = "Ich bin einverstanden, dass der Träger die Angaben für die Ferienbetreuung nutzt. " +
		"Er löscht sie %d Tage nach den Ferien. Gesundheitsangaben löscht er schon %d Tage nach den Ferien. " +
		"Die OGS, die mein Kind in einer Ferienwoche betreut, " +
		"sieht Name und Telefon der zweiten sorgeberechtigten Person. Sie sieht auch die Antworten, " +
		"die der Träger für sie freigibt. Gesundheitsangaben sieht sie nur, wenn ich zustimme. " +
		"Die OGS sieht das schon vor der Ferienwoche. So kann sie sich vorbereiten. " +
		"Die Liste der Kinder sieht sie bis zum letzten Tag der Woche. " +
		"Name, Telefon und Antworten kann sie danach höchstens 7 Tage lesen. Dann werden sie gelöscht."
	// holidayDebtorNumberConsentText follows the privacy text from
	// HolidayConsentVersionDebtorNumber on: the number the family keeps at
	// the Träger goes on 1 January of the fourth year after it was last
	// needed (parentbilling.FamilyDebtorNumberRetentionYears), or after the
	// invoices of the family the Träger must keep.
	holidayDebtorNumberConsentText = "Bekommt meine Rechnung eine Kundennummer der Gemeinde (Debitorennummer), " +
		"behält der Träger sie für spätere Anmeldungen. Er löscht sie am Ende des dritten Jahres nach der letzten Anmeldung. " +
		"Muss er Rechnungen länger aufbewahren, löscht er sie danach."
	holidayHealthConsentText = "Ich bin einverstanden: Der Träger und die OGS sehen die Gesundheitsangaben meiner Kinder. " +
		"Es geht um die OGS, die ein Kind in einer Ferienwoche betreut. " +
		"Sie sieht die Angaben ab der Bestätigung bis zum letzten Tag der Woche."
	holidayBindingConfirmedText = "Ich weiß, dass die Anmeldung verbindlich ist."
	holidayCustodyConfirmedText = "Ich darf die Anmeldung allein abgeben: " +
		"Ich bin allein sorgeberechtigt oder die andere sorgeberechtigte Person ist einverstanden."
	holidayDeclarationNameText = "Mit Ihrem Namen geben Sie die Erklärungen oben ab."
)

// holidayConsentTexts are the known versions with a fixed wording. Version 1
// is missing: its texts lived only in the form and were not kept (#4722).
var holidayConsentTexts = map[int]HolidayConsentTexts{
	HolidayConsentVersionHostHealth: {
		Version: HolidayConsentVersionHostHealth, RetentionDays: HolidayLegacyRetentionDays, PrivacyConsent: holidayPrivacyConsentText,
		HealthConsent: holidayHealthConsentText, BindingConfirmed: holidayBindingConfirmedText,
	},
	holidayConsentVersionDeclaration: {
		Version: holidayConsentVersionDeclaration, RetentionDays: HolidayLegacyRetentionDays, PrivacyConsent: holidayPrivacyConsentText,
		HealthConsent: holidayHealthConsentText, BindingConfirmed: holidayBindingConfirmedText,
		CustodyConfirmed: holidayCustodyConfirmedText, DeclarationName: holidayDeclarationNameText,
	},
}

// holidayConsentVersionDeclaration is the version that added the custody and
// the typed name (#5626); it still promises the fixed three months.
const holidayConsentVersionDeclaration = 3

// HolidayConsentTextsOf returns the texts of a version as a registration
// that keeps retentionDays agreed to them; false for a version whose texts
// are not kept, such as a registration from before #4722. The versions
// before HolidayConsentVersionRetention have one wording and ignore
// retentionDays.
func HolidayConsentTextsOf(version, retentionDays int) (HolidayConsentTexts, bool) {
	if version == HolidayConsentVersionRetention || version == HolidayConsentVersionDebtorNumber {
		texts := holidayConsentTexts[holidayConsentVersionDeclaration]
		texts.Version, texts.RetentionDays = version, retentionDays
		texts.PrivacyConsent = fmt.Sprintf(holidayRetentionPrivacyConsentText, retentionDays, HolidayHealthRetentionDays)
		if version == HolidayConsentVersionDebtorNumber {
			texts.PrivacyConsent += " " + holidayDebtorNumberConsentText
		}
		return texts, true
	}
	texts, ok := holidayConsentTexts[version]
	return texts, ok
}

// HolidayDeclarationStatement is one declaration the parent gave, as the
// receipt mail repeats it: the fixed declarations with their text, a
// confirmed notice of the Träger with the sentence the parent ticked and its
// text.
type HolidayDeclarationStatement struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Detail string `json:"detail,omitempty"`
}

// HolidayRegistrationMailDeclaration is the declaration of the receipt mail:
// the typed name, when it was given, the version of the texts and every
// declaration in its words. It carries no health information.
type HolidayRegistrationMailDeclaration struct {
	Name           string                        `json:"name"`
	GivenAt        time.Time                     `json:"given_at"`
	ConsentVersion int                           `json:"consent_version"`
	Statements     []HolidayDeclarationStatement `json:"statements"`
}
