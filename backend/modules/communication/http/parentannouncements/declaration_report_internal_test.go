package announcement

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

func declarationStatusFixture() *announcementService.ParentDeclarationStatus {
	agreed := announcementService.DeclarationActionAgreed
	at := time.Date(2026, 9, 27, 19, 57, 0, 0, time.UTC)
	version := announcementService.DeclarationVersion{
		ID: 7, VersionNo: 1, Title: "Zoo", Body: "Wir fahren in den Zoo.", ContentHash: strings.Repeat("a", 64),
		PublishedAt: at, IntegrityOK: true,
		Attachments: []announcementService.DeclarationAttachment{{Filename: "Zoo.pdf", SHA256: strings.Repeat("b", 64)}},
	}
	return &announcementService.ParentDeclarationStatus{
		Title:          "Einverständnis Zoo",
		Settings:       announcementService.ParentDeclarationSettings{Kind: announcementService.DeclarationKindConsent, Signers: announcementService.DeclarationSignersAny, Revocable: true},
		CurrentVersion: &version, Versions: []announcementService.DeclarationVersion{version},
		Summary: announcementService.DeclarationSummary{ChildrenTotal: 3, ByState: map[string]int{
			announcementService.DeclarationStateAgreed: 1, announcementService.DeclarationStateOpen: 1, announcementService.DeclarationStateNoSigner: 1,
		}},
		Children: []announcementService.DeclarationChildStatus{
			{FirstName: "Noah", LastName: "Adler", SchoolClass: "1b", State: announcementService.DeclarationStateOpen,
				Signers: []announcementService.DeclarationSignerStatus{{FirstName: "Thomas", LastName: "Adler"}}},
			{FirstName: "Mia", LastName: "Bauer", State: announcementService.DeclarationStateNoSigner},
			{FirstName: "Lina", LastName: "Richter", SchoolClass: "Klasse 1a", State: announcementService.DeclarationStateAgreed,
				Signers: []announcementService.DeclarationSignerStatus{{FirstName: "Klaus", LastName: "Richter", Action: &agreed, SubmittedAt: &at}}},
		},
		Submissions: []announcementService.DeclarationSubmissionRecord{{
			StudentFirstName: "Lina", StudentLastName: "Richter", SignerName: "Klaus Richter", GuardianRole: "primary_guardian",
			Action: agreed, VersionNo: 1, ContentHash: strings.Repeat("a", 64), RecordHash: strings.Repeat("c", 64),
			SubmittedAt: at, IntegrityOK: true,
		}},
		GeneratedAt: at, IntegrityAllGood: true,
	}
}

func reportTexts(doc ReportDocument) string {
	parts := []string{doc.Title, doc.Subtitle, strings.Join(doc.Filters, "|")}
	for _, section := range doc.Sections {
		parts = append(parts, "#"+section.Title)
		for _, card := range section.Cards {
			parts = append(parts, "["+card.Title+"]")
			for _, f := range card.Fields {
				parts = append(parts, f.Label+"="+f.Value)
			}
			for _, b := range card.Blocks {
				parts = append(parts, "("+b.Title+")")
				for _, f := range b.Fields {
					parts = append(parts, f.Label+"="+f.Value)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

func TestDeclarationReportDescribesTheProofWithoutChecksums(t *testing.T) {
	t.Parallel()
	text := reportTexts(declarationReport(declarationStatusFixture()))

	assert.Contains(t, text, "Wer muss antworten=Eine sorgeberechtigte Person genügt")
	assert.Contains(t, text, "Prüfung="+reportIntegrityOK)
	assert.Contains(t, text, "[Fassung 1 (aktuell)]")
	assert.Contains(t, text, "Text=Wir fahren in den Zoo.")
	assert.Contains(t, text, "Dateien=Zoo.pdf")
	assert.Contains(t, text, "(Klaus Richter)\nAntwort=Zugestimmt am 27.09.2026, 21:57 Uhr")
	assert.Contains(t, text, "(Thomas Adler)\nAntwort=Noch keine Antwort")
	assert.Contains(t, text, "Von=Klaus Richter (Hauptsorgeberechtigte Person)")
	// Checksums are for the CSV and the automatic check, not for readers.
	assert.NotContains(t, text, strings.Repeat("a", 64))
	assert.NotContains(t, text, strings.Repeat("b", 64))
	assert.NotContains(t, text, strings.Repeat("c", 64))
	assert.NotContains(t, text, "Prüfsumme")
}

func TestDeclarationReportOrdersChildrenAndGroupsNobody(t *testing.T) {
	t.Parallel()
	doc := declarationReport(declarationStatusFixture())
	require.Len(t, doc.Sections, 4)
	children := doc.Sections[2].Cards
	require.Len(t, children, 3)
	assert.Equal(t, "Lina Richter · Klasse 1a", children[0].Title, "answered first, class not doubled")
	assert.Equal(t, "Noah Adler · Klasse 1b", children[1].Title)
	assert.Equal(t, "Niemand kann antworten", children[2].Title)
	assert.Contains(t, reportTexts(doc), "Kinder=Mia Bauer")
}

func TestDeclarationReportWarnsWhenAnEntryChanged(t *testing.T) {
	t.Parallel()
	status := declarationStatusFixture()
	status.IntegrityAllGood = false
	status.Submissions[0].IntegrityOK = false
	text := reportTexts(declarationReport(status))

	assert.Contains(t, text, "Prüfung="+reportIntegrityBroken)
	assert.Contains(t, text, "Prüfung=Dieser Eintrag wurde nachträglich verändert.")
}

func TestDeclarationReportWithoutAnswers(t *testing.T) {
	t.Parallel()
	status := declarationStatusFixture()
	status.Submissions = nil
	doc := declarationReport(status)
	assert.Equal(t, []ReportCard{{Title: "Noch keine Antworten"}}, doc.Sections[3].Cards)
}
