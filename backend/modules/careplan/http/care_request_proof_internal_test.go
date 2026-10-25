package careplanhttp

import (
	"testing"
	"time"

	carerequests "github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCareRequestProofReport pins the OGS proof of a signed parent request
// (#4780): child, wording, procedure, integrity and the signature image.
func TestCareRequestProofReport(t *testing.T) {
	t.Parallel()

	item := &carerequests.HistoryItem{
		Request:   &carerequests.Request{RequestKind: "pickup_change", Status: carerequests.StatusPending, CreatedAt: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)},
		FirstName: "Mia", LastName: "Muster",
		Requested: []carerequests.DiffEntry{{Label: "10.11.2026 · Abholperson", New: "Erika Muster"}},
	}
	signature := &PickupSignatureProof{Method: "drawn", SignerName: "Sabine Muster", PNG: []byte{1}, SignedAt: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC), Intact: true}
	doc := careRequestProofReport(item, "Ich erlaube, dass Erika Muster das Kind Mia Muster am Dienstag, 10.11.2026 von der OGS abholt.", signature, time.Now())

	require.Len(t, doc.Sections, 1)
	cards := doc.Sections[0].Cards
	require.Len(t, cards, 2)
	assert.Equal(t, "Einzelner Tag", cards[0].Title)
	values := map[string]string{}
	for _, field := range cards[0].Fields {
		values[field.Label] = field.Value
	}
	assert.Equal(t, "Mia Muster", values["Kind"])
	assert.Equal(t, "Einfache elektronische Signatur mit gezeichneter Unterschrift", values["Verfahren"])
	assert.Equal(t, "Unterschrift und Text sind unverändert.", values["Prüfung"])
	assert.Equal(t, "Erika Muster", values["10.11.2026 · Abholperson"])
	assert.Equal(t, []byte{1}, cards[1].Fields[1].PNG)

	broken := careRequestProofReport(item, "x", nil, time.Now())
	for _, field := range broken.Sections[0].Cards[0].Fields {
		if field.Label == "Prüfung" {
			assert.Contains(t, field.Value, "Achtung")
		}
	}
	assert.Len(t, broken.Sections[0].Cards, 1, "a missing signature prints no signature card")
}
