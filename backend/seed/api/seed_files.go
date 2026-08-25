package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/filestore"
)

// seedFileStorageStep fills the school file storage (#2596): one folder per
// visibility, each with files a school would really keep there. Without it
// the Dateien screen is empty on every dev machine and the share model — the
// part of the feature that is hard to reason about from the code — cannot be
// looked at at all.
type seedFileStorageStep struct{}

func (seedFileStorageStep) Name() string { return "Seeding file storage" }

// demoFile is one file to upload into the folder being seeded.
type demoFile struct {
	Filename string
	Contents []byte
}

// demoFolder is one folder of the demo storage.
type demoFolder struct {
	Name       string
	Visibility string
	// Shared marks the folder whose audience is picked from the seeded roles
	// and staff accounts.
	Shared bool
	Files  []demoFile
}

func (s seedFileStorageStep) Run(_ context.Context, rt *Runtime) error {
	// Team uploads on, so the demo also shows the staff side of the storage:
	// an upload area for folders they can see and their own files deletable.
	if _, err := rt.Client.Put("/api/settings/values/"+configModels.KeyFilesStaffUploadEnabled, map[string]any{"value": true}); err != nil {
		return fmt.Errorf("enable team uploads: %w", err)
	}

	roleIDs, accountIDs, err := s.audience(rt)
	if err != nil {
		return err
	}

	folders := s.folders()
	files := 0
	for _, folder := range folders {
		body := map[string]any{
			"name":       folder.Name,
			"visibility": folder.Visibility,
		}
		if folder.Shared {
			body["role_ids"] = roleIDs
			body["account_ids"] = accountIDs
		}
		raw, err := rt.Client.Post("/api/files/folders", body)
		if err != nil {
			return fmt.Errorf("create folder %q: %w", folder.Name, err)
		}
		folderID, err := folderIDFromResponse(raw)
		if err != nil {
			return fmt.Errorf("create folder %q: %w", folder.Name, err)
		}

		for _, file := range folder.Files {
			path := fmt.Sprintf("/api/files/folders/%s/files", folderID)
			if _, err := rt.Client.PostFile(path, "file", file.Filename, file.Contents); err != nil {
				return fmt.Errorf("upload %q into %q: %w", file.Filename, folder.Name, err)
			}
			files++
		}
	}

	fmt.Printf("  %d folders with %d files created\n", len(folders), files)
	return nil
}

// folders is the demo storage: every visibility once, so all three branches of
// the share model are visible on a seeded machine.
func (seedFileStorageStep) folders() []demoFolder {
	return []demoFolder{
		{
			Name:       "Vorlagen und Formulare",
			Visibility: filestore.VisibilityAllStaff,
			Files: []demoFile{
				{
					Filename: "Elternbrief Vorlage.docx",
					Contents: demoDOCX("Elternbrief", []string{
						"Liebe Eltern,",
						"am Freitag machen wir einen Ausflug in den Stadtpark.",
						"Bitte geben Sie Ihrem Kind feste Schuhe und eine Regenjacke mit.",
						"Wir sind um 16:00 Uhr zurück in der Schule.",
					}),
				},
				{
					Filename: "Notfallblatt.pdf",
					Contents: demoPDF("Notfallblatt", []string{
						"Erste Hilfe: Raum 12, neben dem Sekretariat.",
						"Notruf 112. Schulleitung informieren.",
						"Notfallkontakte der Kinder stehen in der App.",
					}),
				},
			},
		},
		{
			Name:       "Unterlagen der Leitung",
			Visibility: filestore.VisibilityAdmins,
			Files: []demoFile{
				{
					Filename: "Dienstanweisung Aufsicht.pdf",
					Contents: demoPDF("Dienstanweisung Aufsicht", []string{
						"Die Aufsicht beginnt 10 Minuten vor der Betreuung.",
						"Auf dem Schulhof sind immer zwei Personen im Dienst.",
						"Abweichungen bitte im Dienstplan eintragen.",
					}),
				},
			},
		},
		{
			Name:       "AG-Planung",
			Visibility: filestore.VisibilitySelected,
			Shared:     true,
			Files: []demoFile{
				{
					Filename: "AG-Planung Herbst.docx",
					Contents: demoDOCX("AG-Planung Herbst", []string{
						"Montag: Fußball in der Turnhalle.",
						"Dienstag: Kochen in der Küche.",
						"Donnerstag: Theater im Musikraum.",
					}),
				},
			},
		},
	}
}

