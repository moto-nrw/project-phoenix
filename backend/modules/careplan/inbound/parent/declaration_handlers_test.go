package parent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// The portal names who settled a child under "any" together with the date,
// so the other signer's submission time must reach the wire.
func TestDeclarationResponseCarriesOtherSignerSubmittedAt(t *testing.T) {
	agreed := usersModels.DeclarationActionAgreed
	at := time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)
	out := toDeclarationResponse(&usersModels.AnnouncementFeedDeclaration{
		Children: []*usersModels.AnnouncementFeedDeclarationChild{{
			StudentID: 7,
			OtherSigners: []usersModels.DeclarationSignerState{
				{FirstName: "Sabine", LastName: "Muster", Action: &agreed, SubmittedAt: &at},
				{FirstName: "Tom", LastName: "Muster"},
			},
		}},
	})

	raw, err := json.Marshal(out.Children[0].OtherSigners)
	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"first_name":"Sabine","last_name":"Muster","action":"agreed","submitted_at":"2026-09-20T08:30:00Z"},
		{"first_name":"Tom","last_name":"Muster","action":null,"submitted_at":null}
	]`, string(raw))
}

func proofTexts(doc ReportDocument) string {
	parts := []string{doc.Title, doc.Subtitle, strings.Join(doc.Filters, "|")}
	for _, section := range doc.Sections {
		parts = append(parts, "#"+section.Title)
		for _, card := range section.Cards {
			parts = append(parts, "["+card.Title+"]")
			for _, f := range card.Fields {
				parts = append(parts, f.Label+"="+f.Value)
			}
		}
	}
	return strings.Join(parts, "\n")
}

func declarationProofFixture(intact bool) *parentService.DeclarationProof {
	at := time.Date(2026, 9, 27, 19, 57, 0, 0, time.UTC)
	older := &usersModels.DeclarationVersion{ID: 1, VersionNo: 1, Title: "Zoo", Body: "Alter Text", ContentHash: strings.Repeat("a", 64), PublishedAt: at}
	newer := &usersModels.DeclarationVersion{ID: 2, VersionNo: 2, Title: "Zoo", Body: "Neuer Text", ContentHash: strings.Repeat("b", 64), PublishedAt: at}
	unrelated := &usersModels.DeclarationVersion{ID: 3, VersionNo: 3, Title: "Zoo", Body: "Nie beantwortet", PublishedAt: at}
	return &parentService.DeclarationProof{
		Title: "Einverständnis Zoo", SchoolName: "OGS Musterstadt", ChildName: "Lina Richter",
		Kind:     usersModels.DeclarationKindConsent,
		Versions: map[int64]*usersModels.DeclarationVersion{1: older, 2: newer, 3: unrelated},
		Submissions: []*usersModels.DeclarationSubmission{
			{VersionID: 2, Action: usersModels.DeclarationActionAgreed, SignerName: "Klaus Richter", GuardianRole: "primary_guardian", SubmittedAt: at, RecordHash: strings.Repeat("c", 64)},
			{VersionID: 1, Action: usersModels.DeclarationActionDeclined, SignerName: "Klaus Richter", GuardianRole: "primary_guardian", PasswordConfirmed: true, SubmittedAt: at},
		},
		IntegrityOK: intact,
	}
}

func TestDeclarationProofReportShowsOwnAnswersAndTextsWithoutChecksums(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)
	text := proofTexts(declarationProofReport(declarationProofFixture(true), now))

	assert.Contains(t, text, "Kind=Lina Richter")
	assert.Contains(t, text, "Prüfung=Text und Antworten sind seit der Veröffentlichung unverändert.")
	assert.Contains(t, text, "[Zugestimmt am 27.09.2026, 21:57 Uhr]\nVon=Klaus Richter (Hauptberechtigt)\nFassung=2\nPasswort bestätigt=Nein")
	assert.Contains(t, text, "Passwort bestätigt=Ja")
	assert.Less(t, strings.Index(text, "[Fassung 2]"), strings.Index(text, "[Fassung 1]"), "newest version first")
	assert.NotContains(t, text, "Nie beantwortet", "only versions the guardian answered on")
	assert.NotContains(t, text, strings.Repeat("a", 64))
	assert.NotContains(t, text, strings.Repeat("c", 64))
}

func TestDeclarationProofReportWarnsWhenAnEntryChanged(t *testing.T) {
	text := proofTexts(declarationProofReport(declarationProofFixture(false), time.Date(2026, 9, 28, 8, 0, 0, 0, time.UTC)))
	assert.Contains(t, text, "Prüfung=Achtung: Ein gespeicherter Eintrag wurde nachträglich verändert.")
}

type capturingProofRenderer struct{ doc *ReportDocument }

func (c *capturingProofRenderer) RenderReport(doc ReportDocument, filenameBase string) (ReportFile, error) {
	c.doc = &doc
	return ReportFile{Data: []byte("%PDF-1.7"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

func declarationProofRequest(query string) *http.Request {
	req := withClaims(httptest.NewRequest(http.MethodGet, "/me/news/42/declaration/proof?"+query, nil), 1234)
	route := chi.NewRouteContext()
	route.URLParams.Add("announcementId", "42")
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
}

func TestDeclarationProofDownloadsTheMotoPDF(t *testing.T) {
	t.Parallel()
	renderer := &capturingProofRenderer{}
	rs := &Resource{ParentService: &fakeParentService{declarationProof: declarationProofFixture(true)}, Reports: renderer}
	w := httptest.NewRecorder()

	rs.declarationProof(w, declarationProofRequest("student_id=7&format=pdf"))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="nachweis-erklaerung.pdf"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "%PDF-1.7", w.Body.String())
	require.NotNil(t, renderer.doc)
	assert.Equal(t, "Nachweis Ihrer Antwort", renderer.doc.Title)
}

func TestDeclarationProofWithoutFormatStaysJSON(t *testing.T) {
	t.Parallel()
	rs := &Resource{ParentService: &fakeParentService{declarationProof: declarationProofFixture(true)}}
	w := httptest.NewRecorder()

	rs.declarationProof(w, declarationProofRequest("student_id=7"))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"child_name":"Lina Richter"`)
	assert.Contains(t, w.Body.String(), `"integrity_ok":true`)
}
