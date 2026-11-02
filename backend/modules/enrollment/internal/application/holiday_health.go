package application

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Sealing and opening the health information of a holiday registration
// (#4722). The registration splits it off before the store sees the answers:
// the fixed health fields and every answer to a question its form version
// marks as health information go into one sealed value per child and one per
// family; the plaintext answers keep only the questions without health
// information.

// holidayHealthSplit is the health information of one registration before
// it is sealed: the family's and each child's, in the order of the children.
type holidayHealthSplit struct {
	family   enrollment.HolidayHealthRecord
	children []enrollment.HolidayHealthRecord
}

// splitHolidayHealth moves the health information out of a checked
// registration. The returned input carries no fixed health field and no
// answer to a health question.
func splitHolidayHealth(input enrollment.HolidayRegistrationInput, form enrollment.HolidayPeriodForm) (holidayHealthSplit, enrollment.HolidayRegistrationInput) {
	healthKeys := map[string]bool{}
	for _, field := range form.Fields {
		if field.Health {
			healthKeys[field.Key] = true
		}
	}
	split := holidayHealthSplit{children: make([]enrollment.HolidayHealthRecord, len(input.Children))}
	split.family.Answers, input.CustomData = takeHolidayHealthAnswers(input.CustomData, healthKeys)
	children := make([]enrollment.HolidayChildInput, len(input.Children))
	for i, child := range input.Children {
		record := enrollment.HolidayHealthRecord{Allergies: child.Allergies, Medication: child.Medication, HealthNotes: child.HealthNotes}
		record.Answers, child.CustomData = takeHolidayHealthAnswers(child.CustomData, healthKeys)
		child.Allergies, child.Medication, child.HealthNotes = "", "", ""
		split.children[i] = record
		children[i] = child
	}
	input.Children = children
	return split, input
}

func takeHolidayHealthAnswers(answers enrollment.HolidayAnswers, healthKeys map[string]bool) (health, rest enrollment.HolidayAnswers) {
	rest = make(enrollment.HolidayAnswers, len(answers))
	for key, value := range answers {
		if healthKeys[key] {
			if health == nil {
				health = enrollment.HolidayAnswers{}
			}
			health[key] = value
			continue
		}
		rest[key] = value
	}
	return health, rest
}

// sealHolidayHealth seals the split health information of a registration
// for its rows; a record without health information stays nil.
func sealHolidayHealth(cipher enrollment.HolidayHealthCipher, organizationID, registrationID int64, split holidayHealthSplit) (enrollment.HolidaySealedHealth, error) {
	sealed := enrollment.HolidaySealedHealth{Children: make([][]byte, len(split.children))}
	var err error
	if sealed.Family, err = sealHolidayHealthRecord(cipher, split.family,
		enrollment.HolidayFamilyHealthContext(organizationID, registrationID)); err != nil {
		return enrollment.HolidaySealedHealth{}, err
	}
	for i, record := range split.children {
		if sealed.Children[i], err = sealHolidayHealthRecord(cipher, record,
			enrollment.HolidayChildHealthContext(organizationID, registrationID, i+1)); err != nil {
			return enrollment.HolidaySealedHealth{}, err
		}
	}
	return sealed, nil
}

func sealHolidayHealthRecord(cipher enrollment.HolidayHealthCipher, record enrollment.HolidayHealthRecord, context []byte) ([]byte, error) {
	if record.Empty() {
		return nil, nil
	}
	return sealHealthJSON(cipher, record, context)
}

// sealHealthJSON seals the JSON of a health value for its context.
func sealHealthJSON(cipher enrollment.HolidayHealthCipher, value any, context []byte) ([]byte, error) {
	plaintext, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("enrollment: encode holiday health information: %w", err)
	}
	sealed, err := cipher.Seal(plaintext, context)
	if err != nil {
		return nil, fmt.Errorf("enrollment: seal holiday health information: %w", err)
	}
	return sealed, nil
}

// openHealthJSON opens a sealed health value into target and reports
// whether there was one. A value that does not open is
// ErrHolidayHealthUnreadable.
func openHealthJSON(cipher enrollment.HolidayHealthCipher, sealed, context []byte, target any) (bool, error) {
	if len(sealed) == 0 {
		return false, nil
	}
	plaintext, err := cipher.Open(sealed, context)
	if err != nil {
		return false, fmt.Errorf("%w: %w", enrollment.ErrHolidayHealthUnreadable, err)
	}
	if err := json.Unmarshal(plaintext, target); err != nil {
		return false, fmt.Errorf("%w: %w", enrollment.ErrHolidayHealthUnreadable, err)
	}
	return true, nil
}

// openHolidayHealthRecord opens one sealed value; none opens as an empty
// record.
func openHolidayHealthRecord(cipher enrollment.HolidayHealthCipher, sealed, context []byte) (enrollment.HolidayHealthRecord, error) {
	var record enrollment.HolidayHealthRecord
	_, err := openHealthJSON(cipher, sealed, context, &record)
	return record, err
}

