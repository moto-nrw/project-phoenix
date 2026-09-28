package announcement

import (
	"sort"
	"strconv"
	"strings"
	"time"

	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

// The staff proof report of an Erklärung (#3430) in the moto PDF design. The
// report is described here; the root binds the Document Rendering record
// renderer, which this inbound may not import.
//
// The shapes are aliases of unnamed structs, so the parent portal's proof
// document is the same type and one binding serves both.
type (
	// ReportField is one label/value line.
	ReportField = struct{ Label, Value string }
	// ReportBlock is an indented block inside a card.
	ReportBlock = struct {
		Title  string
		Fields []ReportField
	}
	// ReportCard is one card: a heading, its fields and its blocks.
	ReportCard = struct {
		Title  string
		Fields []ReportField
		Blocks []ReportBlock
	}
	// ReportSection is a titled group of cards.
	ReportSection = struct {
		Title string
		Cards []ReportCard
	}
	// ReportDocument is the whole document.
	ReportDocument = struct {
		Title       string
		Subtitle    string
		GeneratedAt time.Time
		Filters     []string
		Footer      string
		Sections    []ReportSection
	}
	// ReportFile is the rendered file.
	ReportFile = struct {
		Data        []byte
		ContentType string
		Filename    string
	}
)

// ReportRenderer renders a report document as a PDF.
type ReportRenderer interface {
	RenderReport(doc ReportDocument, filenameBase string) (ReportFile, error)
}

const (
	reportFooter          = "Vertraulich: enthält personenbezogene Daten von Kindern und Eltern"
	reportIntegrityOK     = "Text und Antworten sind seit der Veröffentlichung unverändert."
	reportIntegrityBroken = "Achtung: Mindestens ein gespeicherter Eintrag wurde nachträglich verändert. Bitte wenden Sie sich an den moto-Support."
)

var declarationStateLabels = map[string]string{
	announcementService.DeclarationStateAgreed:       "Zugestimmt",
	announcementService.DeclarationStateDeclined:     "Abgelehnt",
	announcementService.DeclarationStateAcknowledged: "Zur Kenntnis genommen",
	announcementService.DeclarationStateRevoked:      "Widerrufen",
	announcementService.DeclarationStatePartial:      "Teilweise beantwortet",
	announcementService.DeclarationStateOpen:         "Offen",
	announcementService.DeclarationStateExpired:      "Frist abgelaufen",
	announcementService.DeclarationStateNoSigner:     "Niemand kann antworten",
}

// declarationStateOrder sorts children: answered first, then those still
// waiting; children nobody can answer for are listed apart.
var declarationStateOrder = map[string]int{
	announcementService.DeclarationStateAgreed:       0,
	announcementService.DeclarationStateDeclined:     0,
	announcementService.DeclarationStateAcknowledged: 0,
	announcementService.DeclarationStateRevoked:      0,
	announcementService.DeclarationStatePartial:      0,
	announcementService.DeclarationStateOpen:         1,
	announcementService.DeclarationStateExpired:      1,
}

func declarationKindLabel(kind string) string {
	if kind == announcementService.DeclarationKindAcknowledgement {
		return "Nur zur Kenntnis nehmen"
	}
	return "Zustimmen oder ablehnen"
}

func declarationSignersLabel(signers string) string {
	if signers == announcementService.DeclarationSignersAll {
		return "Alle sorgeberechtigten Personen"
	}
	return "Eine sorgeberechtigte Person genügt"
}

func reportStamp(t time.Time) string {
	return t.In(berlin).Format("02.01.2006, 15:04") + " Uhr"
}

func reportDeadline(deadline *time.Time) string {
	if deadline == nil {
		return "Ohne Frist"
	}
	return "Bis " + deadline.In(berlin).Format("02.01.2006")
}

// declarationReport describes the staff report: overview, versions, the
// state of every child and the full history.
func declarationReport(s *announcementService.ParentDeclarationStatus) ReportDocument {
	filters := []string{declarationKindLabel(s.Settings.Kind), declarationSignersLabel(s.Settings.Signers), reportDeadline(s.Deadline)}
	if s.CurrentVersion != nil {
		filters = append(filters, "Fassung "+strconv.Itoa(s.CurrentVersion.VersionNo))
	}
	return ReportDocument{
		Title: "Nachweisbericht", Subtitle: s.Title, GeneratedAt: s.GeneratedAt, Filters: filters, Footer: reportFooter,
		Sections: []ReportSection{
			{Title: "Überblick", Cards: []ReportCard{declarationSettingsCard(s), declarationSummaryCard(s)}},
			{Title: "Fassungen", Cards: declarationVersionCards(s)},
			{Title: "Stand je Kind", Cards: declarationChildCards(s.Children)},
			{Title: "Verlauf", Cards: declarationHistoryCards(s.Submissions)},
		},
	}
}

func declarationSettingsCard(s *announcementService.ParentDeclarationStatus) ReportCard {
	revocable, password := "Nicht erlaubt", "Nein"
	if s.Settings.Revocable {
		revocable = "Erlaubt, auch nach der Frist"
	}
	if s.Settings.RequiresPassword {
		password = "Ja"
	}
	return ReportCard{Title: "Einstellungen", Fields: []ReportField{
		{Label: "Art", Value: declarationKindLabel(s.Settings.Kind)},
		{Label: "Wer muss antworten", Value: declarationSignersLabel(s.Settings.Signers)},
		{Label: "Widerruf", Value: revocable},
		{Label: "Passwort vor dem Antworten", Value: password},
		{Label: "Frist", Value: reportDeadline(s.Deadline)},
		{Label: "Verfahren", Value: announcementService.DeclarationMethodLabel("simple_electronic")},
	}}
}

func declarationSummaryCard(s *announcementService.ParentDeclarationStatus) ReportCard {
	fields := []ReportField{{Label: "Kinder erreicht", Value: strconv.Itoa(s.Summary.ChildrenTotal)}}
	for _, state := range declarationSummaryStates {
		if count := s.Summary.ByState[state]; count > 0 {
			fields = append(fields, ReportField{Label: declarationStateLabels[state], Value: strconv.Itoa(count)})
		}
	}
	integrity := reportIntegrityOK
	if !s.IntegrityAllGood {
		integrity = reportIntegrityBroken
	}
	fields = append(fields, ReportField{Label: "Prüfung", Value: integrity})
	return ReportCard{Title: "Stand", Fields: fields}
}

func declarationVersionCards(s *announcementService.ParentDeclarationStatus) []ReportCard {
	cards := make([]ReportCard, 0, len(s.Versions))
	for _, v := range s.Versions {
		title := "Fassung " + strconv.Itoa(v.VersionNo)
		if s.CurrentVersion != nil && s.CurrentVersion.ID == v.ID {
			title += " (aktuell)"
		}
		files := make([]string, 0, len(v.Attachments))
		for _, a := range v.Attachments {
			files = append(files, a.Filename)
		}
		attachments := "Keine"
		if len(files) > 0 {
			attachments = strings.Join(files, ", ")
		}
		cards = append(cards, ReportCard{Title: title, Fields: []ReportField{
			{Label: "Veröffentlicht", Value: reportStamp(v.PublishedAt)},
			{Label: "Titel", Value: v.Title},
			{Label: "Text", Value: v.Body},
			{Label: "Dateien", Value: attachments},
		}})
	}
	return cards
}

// declarationChildCards lists every child someone can answer for, answered
// first and by name, and the children nobody can answer for in one card.
func declarationChildCards(children []announcementService.DeclarationChildStatus) []ReportCard {
	sorted := append([]announcementService.DeclarationChildStatus(nil), children...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if oa, ob := childStateRank(a.State), childStateRank(b.State); oa != ob {
			return oa < ob
		}
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		return a.FirstName < b.FirstName
	})
	cards := []ReportCard{}
	nobody := []string{}
	for _, c := range sorted {
		name := strings.TrimSpace(c.FirstName + " " + c.LastName)
		if c.State == announcementService.DeclarationStateNoSigner {
			nobody = append(nobody, name)
			continue
		}
		cards = append(cards, declarationChildCard(c, name))
	}
	if len(nobody) > 0 {
		cards = append(cards, ReportCard{Title: "Niemand kann antworten", Fields: []ReportField{
			{Value: "Keine sorgeberechtigte Person dieser Kinder hat ein Eltern-Konto."},
			{Label: "Kinder", Value: strings.Join(nobody, ", ")},
		}})
	}
	return cards
}

