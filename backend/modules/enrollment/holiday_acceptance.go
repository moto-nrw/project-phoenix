package enrollment

// The acceptance mail of a holiday registration (#6672): when the
// Träger-Büro confirms a registration, the parent who registered gets one
// mail of the Träger with the children, their weeks and time models and the
// text the Träger wrote for the period. Without a text the mail has its
// fixed part only. A parent with an account gets the link to the parents
// portal, which the root adds when Delivery sends the mail: the Träger
// transaction of the confirmation does not read accounts.

// MailKindHolidayRegistrationAccepted is the Delivery outbox kind of the
// acceptance mail; the root registers its renderer under the same kind.
const MailKindHolidayRegistrationAccepted = "holiday_registration_accepted"

// HolidayAcceptanceMailTextMaxLength bounds the text of the acceptance mail
// of a period in characters.
const HolidayAcceptanceMailTextMaxLength = 2000

// HolidayRegistrationAcceptedMail is the payload of the acceptance mail. It
// names the children with their weeks and time models, no date of birth and
// no health information. TraegerText is the period's text as the Träger
// wrote it, plain text. ParentAppURL is the link to the parents portal for a
// parent with an account, empty otherwise; the root sets it at sending, the
// outbox never carries it.
type HolidayRegistrationAcceptedMail struct {
	Recipient    string                         `json:"recipient_email"`
	GuardianName string                         `json:"guardian_name"`
	TraegerName  string                         `json:"traeger_name"`
	PeriodName   string                         `json:"period_name"`
	Binding      bool                           `json:"binding"`
	Children     []HolidayRegistrationMailChild `json:"children"`
	TraegerText  string                         `json:"traeger_text,omitempty"`
	ParentAppURL string                         `json:"parent_app_url,omitempty"`
}
