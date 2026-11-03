package enrollment

import "time"

// The declaration of a holiday registration (#5626). Parents tick the fixed
// consents, confirm the custody and type their name to give the
// declarations. It is no signature and no written form: the name only
// documents who gave the declarations and when. Enrollment holds the German
// texts of each version; the form shows them, the receipt mail repeats them
// word for word, and the Träger-Büro reads them beside the registration.

// HolidayConsentVersionCurrent is the version of the consent texts every new
// registration agrees to. Version 3 keeps the texts of version 2 and adds
// the custody and the name as the declaration. It is at least
// HolidayConsentVersionHostHealth, so the hosting OGS sees the health
// information of a registration under it.
const HolidayConsentVersionCurrent = 3

// HolidayDeclarationNameMaxLength bounds the name a parent types as the
// declaration, like the name of the parent.
const HolidayDeclarationNameMaxLength = 100

// HolidayConsentTexts are the German texts of one version of the fixed
// declarations. A text a version does not have is empty.
type HolidayConsentTexts struct {
	Version          int    `json:"version"`
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
	holidayHealthConsentText = "Ich bin einverstanden: Der Träger und die OGS sehen die Gesundheitsangaben meiner Kinder. " +
		"Es geht um die OGS, die ein Kind in einer Ferienwoche betreut. " +
		"Sie sieht die Angaben ab der Bestätigung bis zum letzten Tag der Woche."
	holidayBindingConfirmedText = "Ich weiß, dass die Anmeldung verbindlich ist."
	holidayCustodyConfirmedText = "Ich darf die Anmeldung allein abgeben: " +
		"Ich bin allein sorgeberechtigt oder die andere sorgeberechtigte Person ist einverstanden."
	holidayDeclarationNameText = "Mit Ihrem Namen geben Sie die Erklärungen oben ab."
)

// holidayConsentTexts are the known versions. Version 1 is missing: its
// texts lived only in the form and were not kept (#4722).
var holidayConsentTexts = map[int]HolidayConsentTexts{
	HolidayConsentVersionHostHealth: {
		Version: HolidayConsentVersionHostHealth, PrivacyConsent: holidayPrivacyConsentText,
		HealthConsent: holidayHealthConsentText, BindingConfirmed: holidayBindingConfirmedText,
	},
	HolidayConsentVersionCurrent: {
		Version: HolidayConsentVersionCurrent, PrivacyConsent: holidayPrivacyConsentText,
		HealthConsent: holidayHealthConsentText, BindingConfirmed: holidayBindingConfirmedText,
		CustodyConfirmed: holidayCustodyConfirmedText, DeclarationName: holidayDeclarationNameText,
	},
}

// HolidayConsentTextsOf returns the texts of a version; false for a version
// whose texts are not kept, such as a registration from before #4722.
func HolidayConsentTextsOf(version int) (HolidayConsentTexts, bool) {
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
