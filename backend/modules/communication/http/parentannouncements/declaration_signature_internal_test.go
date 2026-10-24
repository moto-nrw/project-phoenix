package announcement

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

func signedStatusFixture() *announcementService.ParentDeclarationStatus {
	s := declarationStatusFixture()
	s.Settings.RequiresSignature = true
	s.Submissions[0].Method = announcementService.DeclarationMethodSimpleElectronicSignature
	s.Submissions[0].Signature = &announcementService.DeclarationSignature{
		Method: "drawn", PNG: []byte("\x89PNG-bytes"), ImageSHA256: strings.Repeat("d", 64),
		DocumentSHA256: strings.Repeat("a", 64), SignedAt: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC), IntegrityOK: true,
	}
	return s
}

// The OGS report prints the setting, the procedure and every signature
// image (#4785).
func TestDeclarationReportPrintsSignatures(t *testing.T) {
	t.Parallel()
	doc := declarationReport(signedStatusFixture())
	text := reportTexts(doc)
	assert.Contains(t, text, "Unterschrift=Ja, beim Zustimmen")
	assert.Contains(t, text, "Verfahren=Einfache elektronische Signatur im angemeldeten Eltern-Konto")
	var printed []byte
	for _, section := range doc.Sections {
		for _, card := range section.Cards {
			for _, f := range card.Fields {
				if f.Label == "Unterschrift" && len(f.PNG) > 0 {
					printed = f.PNG
				}
			}
		}
	}
	assert.Equal(t, []byte("\x89PNG-bytes"), printed)
}

func TestDeclarationReportDoesNotPrintInvalidSignatures(t *testing.T) {
	t.Parallel()
	for name, invalidate := range map[string]func(*announcementService.ParentDeclarationStatus){
		"submission": func(status *announcementService.ParentDeclarationStatus) {
			status.Submissions[0].IntegrityOK = false
		},
		"signature": func(status *announcementService.ParentDeclarationStatus) {
			status.Submissions[0].Signature.IntegrityOK = false
		},
	} {
		t.Run(name, func(t *testing.T) {
			status := signedStatusFixture()
			invalidate(status)

			doc := declarationReport(status)
			for _, section := range doc.Sections {
				for _, card := range section.Cards {
					for _, field := range card.Fields {
						if field.Label == "Unterschrift" {
							assert.Empty(t, field.PNG)
						}
					}
				}
			}
			assert.NotContains(t, reportTexts(doc), "Prüfsumme der unterschriebenen Fassung=")
		})
	}
}

// The CSV names the signature and its checksum; an unsigned row says so.
func TestDeclarationCSVCarriesSignatureColumns(t *testing.T) {
	t.Parallel()
	file, err := declarationCSV(signedStatusFixture())
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(file.Data)), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "Unterschrift;Unterschrift getippter Name;Prüfsumme Unterschrift;Unterschrift unverändert")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(lines[1]), "gezeichnet;;"+strings.Repeat("d", 64)+";ja"), lines[1])
	assert.Equal(t, strings.Count(lines[0], ";"), strings.Count(lines[1], ";"))

	unsigned, err := declarationCSV(declarationStatusFixture())
	require.NoError(t, err)
	assert.Contains(t, string(unsigned.Data), ";nein;;;")
}

// The status JSON carries the setting and the signature without the image.
func TestDeclarationStatusResponseCarriesSignatureWithoutImage(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(toDeclarationStatusResponse(signedStatusFixture()))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"requires_signature":true`)
	assert.Contains(t, string(raw), `"signature":{"method":"drawn","typed_name":null,"signed_at":"2026-10-24T09:00:00Z","integrity_ok":true}`)
	assert.NotContains(t, string(raw), "iVBOR")
}