func childStateRank(state string) int {
	if rank, ok := declarationStateOrder[state]; ok {
		return rank
	}
	return 2
}

func declarationChildCard(c announcementService.DeclarationChildStatus, name string) ReportCard {
	title := name
	if class := strings.TrimSpace(c.SchoolClass); class != "" {
		if !strings.HasPrefix(class, "Klasse") {
			class = "Klasse " + class
		}
		title += " · " + class
	}
	blocks := make([]ReportBlock, 0, len(c.Signers))
	for _, signer := range c.Signers {
		answer := "Noch keine Antwort"
		if signer.Action != nil && signer.SubmittedAt != nil {
			answer = announcementService.DeclarationActionLabel(*signer.Action) + " am " + reportStamp(*signer.SubmittedAt)
		}
		blocks = append(blocks, ReportBlock{
			Title:  strings.TrimSpace(signer.FirstName + " " + signer.LastName),
			Fields: []ReportField{{Label: "Antwort", Value: answer}},
		})
	}
	return ReportCard{
		Title:  title,
		Fields: []ReportField{{Label: "Stand", Value: declarationStateLabels[c.State]}},
		Blocks: blocks,
	}
}

// declarationHistoryCards lists every stored answer, earlier versions
// included, in the order the status returns them (newest first).
func declarationHistoryCards(submissions []announcementService.DeclarationSubmissionRecord) []ReportCard {
	if len(submissions) == 0 {
		return []ReportCard{{Title: "Noch keine Antworten"}}
	}
	cards := make([]ReportCard, 0, len(submissions))
	for _, sub := range submissions {
		password := "Nein"
		if sub.PasswordConfirmed {
			password = "Ja"
		}
		fields := []ReportField{
			{Label: "Kind", Value: strings.TrimSpace(sub.StudentFirstName + " " + sub.StudentLastName)},
			{Label: "Von", Value: sub.SignerName + " (" + announcementService.GuardianRoleLabel(sub.GuardianRole) + ")"},
			{Label: "Fassung", Value: strconv.Itoa(sub.VersionNo)},
			{Label: "Passwort bestätigt", Value: password},
		}
		if !sub.IntegrityOK {
			fields = append(fields, ReportField{Label: "Prüfung", Value: "Dieser Eintrag wurde nachträglich verändert."})
		}
		cards = append(cards, ReportCard{
			Title:  announcementService.DeclarationActionLabel(sub.Action) + " am " + reportStamp(sub.SubmittedAt),
			Fields: fields,
		})
	}
	return cards
}
