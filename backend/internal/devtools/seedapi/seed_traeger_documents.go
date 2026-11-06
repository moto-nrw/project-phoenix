package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// seedTraegerDocumentsStep gives the demo Träger documents it shares (#4692)
// through the Träger routes, the way the Verwaltung does it: a house rule
// for every parent and every school, a holiday offer for the parents of one
// school, a letter for the parents of one group, a list for one school and
// a draft nobody sees yet. The school then opens one document, so the read
// trail holds a row and the school's "Dateien" shows the card "Vom Träger".
type seedTraegerDocumentsStep struct{}

func (seedTraegerDocumentsStep) Name() string { return "Seeding the Träger documents" }

// seedTraegerDocument is one demo document with the releases it gets. Every
// text is invented.
type seedTraegerDocument struct {
	title    string
	fileName string
	lines    []string
	releases []seedTraegerRelease
}

// seedTraegerRelease names an audience; school and group pick the first
// school and its first group of the demo Träger.
type seedTraegerRelease struct {
	audience string
	school   bool
	group    bool
}

var seedTraegerDocuments = []seedTraegerDocument{
	{
		title: "Hausordnung Ganztag 2026/27", fileName: "hausordnung-ganztag-2026-27.pdf",
		lines:    []string{"Hausordnung Ganztag 2026/27", "Ganztag im Quartier e.V.", "Abholung bis 16 Uhr, Krankmeldung bis 8 Uhr."},
		releases: []seedTraegerRelease{{audience: "parents"}, {audience: "schools"}},
	},
	{
		title: "Herbstferien: Angebote und Ausflüge", fileName: "herbstferien-angebote.pdf",
		lines:    []string{"Herbstferien 2026", "Ausflüge, Werkstatt und Waldtage."},
		releases: []seedTraegerRelease{{audience: "parents", school: true}},
	},
	{
		title: "Elternbrief zum Gruppenausflug", fileName: "elternbrief-gruppenausflug.pdf",
		lines:    []string{"Elternbrief", "Am Freitag fährt die Gruppe in den Zoo."},
		releases: []seedTraegerRelease{{audience: "parents", school: true, group: true}},
	},
	{
		title: "Ansprechpersonen im Träger-Büro", fileName: "ansprechpersonen-traeger-buero.pdf",
		lines:    []string{"Ansprechpersonen", "Verwaltung: Petra Lindner, +49 5402 1234-0"},
		releases: []seedTraegerRelease{{audience: "schools", school: true}},
	},
	{
		title: "Entwurf Jahresplanung 2027", fileName: "entwurf-jahresplanung-2027.pdf",
		lines: []string{"Entwurf Jahresplanung 2027"},
	},
}

func (seedTraegerDocumentsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger document seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	school, group, err := seedTraegerDocumentAudience(rt.Client, auth)
	if err != nil {
		return err
	}
	opened, err := seedTraegerDocumentsOnce(rt.Client, auth, school, group)
	if err != nil {
		return err
	}
	// The OGS-Leitung of the demo school opens the house rule once.
	if _, err := rt.Client.Get(fmt.Sprintf("/api/traeger-documents/%d/file", opened)); err != nil {
		return fmt.Errorf("open a Träger document at the school: %w", err)
	}
	fmt.Printf("  Träger documents: %d documents, released for parents and schools\n", len(seedTraegerDocuments))
	return nil
}

// seedTraegerDocumentsOnce uploads the documents the Träger does not have
// yet and releases them; it returns the first released document.
func seedTraegerDocumentsOnce(client *Client, auth AuthRef, school, group int64) (int64, error) {
	existing, err := seedTraegerDocumentTitles(client, auth)
	if err != nil {
		return 0, err
	}
	opened := int64(0)
	for _, document := range seedTraegerDocuments {
		id, seen := existing[document.title]
		if !seen {
			if id, err = seedTraegerDocumentUpload(client, auth, document); err != nil {
				return 0, err
			}
		}
		for _, release := range document.releases {
			if err := seedTraegerDocumentRelease(client, auth, id, release, school, group); err != nil {
				return 0, err
			}
		}
		if opened == 0 && len(document.releases) > 0 {
			opened = id
		}
	}
	return opened, nil
}

