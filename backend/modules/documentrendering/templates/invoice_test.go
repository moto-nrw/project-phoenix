package templates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/documentrendering/templates"
)

var invoiceChildren = []templates.InvoiceChild{
	{Name: "Lina", Subtotal: "50,00 €", Lines: []templates.InvoiceLine{
		{Week: "Woche 19.10. bis 23.10.2026", TimeModel: "Vormittag", Amount: "50,00 €"},
	}},
	{Name: "Ben", Subtotal: "42,50 €", Lines: []templates.InvoiceLine{
		{Week: "Woche 19.10. bis 23.10.2026", TimeModel: "Nachmittag", Amount: "65,00 €"},
		{Text: "Ermäßigung für Geschwister", Amount: "-22,50 €"},
	}},
}

func TestInvoiceRowsInTheStandardLayoutNameTheChildOnEveryLine(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []templates.Row{
		{Label: "Lina: Woche 19.10. bis 23.10.2026, Vormittag", Value: "50,00 €"},
		{Label: "Ben: Woche 19.10. bis 23.10.2026, Nachmittag", Value: "65,00 €"},
		{Label: "Ben: Ermäßigung für Geschwister", Value: "-22,50 €"},
		{Label: "Gesamtbetrag", Value: "92,50 €", Bold: true},
	}, templates.InvoiceRows(templates.InvoiceLayout{}, invoiceChildren, "92,50 €"))
}

func TestInvoiceRowsGroupedByChildCloseEachChildWithItsSubtotal(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []templates.Row{
		{Label: "Lina", Bold: true},
		{Label: "Woche 19.10. bis 23.10.2026, Vormittag", Value: "50,00 €"},
		{Label: "Summe für Lina", Value: "50,00 €"},
		{Label: "Ben", Bold: true},
		{Label: "Woche 19.10. bis 23.10.2026, Nachmittag", Value: "65,00 €"},
		{Label: "Ermäßigung für Geschwister", Value: "-22,50 €"},
		{Label: "Summe für Ben", Value: "42,50 €"},
		{Label: "Gesamtbetrag", Value: "92,50 €", Bold: true},
	}, templates.InvoiceRows(templates.InvoiceLayout{GroupByChild: true}, invoiceChildren, "92,50 €"))
}

// The week is the period of the service, a mandatory detail of an invoice:
// no layout leaves it out. Discounts, amounts and the total stay too.
func TestInvoiceRowsLeaveOutTheTimeModelButNeverTheWeekOrAnAmount(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []templates.Row{
		{Label: "Lina: Woche 19.10. bis 23.10.2026", Value: "50,00 €"},
		{Label: "Ben: Woche 19.10. bis 23.10.2026", Value: "65,00 €"},
		{Label: "Ben: Ermäßigung für Geschwister", Value: "-22,50 €"},
		{Label: "Gesamtbetrag", Value: "92,50 €", Bold: true},
	}, templates.InvoiceRows(templates.InvoiceLayout{HideTimeModels: true}, invoiceChildren, "92,50 €"))
	// A booking without week and time model still has a label.
	rows := templates.InvoiceRows(templates.InvoiceLayout{HideTimeModels: true},
		[]templates.InvoiceChild{{Name: "Lina", Lines: []templates.InvoiceLine{{TimeModel: "Vormittag", Amount: "5,00 €"}}}}, "5,00 €")
	assert.Equal(t, "Lina: Betreuung", rows[0].Label)
}

func TestCheckRefusesAnInvoiceLayoutOnAKindWithoutATableOfAmounts(t *testing.T) {
	t.Parallel()
	layout := templates.InvoiceLayout{GroupByChild: true}
	assert.NoError(t, templates.Check(templates.KindConfirmation, templates.Template{Subject: "x", Body: "x", Invoice: layout}))
	err := templates.Check(templates.KindContract, templates.Template{Subject: "x", Body: "x", Invoice: layout})
	var invalid *templates.InvalidError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, map[string]string{"invoice": "has no table of amounts"}, invalid.Fields)
	assert.True(t, templates.HasInvoice(templates.KindInvoice))
	assert.False(t, templates.HasInvoice(templates.KindCancellation))
}

func TestNormalizeKeepsTheInvoiceLayout(t *testing.T) {
	t.Parallel()
	layout := templates.InvoiceLayout{GroupByChild: true, HideTimeModels: true}
	assert.Equal(t, layout, templates.Normalize(templates.Template{Subject: "x", Body: "x", Invoice: layout}).Invoice)
}

func TestFillSampleShowsAddressNumbersAndTheChosenTable(t *testing.T) {
	t.Parallel()
	template := templates.Default(templates.KindConfirmation)
	template.Invoice = templates.InvoiceLayout{GroupByChild: true}
	template.Body = "Ihre Debitorennummer: {{debitorennummer}}\n\n{{positionen}}"
	letter := templates.FillSample(templates.KindConfirmation, template)
	assert.Equal(t, []string{"Familie Muster", "Erika Muster", "Lindenstraße 4", "49143 Bissendorf"}, letter.Recipient)
	assert.Equal(t, []templates.Row{
		{Label: "Rechnungsnummer", Value: "FE-2026-0001"},
		{Label: "Debitorennummer", Value: "10234"},
		{Label: "Datum", Value: "02.11.2026"},
		{Label: "Zahlen bis", Value: "16.11.2026"},
	}, letter.Facts)
	require.Len(t, letter.Blocks, 2)
	assert.Equal(t, "Ihre Debitorennummer: 10234", letter.Blocks[0].Text)
	assert.Equal(t, templates.Row{Label: "Lina", Bold: true}, letter.Blocks[1].Rows[0])

	// A kind without numbers prints only the date beside the address.
	contract := templates.FillSample(templates.KindContract, templates.Default(templates.KindContract))
	assert.Equal(t, []templates.Row{{Label: "Datum", Value: "02.11.2026"}}, contract.Facts)
}
