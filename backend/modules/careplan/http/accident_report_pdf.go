package careplanhttp

import (
	strings "strings"
	time "time"

	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// accidentReportFacts are the school and child facts the report prints next
// to what the team recorded.
type accidentReportFacts struct {
	SchoolName    string
	SchoolAddress string
	Name          string
	Birthday      string
	Address       string
	SchoolClass   string
}

// accidentNotRecorded fills a field of the official form moto does not
// record; the school completes it on paper before passing the report on.
const accidentNotRecorded = "nicht in moto erfasst"

// accidentReportDocument describes the report in the order and numbering of
// the official form for children in day care, pupils and students (fields 1
// to 22), followed by what the form leaves to the school's own record: first
// aid, parents and supervision.
func accidentReportDocument(file careplan.AccidentReportFile, facts accidentReportFacts, now time.Time) ReportDocument {
	doc := file.Document
	report := careplan.AccidentReport{}
	if doc.AccidentReport != nil {
		report = *doc.AccidentReport
	}
	institution := ReportCard{Title: "Einrichtung und Versicherter", Fields: []ReportField{
		{Label: "1 Name und Anschrift der Einrichtung", Value: orNotRecorded(joinNonEmpty(", ", facts.SchoolName, facts.SchoolAddress))},
		{Label: "2 Träger der Einrichtung", Value: accidentNotRecorded},
		{Label: "3 Unternehmensnummer des Unfallversicherungsträgers", Value: accidentNotRecorded},
		{Label: "4 Empfänger", Value: accidentNotRecorded},
		{Label: "5 Name, Vorname des Versicherten", Value: orNotRecorded(facts.Name)},
		{Label: "6 Geburtsdatum", Value: orNotRecorded(accidentBirthday(facts.Birthday))},
		{Label: "7 Straße, Hausnummer, Postleitzahl, Ort", Value: orNotRecorded(facts.Address)},
		{Label: "Klasse", Value: orNotRecorded(facts.SchoolClass)},
	}}
	accident := ReportCard{Title: "Unfall", Fields: []ReportField{
		{Label: "12 Unfallzeitpunkt", Value: accidentStamp(report.OccurredAt)},
		{Label: "13 Unfallort", Value: report.Location},
		{Label: "14 Ausführliche Schilderung des Unfallhergangs", Value: report.Description},
		{Label: "15 Verletzte Körperteile", Value: orNotRecorded(report.InjuredBodyParts)},
		{Label: "16 Art der Verletzung", Value: report.InjuryKind},
		{Label: "19 Wer hat von dem Unfall zuerst Kenntnis genommen? Zeugen", Value: orNotRecorded(report.Witnesses)},
		{Label: "20 Erstbehandelnde Ärztin, erstbehandelnder Arzt oder Krankenhaus", Value: accidentDoctor(report)},
	}}
	care := ReportCard{Title: "Versorgung und Aufsicht", Fields: []ReportField{
		{Label: "Erste Hilfe", Value: orNotRecorded(report.FirstAid)},
		{Label: "Benachrichtigte Eltern", Value: accidentParents(report)},
		{Label: "Aufsichtsperson", Value: report.Supervisor},
	}}
	cards := []ReportCard{institution, accident, care, accidentSignatureCard(file)}
	title := "Unfallanzeige"
	if doc.AccidentSupersedesID != nil {
		title = "Unfallanzeige (Korrektur)"
	}
	return ReportDocument{
		Title: title, Subtitle: "für Kinder in Tageseinrichtungen, Schüler, Studierende", GeneratedAt: now,
		Filters: []string{facts.Name, "Stand: " + accidentStatusLabel(doc)},
		Footer:  "Vertraulich: enthält Gesundheitsdaten (Art. 9 DSGVO)",
		Sections: []ReportSection{{
			Title: "Angaben zum Unfall", Cards: cards,
		}},
	}
}

// accidentSignatureCard is field 22: the date and the leadership's
// signature, or the note how the report was signed.
func accidentSignatureCard(file careplan.AccidentReportFile) ReportCard {
	doc := file.Document
	card := ReportCard{Title: "22 Datum, Leitung (Beauftragte) der Einrichtung"}
	switch {
	case file.Signature != nil:
		integrity := "Unterschrift und Angaben sind unverändert."
		if !file.Signature.Intact {
			integrity = "Achtung: Die Angaben oder die Unterschrift wurden nachträglich verändert."
		}
		typed := ""
		if file.Signature.TypedName != "" {
			typed = "Name getippt: " + file.Signature.TypedName
		}
		card.Fields = []ReportField{
			{Label: "Datum", Value: accidentStamp(file.Signature.SignedAt)},
			{Label: "Unterschrieben von", Value: file.Signature.SignerName},
			{Label: "Unterschrift", Value: typed, PNG: file.Signature.PNG},
			{Label: "Verfahren", Value: accidentSignatureMethodLabel(file.Signature.Method)},
			{Label: "Prüfung", Value: integrity},
		}
	case doc.AccidentSignatureID != nil:
		card.Fields = []ReportField{{Label: "Prüfung", Value: "Achtung: Die gespeicherte Unterschrift fehlt."}}
	case doc.AccidentSignedAt != nil:
		card.Fields = []ReportField{
			{Label: "Datum", Value: accidentStamp(*doc.AccidentSignedAt)},
			{Label: "Unterschrift", Value: "Das unterschriebene Papierexemplar liegt am Kind unter Dokumente."},
		}
	default:
		card.Fields = []ReportField{
			{Label: "Datum", Value: ""},
			{Label: "Unterschrift", Value: "Noch nicht unterschrieben"},
		}
	}
	return card
}

func accidentSignatureMethodLabel(method string) string {
	if method == "typed" {
		return "Einfache elektronische Signatur mit getipptem Namen"
	}
	return "Einfache elektronische Signatur mit gezeichneter Unterschrift"
}

func accidentStatusLabel(doc careplan.CareDocument) string {
	if careplan.AccidentReportStatus(doc) == careplan.AccidentReportStatusSigned {
		return "unterschrieben"
	}
	return "offen"
}

func accidentDoctor(report careplan.AccidentReport) string {
	if !report.DoctorVisited {
		return "Kein Arztbesuch"
	}
	return report.Doctor
}

func accidentParents(report careplan.AccidentReport) string {
	if report.ParentsNotified == "" {
		return "Noch nicht benachrichtigt"
	}
	if report.ParentsNotifiedAt == nil {
		return report.ParentsNotified
	}
	return report.ParentsNotified + ", " + accidentStamp(*report.ParentsNotifiedAt)
}

func accidentBirthday(birthday string) string {
	date, err := calendar.ParseDate(strings.TrimSpace(birthday))
	if err != nil {
		return strings.TrimSpace(birthday)
	}
	return date.Format("02.01.2006")
}

func accidentStamp(t time.Time) string {
	return t.In(calendar.Berlin).Format("02.01.2006, 15:04") + " Uhr"
}

func orNotRecorded(value string) string {
	if strings.TrimSpace(value) == "" {
		return accidentNotRecorded
	}
	return value
}
