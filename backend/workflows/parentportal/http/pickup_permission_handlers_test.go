package parent

import (
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pickupProofFixture(intact bool) *parentService.PickupProof {
	return &parentService.PickupProof{
		Request:    carerequests.Request{ID: 12, RequestKind: carerequests.KindPickupPermission, Status: carerequests.StatusApproved},
		ChildName:  "Mia Muster",
		SchoolName: "OGS am Berg",
		Statement:  "Ich erlaube, dass Erika Muster das Kind Mia Muster dienstags von der OGS abholt.",
		Summary:    []carerequests.DiffEntry{{Label: "Abholberechtigung · Erika Muster", Old: "Nicht abholberechtigt", New: "Nur Di"}},
		Signature: &parentService.PickupSignature{
			Method: "typed", TypedName: "Sabine Muster", SignerName: "Sabine Muster",
			PNG: []byte{0x89, 'P', 'N', 'G'}, SignedAt: time.Date(2026, 11, 3, 11, 0, 0, 0, time.UTC),
		},
		Intact: intact,
	}
}

// TestPickupProofReportNamesTheProcedureAndTheSignature pins the PDF
// description of the guardian's proof (#4780).
func TestPickupProofReportNamesTheProcedureAndTheSignature(t *testing.T) {
	t.Parallel()

	doc := pickupProofReport(pickupProofFixture(true), time.Date(2026, 11, 4, 9, 0, 0, 0, time.UTC))
	require.Len(t, doc.Sections, 1)
	cards := doc.Sections[0].Cards
	require.Len(t, cards, 2)

	values := map[string]string{}
	for _, field := range cards[0].Fields {
		values[field.Label] = field.Value
	}
	assert.Equal(t, "Einfache elektronische Signatur mit gezeichneter Unterschrift", values["Verfahren"])
	assert.Equal(t, "Von der OGS bestätigt", values["Stand"])
	assert.Equal(t, "Unterschrift und Text sind unverändert.", values["Prüfung"])
	assert.Contains(t, values["Erklärung"], "dienstags")
	assert.Equal(t, "Nicht abholberechtigt → Nur Di", values["Abholberechtigung · Erika Muster"])

	assert.True(t, strings.HasPrefix(cards[1].Title, "Unterschrift am 03.11.2026"))
	require.Len(t, cards[1].Fields, 2)
	assert.Equal(t, "Name getippt: Sabine Muster", cards[1].Fields[1].Value)
	assert.NotEmpty(t, cards[1].Fields[1].PNG)
}

func TestPickupProofReportWarnsAboutAChangedEntry(t *testing.T) {
	t.Parallel()

	doc := pickupProofReport(pickupProofFixture(false), time.Now())
	found := false
	for _, field := range doc.Sections[0].Cards[0].Fields {
		if field.Label == "Prüfung" {
			found = true
			assert.Contains(t, field.Value, "nachträglich verändert")
		}
	}
	assert.True(t, found)
}

func TestPickupProofResponseCarriesTheImageAsDataURL(t *testing.T) {
	t.Parallel()

	response := toPickupProofResponse(pickupProofFixture(true), time.Now())
	assert.Equal(t, "12", response.RequestID)
	assert.True(t, response.IntegrityOK)
	require.NotNil(t, response.Signature)
	assert.True(t, strings.HasPrefix(response.Signature.ImageDataURL, "data:image/png;base64,"))
	require.Len(t, response.Summary, 1)
}

func TestParsePickupPersonID(t *testing.T) {
	t.Parallel()

	id, err := parsePickupPersonID(nil)
	require.NoError(t, err)
	assert.Zero(t, id)
	value := "42"
	id, err = parsePickupPersonID(&value)
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
	invalid := "abc"
	_, err = parsePickupPersonID(&invalid)
	assert.Error(t, err)
}