// audience picks the roles and persons the shared folder is released to. A
// school without either would leave the selected-visibility folder without an
// audience, which the API rejects — so this fails loudly rather than seeding a
// storage whose share model is untested.
func (seedFileStorageStep) audience(rt *Runtime) ([]string, []string, error) {
	raw, err := rt.Client.Get("/api/files/audience")
	if err != nil {
		return nil, nil, fmt.Errorf("read file storage audience: %w", err)
	}
	var payload struct {
		Data struct {
			Roles []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"roles"`
			Accounts []struct {
				AccountID string `json:"account_id"`
			} `json:"accounts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, fmt.Errorf("decode file storage audience: %w", err)
	}
	if len(payload.Data.Roles) == 0 && len(payload.Data.Accounts) == 0 {
		return nil, nil, fmt.Errorf("file storage audience is empty: no role and no person to share a folder with")
	}

	roleIDs := make([]string, 0, 1)
	if role, ok := pickAudienceRole(payload.Data.Roles); ok {
		roleIDs = append(roleIDs, role)
	}
	accountIDs := make([]string, 0, 2)
	for _, account := range payload.Data.Accounts {
		if len(accountIDs) == cap(accountIDs) {
			break
		}
		accountIDs = append(accountIDs, account.AccountID)
	}
	return roleIDs, accountIDs, nil
}

// pickAudienceRole prefers the role every OGS has over whatever sorts first.
func pickAudienceRole(roles []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}) (string, bool) {
	if len(roles) == 0 {
		return "", false
	}
	for _, role := range roles {
		if strings.EqualFold(role.Name, "betreuungskraft") {
			return role.ID, true
		}
	}
	return roles[0].ID, true
}

// folderIDFromResponse reads the folder id out of the create response. Ids
// travel as decimal strings on this API, so the seeder never has to know how
// wide they are.
func folderIDFromResponse(raw []byte) (string, error) {
	var payload struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("decode folder response: %w", err)
	}
	if payload.Data.ID == "" {
		return "", fmt.Errorf("folder response carried no id")
	}
	return payload.Data.ID, nil
}

// --- demo file contents ------------------------------------------------------
//
// The upload endpoint validates magic bytes and, for Office formats, the parts
// inside the OOXML container. Demo files therefore have to be real files: a
// placeholder with a .pdf name is rejected, exactly as it should be.

// demoPDF renders a one-page PDF with a heading and a few lines, using the
// base-14 Helvetica every reader substitutes, so nothing has to be embedded.
func demoPDF(title string, lines []string) []byte {
	var content bytes.Buffer
	content.WriteString("BT\n/F1 20 Tf\n72 780 Td\n(" + pdfString(title) + ") Tj\n/F1 12 Tf\n")
	for _, line := range lines {
		content.WriteString("0 -28 Td\n(" + pdfString(line) + ") Tj\n")
	}
	content.WriteString("ET")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
	}

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	startxref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, startxref)
	return out.Bytes()
}

// pdfString escapes a literal string and drops what WinAnsi cannot carry, so a
// stray character can never produce a broken file.
func pdfString(s string) string {
	var out strings.Builder
	for _, r := range s {
		switch {
		case r == '(' || r == ')' || r == '\\':
			out.WriteByte('\\')
			out.WriteByte(byte(r))
		case r >= 32 && r < 256:
			out.WriteByte(byte(r))
		default:
			out.WriteByte('?')
		}
	}
	return out.String()
}

// demoDOCX writes the smallest DOCX Word opens: the content-type map, the
// package relationships, and one document part with the paragraphs.
func demoDOCX(title string, paragraphs []string) []byte {
	var body strings.Builder
	for _, text := range append([]string{title}, paragraphs...) {
		body.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + xmlText(text) + `</w:t></w:r></w:p>`)
	}

	parts := []struct {
		name    string
		content string
	}{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>` +
			`</Relationships>`},
		{"word/document.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:body>` + body.String() + `</w:body></w:document>`},
	}

	var buf bytes.Buffer
	archive := zip.NewWriter(&buf)
	for _, part := range parts {
		writer, err := archive.Create(part.name)
		if err != nil {
			// Writing to a bytes.Buffer cannot fail; a change that makes it
			// fail must not ship a file the upload endpoint would reject.
			panic(fmt.Sprintf("build demo docx part %s: %v", part.name, err))
		}
		if _, err := writer.Write([]byte(part.content)); err != nil {
			panic(fmt.Sprintf("write demo docx part %s: %v", part.name, err))
		}
	}
	if err := archive.Close(); err != nil {
		panic(fmt.Sprintf("close demo docx: %v", err))
	}
	return buf.Bytes()
}

// xmlText escapes the characters that would break the document part.
func xmlText(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(s)
}
