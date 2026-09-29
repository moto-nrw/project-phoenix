package parent

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The guardian's proof of an Erklärung (#3430) as a PDF in the moto design.
// The document is described here; the root binds the Document Rendering
// record renderer, which this inbound may not import. The shapes are aliases
// of unnamed structs, identical to the staff report's, so one binding serves
// both. The PDF is German like the school's own record; the portal page
// stays in the guardian's language.
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

var errProofRendererUnbound = errors.New("declaration proof renderer is not bound")

var proofActionLabels = map[string]string{
	usersModels.DeclarationActionAgreed:   "Zugestimmt",
	usersModels.DeclarationActionDeclined: "Abgelehnt",
	usersModels.DeclarationActionRevoked:  "Zustimmung widerrufen",
}

var proofRoleLabels = map[string]string{
	"primary_guardian": "Hauptberechtigt",
	"legal_guardian":   "Erziehungsberechtigt",
	"co_guardian":      "Mitberechtigt",
	"custom":           "Individuell",
}

func labelOr(labels map[string]string, key string) string {
	if label, ok := labels[key]; ok {
		return label
	}
	return key
}

func proofStamp(t time.Time) string {
	return t.In(berlinLocation()).Format("02.01.2006, 15:04") + " Uhr"
}

func berlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.FixedZone("CET", 60*60)
	}
	return location
}

// declarationProofReport describes the guardian's proof: what they answered
// on, their own answers, and the full text of every version they answered.
func declarationProofReport(proof *parentService.DeclarationProof, now time.Time) ReportDocument {
	integrity := "Text und Antworten sind seit der Veröffentlichung unverändert."
	if !proof.IntegrityOK {
		integrity = "Achtung: Ein gespeicherter Eintrag wurde nachträglich verändert. Bitte wenden Sie sich an die OGS."
	}
	overview := ReportCard{Title: "Einverständnis", Fields: []ReportField{
		{Label: "Kind", Value: proof.ChildName},
		{Label: "Schule", Value: proof.SchoolName},
		{Label: "Titel", Value: proof.Title},
		{Label: "Verfahren", Value: methodLabel},
		{Label: "Prüfung", Value: integrity},
	}}
	return ReportDocument{
		Title: "Nachweis Ihrer Antwort", Subtitle: proof.Title, GeneratedAt: now,
		Filters: []string{proof.ChildName, proof.SchoolName},
		Footer:  "Vertraulich: enthält personenbezogene Daten",
		Sections: []ReportSection{
			{Title: "Überblick", Cards: []ReportCard{overview}},
			{Title: "Ihr Verlauf", Cards: proofHistoryCards(proof)},
			{Title: "Fassungen", Cards: proofVersionCards(proof)},
		},
	}
}

func proofHistoryCards(proof *parentService.DeclarationProof) []ReportCard {
	cards := make([]ReportCard, 0, len(proof.Submissions))
	for _, sub := range proof.Submissions {
		version := "?"
		if v, ok := proof.Versions[sub.VersionID]; ok {
			version = strconv.Itoa(v.VersionNo)
		}
		confirmed := "Nein"
		if sub.PasswordConfirmed {
			confirmed = "Ja"
		}
		cards = append(cards, ReportCard{
			Title: labelOr(proofActionLabels, sub.Action) + " am " + proofStamp(sub.SubmittedAt),
			Fields: []ReportField{
				{Label: "Von", Value: sub.SignerName + " (" + labelOr(proofRoleLabels, sub.GuardianRole) + ")"},
				{Label: "Fassung", Value: version},
				{Label: "Passwort bestätigt", Value: confirmed},
			},
		})
	}
	return cards
}

// proofVersionCards prints the versions the guardian answered on, newest
// first, with their full text.
func proofVersionCards(proof *parentService.DeclarationProof) []ReportCard {
	declared := map[int64]bool{}
	for _, sub := range proof.Submissions {
		declared[sub.VersionID] = true
	}
	versions := make([]*usersModels.DeclarationVersion, 0, len(declared))
	for id := range declared {
		if v, ok := proof.Versions[id]; ok {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i].VersionNo > versions[j].VersionNo })
	cards := make([]ReportCard, 0, len(versions))
	for _, v := range versions {
		files := make([]string, 0, len(v.Attachments))
		for _, a := range v.Attachments {
			files = append(files, a.Filename+" ("+strconv.FormatInt(a.SizeBytes, 10)+" Bytes)")
		}
		attachments := "Keine"
		if len(files) > 0 {
			attachments = strings.Join(files, ", ")
		}
		cards = append(cards, ReportCard{Title: "Fassung " + strconv.Itoa(v.VersionNo), Fields: []ReportField{
			{Label: "Veröffentlicht", Value: proofStamp(v.PublishedAt)},
			{Label: "Titel", Value: v.Title},
			{Label: "Text", Value: v.Body},
			{Label: "Dateien", Value: attachments},
		}})
	}
	return cards
}

func (rs *Resource) renderDeclarationProof(proof *parentService.DeclarationProof, now time.Time) (ReportFile, error) {
	if rs.Reports == nil {
		return ReportFile{}, errProofRendererUnbound
	}
	return rs.Reports.RenderReport(declarationProofReport(proof, now), "nachweis-erklaerung")
}
