package careplanhttp

import (
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func accidentReportForPDF() careplan.AccidentReport {
	return careplan.AccidentReport{
		OccurredAt: time.Date(2026, 10, 20, 8, 40, 0, 0, time.UTC), Location: "Schulhof",
		Description: "Vom Klettergerüst gefallen.", InjuryKind: "Prellung", Supervisor: "Frau Berger",
	}
}

func reportFields(doc ReportDocument) map[string]string {
	fields := map[string]string{}
	for _, section := range doc.Sections {
		for _, card := range section.Cards {
			for _, field := range card.Fields {
				fields[field.Label] = field.Value
			}
		}
	}
	return fields
}

// The PDF follows the numbered fields of the official form; what moto does
// not record says so instead of staying blank.
func TestAccidentReportDocumentFollowsTheOfficialForm(t *testing.T) {
	t.Parallel()
	report := accidentReportForPDF()
	doc := accidentReportDocument(careplan.AccidentReportFile{Document: careplan.CareDocument{AccidentReport: &report}},
		accidentReportFacts{SchoolName: "OGS Sonnenschule", SchoolAddress: "Hauptstraße 1, 48143 Münster", Name: "Muster, Mia", Birthday: "2018-03-04"},
		time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC))

	fields := reportFields(doc)
	assert.Equal(t, "Unfallanzeige", doc.Title)
	assert.Equal(t, "OGS Sonnenschule, Hauptstraße 1, 48143 Münster", fields["1 Name und Anschrift der Einrichtung"])
	assert.Equal(t, accidentNotRecorded, fields["3 Unternehmensnummer des Unfallversicherungsträgers"])
	assert.Equal(t, "04.03.2018", fields["6 Geburtsdatum"])
	assert.Equal(t, "20.10.2026, 10:40 Uhr", fields["12 Unfallzeitpunkt"], "the time prints in Berlin")
	assert.Equal(t, "Kein Arztbesuch", fields["20 Erstbehandelnde Ärztin, erstbehandelnder Arzt oder Krankenhaus"])
	assert.Equal(t, "Noch nicht unterschrieben", fields["Unterschrift"])
	assert.Contains(t, doc.Filters, "Stand: offen")
}

func TestAccidentReportDocumentShowsSignatureAndItsCheck(t *testing.T) {
	t.Parallel()
	report := accidentReportForPDF()
	signedAt := time.Date(2026, 10, 21, 7, 30, 0, 0, time.UTC)
	signatureID, signer := int64(5), int64(9)
	file := careplan.AccidentReportFile{
		Document: careplan.CareDocument{AccidentReport: &report, AccidentSignatureID: &signatureID, AccidentSignedAt: &signedAt, AccidentSignedBy: &signer},
		Signature: &careplan.AccidentSignature{
			Method: "drawn", SignerName: "Petra Leitung", PNG: []byte{1}, SignedAt: signedAt, Intact: false,
		},
	}
	fields := reportFields(accidentReportDocument(file, accidentReportFacts{}, signedAt))
	assert.Equal(t, "Petra Leitung", fields["Unterschrieben von"])
	assert.Equal(t, "Einfache elektronische Signatur mit gezeichneter Unterschrift", fields["Verfahren"])
	assert.Contains(t, fields["Prüfung"], "nachträglich verändert", "a broken checksum is printed, never hidden")

	file.Signature.Method = "typed"
	file.Signature.TypedName = "Petra Leitung"
	fields = reportFields(accidentReportDocument(file, accidentReportFacts{}, signedAt))
	assert.Equal(t, "Einfache elektronische Signatur mit getipptem Namen", fields["Verfahren"])

	supersedes := int64(3)
	file.Document.AccidentSupersedesID = &supersedes
	assert.Equal(t, "Unfallanzeige (Korrektur)", accidentReportDocument(file, accidentReportFacts{}, signedAt).Title)
}

func TestAccidentReportVersionsFollowTheCorrections(t *testing.T) {
	t.Parallel()
	report := accidentReportForPDF()
	first, second := int64(1), int64(2)
	docs := []careplan.CareDocument{
		{ID: 3, AccidentReport: &report, AccidentSupersedesID: &second},
		{ID: 2, AccidentReport: &report, AccidentSupersedesID: &first},
		{ID: 1, AccidentReport: &report},
		{ID: 4},
	}
	versions, successors := accidentReportVersions(docs)
	assert.Equal(t, map[int64]int{1: 1, 2: 2, 3: 3}, versions)
	assert.Equal(t, map[int64]int64{1: 2, 2: 3}, successors)
}

// Every accident report failure answers with its own registered code.
func TestAccidentReportErrorsCarryTheirCodes(t *testing.T) {
	t.Parallel()
	cases := map[error]string{
		careplan.ErrStudentDocumentForbidden: common.CodeStudentsAccidentReportForbidden,
		careplan.ErrStudentDocumentNoAccess:  common.CodeStudentsAccidentReportForbidden,
		careplan.ErrAccidentReportInvalid:    common.CodeStudentsAccidentReportInvalid,
		careplan.ErrAccidentSignatureInvalid: common.CodeStudentsAccidentSignatureInvalid,
		careplan.ErrAccidentReportSigned:     common.CodeStudentsAccidentReportSigned,
		careplan.ErrAccidentReportNotSigned:  common.CodeStudentsAccidentReportNotSigned,
		careplan.ErrAccidentReportSuperseded: common.CodeStudentsAccidentReportSuperseded,
		careplan.ErrAccidentReportNotFound:   common.CodeStudentsAccidentReportNotFound,
		careplan.ErrCareDocumentNotFound:     common.CodeStudentsAccidentReportNotFound,
	}
	for err, code := range cases {
		renderer := accidentReportErrorRenderer(errors.Join(errors.New("wrapped"), err))
		response, ok := renderer.(*common.ErrResponse)
		require.True(t, ok, err.Error())
		assert.Equal(t, code, response.Code, err.Error())
		recorder := httptest.NewRecorder()
		require.NoError(t, render.Render(recorder, httptest.NewRequest("GET", "/", nil), renderer))
	}
}
