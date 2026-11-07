package application

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The contract letter (#4678, ADR 0153). Subject, salutation, body and
// closing come from the Träger's template of the kind "Vertrag" (#4677,
// ADR 0152), filled with the frozen version. After the body the letter
// always prints the terms of the version as a table, so a template text can
// never drop what the parents agree to.

// Letter is a contract letter in the parts of a letter.
type Letter struct {
	Title       string
	AccentColor string
	Logo        []byte
	Sender      []string
	Recipient   []string
	Facts       []LetterFact
	Subject     string
	Salutation  string
	Blocks      []LetterBlock
	Closing     []string
	Footer      []string
}

// LetterFact is a label with its value.
type LetterFact struct {
	Label string
	Value string
	Bold  bool
}

// LetterBlock is a paragraph with an optional heading and table.
type LetterBlock struct {
	Heading string
	Text    string
	Bold    bool
	Facts   []LetterFact
}

var weekdayNames = map[string]string{
	"mon": "Montag", "tue": "Dienstag", "wed": "Mittwoch", "thu": "Donnerstag",
	"fri": "Freitag", "sat": "Samstag", "sun": "Sonntag",
}

var weekdayOrder = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// feeByRules names the fee of a contract over several modules until the
// tariffs per weekday and module (#4684) price it.
const feeByRules = "laut Beitragsordnung für die gebuchten Module"

// feeByStatute names the fee of a contract whose only module has no price,
// so the frozen PDF never prints an empty amount.
const feeByStatute = "laut Beitragsordnung"

func contractLetter(record ContractRecord, head carecontract.Letterhead) Letter {
	version := record.Version
	filled := templates.Fill(letterTemplate(head.Template), letterValues(record, head))
	blocks := letterBlocks(filled.Blocks)
	blocks = append(blocks,
		LetterBlock{Heading: "Vertragsangaben", Text: "Das gilt für diesen Vertrag:", Facts: termFacts(version, head)},
		LetterBlock{Heading: "Gebuchte Betreuung", Text: "An diesen Tagen ist Ihr Kind in der Betreuung:", Facts: ServiceFacts(version.Services)},
	)
	if concluded := record.Concluded; concluded != nil && concluded.Method == carecontract.MethodWithApplication {
		blocks = append(blocks, LetterBlock{Heading: "Vertragsschluss", Text: "Dieser Vertrag ist mit der Annahme der Anmeldung am " +
			germanDate(calendar.DateFromTime(concluded.At)) + " geschlossen. Sie müssen ihn nicht unterschreiben."})
	}
	if version.Terms.SpecialTermination {
		blocks = append(blocks, LetterBlock{Heading: "Sonderkündigung", Text: strings.TrimSpace(version.Terms.SpecialTerminationNote)})
	}
	if version.Terms.StatuteRequired {
		text := "Mit diesem Vertrag erkennen Sie die „" + strings.TrimSpace(version.Terms.StatuteTitle) + "“ an."
		if url := strings.TrimSpace(version.Terms.StatuteURL); url != "" {
			text += " Sie finden sie hier: " + url
		}
		blocks = append(blocks, LetterBlock{Heading: "Satzung", Text: text})
	}
	recipientLines := []string{}
	for _, party := range record.Parties {
		recipientLines = append(recipientLines, fullName(party.FirstName, party.LastName))
	}
	facts := []LetterFact{{Label: "Vertrag", Value: "Nr. " + strconv.FormatInt(record.Summary.ID, 10)}}
	if !version.EffectiveFrom.IsZero() {
		facts = append(facts, LetterFact{Label: "Beginn", Value: germanDate(version.EffectiveFrom)})
	}
	for _, fact := range []LetterFact{
		{Label: "Ansprechperson", Value: head.ContactName},
		{Label: "Telefon", Value: head.ContactPhone},
		{Label: "E-Mail", Value: head.ContactEmail},
	} {
		if strings.TrimSpace(fact.Value) != "" {
			facts = append(facts, fact)
		}
	}
	return Letter{
		Title:       "Betreuungsvertrag Nr. " + strconv.FormatInt(record.Summary.ID, 10),
		AccentColor: head.AccentColor, Logo: head.Logo,
		Sender:    textLines(head.SenderText),
		Recipient: nonBlank(recipientLines...),
		Facts:     facts,
		Subject:   filled.Subject, Salutation: filled.Salutation,
		Blocks:  blocks,
		Closing: filled.Closing,
		Footer:  textLines(head.FooterText),
	}
}

// letterTemplate is the Träger's template of the contract, or the standard
// one while it has none of its own.
func letterTemplate(template carecontract.LetterTemplate) templates.Template {
	if strings.TrimSpace(template.Body) == "" {
		return templates.Default(templates.KindContract)
	}
	return templates.Template{Subject: template.Subject, Salutation: template.Salutation, Body: template.Body, Closing: template.Closing}
}

func letterValues(record ContractRecord, head carecontract.Letterhead) map[string]templates.Value {
	version := record.Version
	text := func(value string) templates.Value { return templates.Value{Text: strings.TrimSpace(value)} }
	familyName, registrant := version.ChildLastName, ""
	if len(record.Parties) > 0 {
		familyName = record.Parties[0].LastName
		registrant = fullName(record.Parties[0].FirstName, record.Parties[0].LastName)
	}
	values := map[string]templates.Value{
		"familienname":   text(familyName),
		"anmelder":       text(registrant),
		"kinder":         text(fullName(version.ChildFirstName, version.ChildLastName)),
		"datum":          text(germanDate(calendar.DateFromTime(version.CreatedAt))),
		"traeger":        text(firstLine(textLines(head.SenderText))),
		"ansprechperson": text(head.ContactName),
		"rueckfragen":    text(questionText(head)),
		"angebot":        text(joinGerman(ModuleNames(version.Services))),
		"betreuungsort":  text(version.SchoolName),
		"vertragsbeginn": text(germanDate(version.EffectiveFrom)),
		"summe":          {},
	}
	switch {
	case version.PriceCents != nil:
		values["summe"] = text(formatEuro(*version.PriceCents))
	case moduleCount(version.Services) > 1:
		values["summe"] = text(feeByRules)
	default:
		values["summe"] = text(feeByStatute)
	}
	return values
}

func termFacts(version carecontract.Version, head carecontract.Letterhead) []LetterFact {
	terms := version.Terms
	partner := strings.TrimSpace(terms.ContractPartner)
	if partner == "" {
		partner = firstLine(textLines(head.SenderText))
	}
	facts := []LetterFact{
		{Label: "Kind", Value: fullName(version.ChildFirstName, version.ChildLastName)},
	}
	if version.ChildBirthDate != nil {
		facts = append(facts, LetterFact{Label: "Geboren am", Value: germanDate(*version.ChildBirthDate)})
	}
	facts = append(facts,
		LetterFact{Label: "Betreuungsort", Value: version.SchoolName},
		LetterFact{Label: "Beginn", Value: germanDate(version.EffectiveFrom)},
		LetterFact{Label: "Laufzeit", Value: TermText(terms, version.EndsOn)},
	)
	switch {
	case version.PriceCents != nil:
		facts = append(facts, LetterFact{Label: "Beitrag im Monat", Value: formatEuro(*version.PriceCents), Bold: true})
	case moduleCount(version.Services) > 1:
		facts = append(facts, LetterFact{Label: "Beitrag im Monat", Value: feeByRules})
	default:
		facts = append(facts, LetterFact{Label: "Beitrag im Monat", Value: feeByStatute})
	}
	facts = append(facts,
		LetterFact{Label: "Kündigung", Value: NoticeText(terms)},
		LetterFact{Label: "Vertragspartner", Value: partner},
	)
	kept := facts[:0]
	for _, fact := range facts {
		if strings.TrimSpace(fact.Value) != "" {
			kept = append(kept, fact)
		}
	}
	return kept
}

// ServiceFacts is one row per weekday Monday to Friday, and Saturday or
// Sunday when booked: the modules of the day with their end times, or
// "keine Betreuung".
func ServiceFacts(services []carecontract.Service) []LetterFact {
	facts := []LetterFact{}
	for i, day := range weekdayOrder {
		parts := []string{}
		for _, service := range services {
			if service.Weekday == day {
				parts = append(parts, ServiceText(service))
			}
		}
		if len(parts) == 0 && i >= 5 {
			continue
		}
		value := "keine Betreuung"
		if len(parts) > 0 {
			value = joinGerman(parts)
		}
		facts = append(facts, LetterFact{Label: weekdayNames[day], Value: value})
	}
	return facts
}

// ServiceText names one module of a day with its end time.
func ServiceText(service carecontract.Service) string {
	text := service.ModuleName
	if service.EndsAt != "" {
		text += " bis " + service.EndsAt + " Uhr"
	}
	if service.LastDay != nil {
		text += ", gebucht bis " + germanDate(*service.LastDay)
	}
	return text
}

// moduleCount counts the booked modules by offering, so two offerings with
// the same name still count as two.
func moduleCount(services []carecontract.Service) int {
	offerings := map[int64]bool{}
	for _, service := range services {
		offerings[service.CareOfferingID] = true
	}
	return len(offerings)
}

// ModuleNames lists the modules of the services once each, in the order of
// their first day.
func ModuleNames(services []carecontract.Service) []string {
	names := []string{}
	for _, service := range services {
		if !slices.Contains(names, service.ModuleName) {
			names = append(names, service.ModuleName)
		}
	}
	return names
}

// TermText names the term of a contract.
func TermText(terms carecontract.TemplateInput, endsOn *calendar.Date) string {
	var text string
	switch terms.TermKind {
	case carecontract.TermSchoolYear:
		text = "bis zum Ende des Schuljahres"
	case carecontract.TermFixedMonths:
		if terms.TermMonths != nil {
			text = fmt.Sprintf("%d Monate", *terms.TermMonths)
			if *terms.TermMonths == 1 {
				text = "1 Monat"
			}
		}
	default:
		text = "unbefristet"
	}
	if endsOn != nil {
		text += ", endet am " + germanDate(*endsOn)
	}
	return text
}

// NoticeText names the notice period and the date it takes effect at.
func NoticeText(terms carecontract.TemplateInput) string {
	to := map[string]string{
		carecontract.NoticeMonthEnd:      "zum Monatsende",
		carecontract.NoticeHalfYear:      "zum Halbjahr (31. Januar oder 31. Juli)",
		carecontract.NoticeSchoolYearEnd: "zum Ende des Schuljahres",
	}[terms.NoticeTo]
	switch terms.NoticeMonths {
	case 0:
		return "ohne Frist " + to
	case 1:
		return "mit 1 Monat Frist " + to
	default:
		return fmt.Sprintf("mit %d Monaten Frist %s", terms.NoticeMonths, to)
	}
}

func letterBlocks(blocks []templates.Block) []LetterBlock {
	result := make([]LetterBlock, len(blocks))
	for i, block := range blocks {
		var facts []LetterFact
		for _, row := range block.Rows {
			facts = append(facts, LetterFact(row))
		}
		result[i] = LetterBlock{Heading: block.Heading, Text: block.Text, Bold: block.Bold, Facts: facts}
	}
	return result
}

// documentFilenameBase names the PDF of a contract without a child's name.
func documentFilenameBase(record ContractRecord) string {
	return "betreuungsvertrag-" + strconv.FormatInt(record.Summary.ID, 10)
}

func questionText(head carecontract.Letterhead) string {
	name := strings.TrimSpace(head.ContactName)
	ways := nonBlank(head.ContactPhone, head.ContactEmail)
	switch {
	case name == "" && len(ways) == 0:
		return ""
	case name == "":
		return "Bei Fragen erreichen Sie uns unter " + strings.Join(ways, " oder ") + "."
	case len(ways) == 0:
		return "Bei Fragen hilft Ihnen " + name + "."
	}
	return "Bei Fragen erreichen Sie " + name + " unter " + strings.Join(ways, " oder ") + "."
}

func joinGerman(names []string) string {
	if len(names) <= 1 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " und " + names[len(names)-1]
}

func formatEuro(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	euros := strconv.FormatInt(cents/100, 10)
	for i := len(euros) - 3; i > 0; i -= 3 {
		euros = euros[:i] + "." + euros[i:]
	}
	return fmt.Sprintf("%s%s,%02d €", sign, euros, cents%100)
}

func germanDate(date calendar.Date) string {
	if date.IsZero() {
		return ""
	}
	return date.Format("02.01.2006")
}

func fullName(first, last string) string {
	return strings.TrimSpace(strings.TrimSpace(first) + " " + strings.TrimSpace(last))
}

func textLines(text string) []string {
	return nonBlank(strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")...)
}

func firstLine(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return lines[0]
}

func nonBlank(values ...string) []string {
	kept := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			kept = append(kept, value)
		}
	}
	return kept
}
