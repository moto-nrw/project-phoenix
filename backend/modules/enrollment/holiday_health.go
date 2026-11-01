package enrollment

import (
	"errors"
	"fmt"
)

// Health information of a holiday registration (#4722, user decision of
// 01.11.2026 "keine Datensilos", ADR 0116 and ADR 0120 addenda). Allergies,
// medication, further health notes and the answers to questions marked as
// health information are sealed at rest: the database holds them only as
// values Enrollment seals and opens with HolidayHealthCipher. In the Träger
// portal only the right traeger_health:read opens them. The school that
// hosts a week sees them for its confirmed children until the last day of
// the week, when the parents agreed under consent texts of
// HolidayConsentVersionHostHealth; a holiday guest carries them into its
// health field and its sheet until its contacts go.

// HolidayConsentVersionHostHealth is the version of the consent texts of the
// public registration that names the hosting OGS as a reader of the health
// information, from when on and until when. A registration submitted under
// an earlier version (stored as none) shares no health information.
const HolidayConsentVersionHostHealth = 2

// ErrHolidayHealthUnreadable reports a sealed value that does not open: a
// missing key of the ring or a changed value. It is a server error, never a
// reason to answer without the value.
var ErrHolidayHealthUnreadable = errors.New("holiday health information does not open")

// HolidayHealthCipher seals and opens the health information. context binds
// a value to its row; a value opens only with the context it was sealed
// with. The root binds Security Runtime's field cipher.
type HolidayHealthCipher interface {
	Seal(plaintext, context []byte) ([]byte, error)
	Open(sealed, context []byte) ([]byte, error)
}

// HolidayHealthRecord is what one sealed value holds: for a child the fixed
// health fields and the answers to the child questions marked as health
// information, for a family only the answers to its health questions.
type HolidayHealthRecord struct {
	Allergies   string         `json:"allergies,omitempty"`
	Medication  string         `json:"medication,omitempty"`
	HealthNotes string         `json:"health_notes,omitempty"`
	Answers     HolidayAnswers `json:"answers,omitempty"`
}

// Empty reports a record without any health information.
func (r HolidayHealthRecord) Empty() bool {
	return r.Allergies == "" && r.Medication == "" && r.HealthNotes == "" && len(r.Answers) == 0
}

// HolidayChildHealthContext binds the sealed value of a child to its Träger,
// registration and position. The migration of #4722 seals with the same.
func HolidayChildHealthContext(organizationID, registrationID int64, position int) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_registration_children.health_data:%d:%d:%d", organizationID, registrationID, position)
}

// HolidayFamilyHealthContext binds the sealed value of a family to its
// Träger and registration.
func HolidayFamilyHealthContext(organizationID, registrationID int64) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_registrations.health_data:%d:%d", organizationID, registrationID)
}

// HolidayGuestHealthContext binds the sealed value of a guest copy to the
// hosting school and the registered child it copies.
func HolidayGuestHealthContext(schoolID, childID int64) []byte {
	return fmt.Appendf(nil, "enrollment.holiday_guests.health_data:%d:%d", schoolID, childID)
}

// HolidayHostHealth is the health information the hosting school sees of a
// child: the fixed fields and the answers to the health questions the
// Träger shares with it, named as in the participant list.
type HolidayHostHealth struct {
	Allergies   string              `json:"allergies"`
	Medication  string              `json:"medication"`
	HealthNotes string              `json:"health_notes"`
	Answers     []HolidayHostAnswer `json:"answers"`
}

// Empty reports health information without any value.
func (h HolidayHostHealth) Empty() bool {
	return h.Allergies == "" && h.Medication == "" && h.HealthNotes == "" && len(h.Answers) == 0
}

// HolidayRegistrationHealthDetail is the health information of one
// registration as the Träger-Büro sees it with traeger_health:read: the
// family's answers to its health questions and each child's health fields
// and answers.
type HolidayRegistrationHealthDetail struct {
	CustomData HolidayAnswers              `json:"custom_data"`
	Children   []HolidayRegistrationHealth `json:"children"`
}

// HolidaySealedHealth are the sealed values of one registration as the
// store writes them: the family's and each child's in the order of the
// children, nil where there is no health information.
type HolidaySealedHealth struct {
	Family   []byte
	Children [][]byte
}

// HolidaySealedRegistrationHealth is the sealed health information of one
// registration as the store reads it for the Träger-Büro: the family's value
// and each child's, in the order of the children, with what each is bound to.
type HolidaySealedRegistrationHealth struct {
	Family   []byte
	Children []HolidaySealedChildHealth
}

// HolidaySealedChildHealth is the sealed value of one child, nil without
// health information.
type HolidaySealedChildHealth struct {
	ChildID  int64
	Position int
	Sealed   []byte
}

// HolidaySealedHostHealth is the sealed health information of one
// participant as the hosting school's function returns it: the child's and
// the family's value, what they are bound to, and the shared health
// questions of the registration's version that name the answers.
type HolidaySealedHostHealth struct {
	OrganizationID int64
	RegistrationID int64
	Position       int
	Child          []byte
	Family         []byte
	Fields         []HolidayFormField
}