// holidayHostHealth names the health information of a child for the hosting
// school: the fixed fields, and each shared health question of the
// registration's version, in form order, with the answer of the child or
// the family, as the participant list names an answer. Nil without any
// value.
func holidayHostHealth(fields []enrollment.HolidayFormField, family, child enrollment.HolidayHealthRecord) *enrollment.HolidayHostHealth {
	health := enrollment.HolidayHostHealth{
		Allergies: child.Allergies, Medication: child.Medication, HealthNotes: child.HealthNotes,
		Answers: []enrollment.HolidayHostAnswer{},
	}
	for _, field := range fields {
		answers := family.Answers
		if field.AppliesToCh {
			answers = child.Answers
		}
		raw, ok := answers[field.Key]
		if !ok {
			continue
		}
		health.Answers = append(health.Answers, enrollment.HolidayHostAnswer{
			Key: field.Key, Label: field.Label, Type: field.Type, Value: holidayHostAnswerValue(field.FormField, raw),
		})
	}
	if health.Empty() {
		return nil
	}
	return &health
}

// holidayHostAnswerValue names an answer as the hosting school's function
// does: a boolean for a yes/no question, the label of the chosen option for
// a selection, the text otherwise, and null for an answer of another type.
func holidayHostAnswerValue(field enrollment.FormField, raw json.RawMessage) json.RawMessage {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return json.RawMessage("null")
	}
	if field.Type == enrollment.FormFieldBoolean {
		if _, ok := value.(bool); ok {
			return raw
		}
		return json.RawMessage("null")
	}
	text, ok := value.(string)
	if !ok {
		return json.RawMessage("null")
	}
	if field.Type == enrollment.FormFieldSelect {
		for _, option := range field.Options {
			if option.Value == text {
				text = option.Label
				break
			}
		}
	}
	encoded, err := json.Marshal(text)
	if err != nil {
		return json.RawMessage("null")
	}
	return encoded
}

// errHolidayHealthCipherRequired is a composition fault: the health
// information cannot be sealed or opened without the key ring.
var errHolidayHealthCipherRequired = errors.New("enrollment: the holiday health cipher is required")

// openHolidayHostHealth opens the sealed health information of a
// participant for the hosting school; nil without any.
func openHolidayHostHealth(cipher enrollment.HolidayHealthCipher, sealed *enrollment.HolidaySealedHostHealth) (*enrollment.HolidayHostHealth, error) {
	if sealed == nil {
		return nil, nil
	}
	child, err := openHolidayHealthRecord(cipher, sealed.Child,
		enrollment.HolidayChildHealthContext(sealed.OrganizationID, sealed.RegistrationID, sealed.Position))
	if err != nil {
		return nil, err
	}
	family, err := openHolidayHealthRecord(cipher, sealed.Family,
		enrollment.HolidayFamilyHealthContext(sealed.OrganizationID, sealed.RegistrationID))
	if err != nil {
		return nil, err
	}
	return holidayHostHealth(sealed.Fields, family, child), nil
}

// openHostHealth opens the health information of a guest copy; nil without
// any.
func openHostHealth(cipher enrollment.HolidayHealthCipher, sealed, context []byte) (*enrollment.HolidayHostHealth, error) {
	var health enrollment.HolidayHostHealth
	found, err := openHealthJSON(cipher, sealed, context, &health)
	if err != nil || !found {
		return nil, err
	}
	if health.Answers == nil {
		health.Answers = []enrollment.HolidayHostAnswer{}
	}
	return &health, nil
}

func sameHolidayHostHealth(a, b *enrollment.HolidayHostHealth) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	encodedA, errA := json.Marshal(a)
	encodedB, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(encodedA) == string(encodedB)
}

// holidayGuestHealthText is the health information of a guest as it stands
// in the health field of its child: one line per value, named as the parents
// saw it in the form. Nil clears the field.
func holidayGuestHealthText(health *enrollment.HolidayHostHealth) *string {
	if health == nil || health.Empty() {
		return nil
	}
	lines := []string{"Aus der Ferienanmeldung:"}
	add := func(label, value string) {
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	add("Allergien", health.Allergies)
	add("Medikamente", health.Medication)
	add("Weitere Hinweise zur Gesundheit", health.HealthNotes)
	for _, answer := range health.Answers {
		add(answer.Label, holidayAnswerText(answer.Value))
	}
	text := strings.Join(lines, "\n")
	return &text
}

// holidayAnswerText reads a named answer as text: "Ja" or "Nein" for a
// yes/no question, the text otherwise, nothing for no answer.
func holidayAnswerText(value json.RawMessage) string {
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		return ""
	}
	switch typed := decoded.(type) {
	case bool:
		if typed {
			return "Ja"
		}
		return "Nein"
	case string:
		return typed
	}
	return ""
}