// seedTraegerDocumentAudience picks the demo school, the first school of the
// Träger, and its first group; group is 0 without one.
func seedTraegerDocumentAudience(client *Client, auth AuthRef) (school, group int64, err error) {
	raw, err := client.GetWithAuth(auth, "/traeger/documents/audiences")
	if err != nil {
		return 0, 0, fmt.Errorf("read the audiences of the Träger documents: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID     int64 `json:"id"`
			Groups []struct {
				ID int64 `json:"id"`
			} `json:"groups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Data) == 0 {
		return 0, 0, fmt.Errorf("read the schools of the Träger documents: %w", errors.Join(err, errNoTraegerSchool))
	}
	first := envelope.Data[0]
	for _, candidate := range envelope.Data {
		if len(candidate.Groups) > 0 {
			first = candidate
			break
		}
	}
	if len(first.Groups) > 0 {
		group = first.Groups[0].ID
	}
	return first.ID, group, nil
}

var errNoTraegerSchool = errors.New("the demo Träger has no school")

// seedTraegerDocumentTitles reads the documents the Träger already has.
// Several demo schools seed the same Träger at once; a title is uploaded once.
func seedTraegerDocumentTitles(client *Client, auth AuthRef) (map[string]int64, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/documents/")
	if err != nil {
		return nil, fmt.Errorf("read the Träger documents: %w", err)
	}
	var envelope struct {
		Data []struct {
			ID    int64  `json:"id"`
			Title string `json:"title"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("read the Träger documents: %w", err)
	}
	titles := make(map[string]int64, len(envelope.Data))
	for _, document := range envelope.Data {
		titles[document.Title] = document.ID
	}
	return titles, nil
}

func seedTraegerDocumentUpload(client *Client, auth AuthRef, document seedTraegerDocument) (int64, error) {
	raw, err := client.PostFileWithAuthFields(auth, "/traeger/documents/", "file", document.fileName,
		seedTraegerDocumentPDF(document.lines), map[string]string{"title": document.title})
	if err != nil {
		return 0, fmt.Errorf("upload the Träger document %q: %w", document.title, err)
	}
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data.ID == 0 {
		return 0, fmt.Errorf("read the uploaded Träger document %q: %w", document.title, errors.Join(err, errors.New("no document id")))
	}
	return envelope.Data.ID, nil
}

// seedTraegerDocumentReleaseActive is the code of a release that another
// demo school of the same Träger already made.
const seedTraegerDocumentReleaseActive = "traeger.document_already_released"

func seedTraegerDocumentRelease(client *Client, auth AuthRef, documentID int64, release seedTraegerRelease, school, group int64) error {
	body := map[string]any{"audience": release.audience}
	if release.school {
		body["school_id"] = school
	}
	if release.group {
		if group == 0 {
			return nil
		}
		body["group_id"] = group
	}
	_, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/documents/%d/releases", documentID), body)
	var refused *APIError
	if errors.As(err, &refused) && refused.Code == seedTraegerDocumentReleaseActive {
		return nil
	}
	if err != nil {
		return fmt.Errorf("release the Träger document %d: %w", documentID, err)
	}
	return nil
}

// seedTraegerDocumentPDF writes a small one-page PDF with the given lines,
// so a downloaded demo document opens in every viewer.
func seedTraegerDocumentPDF(lines []string) []byte {
	var text strings.Builder
	text.WriteString("BT /F1 16 Tf 72 760 Td 20 TL\n")
	for _, line := range lines {
		text.WriteString("(" + pdfEscape(line) + ") Tj T*\n")
	}
	text.WriteString("ET")
	stream := text.String()
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	var out strings.Builder
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(out.String())
}

// pdfEscape escapes a line for a PDF string and spells the German letters
// in WinAnsi, the encoding of the font.
func pdfEscape(line string) string {
	var out strings.Builder
	for _, r := range line {
		switch {
		case r == '(' || r == ')' || r == '\\':
			out.WriteString(`\` + string(r))
		case r < 128:
			out.WriteRune(r)
		case r < 256:
			fmt.Fprintf(&out, `\%03o`, r)
		default:
			out.WriteRune('?')
		}
	}
	return out.String()
}
