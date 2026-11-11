package enrollmenthttp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The file of one holiday registration (#6691): everything the family
// entered, the booked weeks, the state and the declarations word for word
// with name and time, as a block document. It names the health information
// only when the record carries it.

const (
	holidayRecordFooter   = "Vertraulich. Enthält personenbezogene Daten."
	holidayRecordDateTime = "02.01.2006 15:04"
	holidayRecordDate     = "02.01.2006"
)

var holidayPlacementLabels = map[string]string{
	capability.HolidayPlacementPlaced:     "Platz",
	capability.HolidayPlacementWaitlisted: "Warteliste",
	capability.HolidayPlacementPending:    "Entscheidung offen",
}

var holidayCriterionLabels = map[string]string{
	capability.HolidayCriterionYes:     "Ja",
	capability.HolidayCriterionNo:      "Nein",
	capability.HolidayCriterionUnknown: "Keine Angabe",
}

// holidayRecordNames names what a file needs beyond the record: the Träger's
// schools by ID and the instant the file was made.
type holidayRecordNames struct {
	schools     map[int64]string
	generatedAt time.Time
}

func (n holidayRecordNames) school(id int64) string {
	if name, ok := n.schools[id]; ok {
		return name
	}
	return "Schule nicht mehr vorhanden"
}

func holidayYesNo(value bool) string {
	if value {
		return "Ja"
	}
	return "Nein"
}

func holidayRecordTime(at time.Time) string {
	return at.In(calendar.Berlin).Format(holidayRecordDateTime) + " Uhr"
}

// field is one line of the file; user text loses the renderer's markers.
func field(label, value string) lists.Field {
	return lists.Field{Label: lists.SanitizeUserText(label), Value: lists.SanitizeUserText(value)}
}

// optional appends the line only when it has a value.
func optional(fields []lists.Field, label, value string) []lists.Field {
	if strings.TrimSpace(value) == "" {
		return fields
	}
	return append(fields, field(label, value))
}

// holidayRegistrationRecordDocument builds the file of one registration.
func holidayRegistrationRecordDocument(period capability.HolidayPeriod, record capability.HolidayRegistrationRecord, names holidayRecordNames) lists.RecordDocument {
	detail := record.Detail
	records := []lists.Record{holidayRecordState(period, detail), holidayRecordParents(record)}
	for index := range detail.Children {
		records = append(records, holidayRecordChild(record, index, names))
	}
	if contacts := holidayRecordContacts(detail.Contacts); len(contacts.Fields) > 0 {
		records = append(records, contacts)
	}
	if criteria := holidayRecordFamilyCriteria(detail); len(criteria.Fields) > 0 {
		records = append(records, criteria)
	}
	records = append(records, holidayRecordDeclaration(detail))
	return lists.RecordDocument{
		Title:       "Ferienanmeldung Nr. " + strconv.FormatInt(detail.ID, 10),
		Subtitle:    lists.SanitizeUserText(period.Name),
		GeneratedAt: names.generatedAt, Footer: holidayRecordFooter, Records: records,
	}
}

func holidayRecordState(period capability.HolidayPeriod, detail capability.HolidayRegistrationDetail) lists.Record {
	fields := []lists.Field{
		field("Ferien", period.Name),
		field("Status", holidayRegistrationStatusLabels[detail.Status]),
		field("Eingegangen am", holidayRecordTime(detail.SubmittedAt)),
		field("Eingefroren", holidayYesNo(detail.Frozen)),
	}
	fields = optional(fields, "Debitorennummer", detail.DebtorNumber)
	fields = append(fields, field("Leistungen angegeben", holidayYesNo(detail.BenefitsDeclared)))
	if detail.BenefitsDeclared {
		fields = append(fields, field("Nachweis liegt vor", holidayYesNo(detail.BenefitsProofAttached)))
	}
	if detail.DeletesOn != nil {
		fields = append(fields, field("Löschung in moto am", detail.DeletesOn.Format(holidayRecordDate)))
	}
	return lists.Record{Title: "Anmeldung", Fields: fields}
}

func holidayRecordParents(record capability.HolidayRegistrationRecord) lists.Record {
	detail := record.Detail
	fields := []lists.Field{
		field("Name", detail.GuardianFirstName+" "+detail.GuardianLastName),
		field("E-Mail", detail.GuardianEmail),
		field("Telefon", detail.GuardianPhone),
	}
	if address := detail.GuardianAddress; address != nil {
		fields = append(fields, field("Anschrift", address.Street+", "+address.PostalCode+" "+address.City))
	}
	fields = append(fields, holidayRecordAnswers(detail.Form, false, false, detail.CustomData)...)
	parents := lists.Record{Title: "Eltern", Fields: fields}
	if second := detail.SecondGuardian; second != nil {
		sub := []lists.Field{field("Name", second.FirstName+" "+second.LastName)}
		sub = optional(optional(sub, "E-Mail", second.Email), "Telefon", second.Phone)
		parents.Subs = append(parents.Subs, lists.SubRecord{Title: "Zweite sorgeberechtigte Person", Fields: sub})
	}
	if record.Health != nil {
		if health := holidayRecordAnswers(detail.Form, false, true, record.Health.CustomData); len(health) > 0 {
			parents.Subs = append(parents.Subs, lists.SubRecord{Title: "Gesundheitsangaben der Familie", Fields: health})
		}
	}
	return parents
}

// holidayRecordAnswers are the answers to the questions of the form for the
// family or a child, the health questions or the others, in form order. A
// question without an answer is left out.
func holidayRecordAnswers(form *capability.HolidayPeriodForm, child, health bool, answers capability.HolidayAnswers) []lists.Field {
	fields := []lists.Field{}
	if form == nil {
		return fields
	}
	for _, question := range form.Fields {
		raw, ok := answers[question.Key]
		if !ok || question.AppliesToCh != child || question.Health != health {
			continue
		}
		fields = optional(fields, question.Label, holidayAnswerText(question, raw))
	}
	return fields
}

func holidayRecordChild(record capability.HolidayRegistrationRecord, index int, names holidayRecordNames) lists.Record {
	detail, child := record.Detail, record.Detail.Children[index]
	school := "Feriengast"
	if child.HomeSchoolID != nil {
		school = names.school(*child.HomeSchoolID)
	} else if child.GuestSchoolName != "" {
		school = "Feriengast (" + child.GuestSchoolName + ")"
	}
	fields := []lists.Field{
		field("Geburtsdatum", child.BirthDate.Format(holidayRecordDate)),
		field("Schule", school),
	}
	fields = append(fields, holidayRecordAnswers(detail.Form, true, false, holidayChildAnswers(detail.ChildCustomData, child.ID))...)
	fields = append(fields, holidayRecordChildCriteria(detail, child.ID)...)
	weeks := make([]lists.Field, len(child.Bookings))
	for i, booking := range child.Bookings {
		weeks[i] = field(holidayExportWeek(booking)+", "+names.school(booking.HostSchoolID),
			booking.TimeModelName+" ("+holidayPlacementLabels[booking.Placement]+")")
	}
	out := lists.Record{
		Title: lists.SanitizeUserText("Kind: " + child.FirstName + " " + child.LastName), Fields: fields,
		Subs: []lists.SubRecord{{Title: "Gebuchte Wochen", Fields: weeks}},
	}
	if health := holidayRecordChildHealth(record, child.ID); len(health) > 0 {
		out.Subs = append(out.Subs, lists.SubRecord{Title: "Gesundheitsangaben", Fields: health})
	}
	return out
}

func holidayChildAnswers(children []capability.HolidayChildAnswers, childID int64) capability.HolidayAnswers {
	for _, child := range children {
		if child.ChildID == childID {
			return child.CustomData
		}
	}
	return nil
}

// holidayRecordChildHealth are the health fields and health answers of one
// child; empty when the record carries no health information.
func holidayRecordChildHealth(record capability.HolidayRegistrationRecord, childID int64) []lists.Field {
	fields := []lists.Field{}
	if record.Health == nil {
		return fields
	}
	for _, health := range record.Health.Children {
		if health.ChildID != childID {
			continue
		}
		fields = optional(fields, "Allergien", health.Allergies)
		fields = optional(fields, "Medikamente", health.Medication)
		fields = optional(fields, "Hinweise zur Gesundheit", health.HealthNotes)
		fields = append(fields, holidayRecordAnswers(record.Detail.Form, true, true, health.CustomData)...)
	}
	return fields
}

func holidayRecordContacts(contacts []capability.HolidayRegistrationContact) lists.Record {
	kinds := map[string]string{capability.HolidayContactEmergency: "Notfallkontakt", capability.HolidayContactPickup: "Abholberechtigt"}
	fields := make([]lists.Field, len(contacts))
	for i, contact := range contacts {
		parts := []string{contact.Name}
		for _, part := range []string{contact.Relationship, contact.Phone} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		fields[i] = field(kinds[contact.Kind], strings.Join(parts, ", "))
	}
	return lists.Record{Title: "Notfallkontakte und Abholberechtigte", Fields: fields}
}

func holidayCriterionField(criterion capability.HolidayRegistrationCriterion) lists.Field {
	return field(criterion.Question, holidayCriterionLabels[criterion.Stated])
}

func holidayRecordFamilyCriteria(detail capability.HolidayRegistrationDetail) lists.Record {
	fields := []lists.Field{}
	for _, criterion := range detail.Criteria {
		if criterion.ChildID == nil {
			fields = append(fields, holidayCriterionField(criterion))
		}
	}
	return lists.Record{Title: "Angaben zur Platzvergabe", Fields: fields}
}

func holidayRecordChildCriteria(detail capability.HolidayRegistrationDetail, childID int64) []lists.Field {
	fields := []lists.Field{}
	for _, criterion := range detail.Criteria {
		if criterion.ChildID != nil && *criterion.ChildID == childID {
			fields = append(fields, holidayCriterionField(criterion))
		}
	}
	return fields
}

// holidayRecordDeclaration is the declaration of the parents: the typed
// name, when it was given, the version of the texts and every declaration
// given in its words with its time.
func holidayRecordDeclaration(detail capability.HolidayRegistrationDetail) lists.Record {
	fields := optional([]lists.Field{}, "Name", detail.DeclarationName)
	fields = append(fields, field("Abgegeben am", holidayRecordTime(detail.PrivacyConsentAt)))
	record := lists.Record{Title: "Erklärung der Eltern"}
	texts := detail.ConsentTexts
	if texts == nil {
		record.Fields = append(fields, field("Wortlaut", "Der Wortlaut dieser Fassung ist nicht gespeichert."))
		return record
	}
	version := fmt.Sprintf("Fassung %d", texts.Version)
	if texts.Version >= capability.HolidayConsentVersionRetention {
		// Only these versions name the days; an earlier one says "drei Monate".
		version += fmt.Sprintf(", Löschung %d Tage nach den Ferien", texts.RetentionDays)
	}
	record.Fields = append(fields, field("Fassung der Texte", version))
	given := func(title, text string, at *time.Time) {
		if at == nil || text == "" {
			return
		}
		record.Subs = append(record.Subs, lists.SubRecord{Title: title, Fields: []lists.Field{
			field("Erklärt am", holidayRecordTime(*at)), field("Wortlaut", text),
		}})
	}
	given("Datenschutz", texts.PrivacyConsent, &detail.PrivacyConsentAt)
	given("Gesundheitsangaben", texts.HealthConsent, detail.HealthConsentAt)
	given("Verbindliche Anmeldung", texts.BindingConfirmed, detail.BindingConfirmedAt)
	given("Sorgerecht", texts.CustodyConfirmed, detail.CustodyConfirmedAt)
	if detail.Form != nil {
		for _, block := range detail.Form.LegalBlocks {
			if at, ok := detail.LegalAcceptances[block.Key]; ok {
				given(lists.SanitizeUserText(block.Title), strings.TrimSpace(block.Label+"\n"+strings.TrimSpace(block.Text)), &at)
			}
		}
	}
	return record
}
