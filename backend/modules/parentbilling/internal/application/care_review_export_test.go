package application

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

func TestCareReviewTableKeepsSignedReductions(t *testing.T) {
	t.Parallel()
	table := careReviewTable(parentbilling.CareRunDetail{
		Invoices: []parentbilling.CareInvoice{{
			Positions:  []parentbilling.CarePosition{{AmountCents: 10000}},
			Reductions: []parentbilling.CareReduction{{Name: "Ermäßigung", AmountCents: -2000}},
			TotalCents: 8000,
		}},
	})
	require.Len(t, table.Rows, 3)
	assert.Equal(t, "100,00", table.Rows[0]["amount"])
	assert.Equal(t, "-20,00", table.Rows[1]["amount"], "both export formats use the signed reduction")
	assert.Equal(t, "80,00", table.Rows[2]["amount"])
}
