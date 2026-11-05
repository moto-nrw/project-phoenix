package templates

// The made-up document of the preview (#4677, #5691): a family with two
// children, an invoice with its numbers and the address field, so the
// Träger-Büro checks text, numbers and table in context before a letter
// goes out.

const sampleDebtorNumber = "10234"

// sampleChildren are the positions of the sample invoice.
var sampleChildren = []InvoiceChild{
	{Name: "Lina", Subtotal: "50,00 €", Lines: []InvoiceLine{
		{Week: "Woche 19.10. bis 23.10.2026", TimeModel: "Vormittag", Amount: "50,00 €"},
	}},
	{Name: "Ben", Subtotal: "42,50 €", Lines: []InvoiceLine{
		{Week: "Woche 19.10. bis 23.10.2026", TimeModel: "Nachmittag", Amount: "65,00 €"},
		{Text: "Ermäßigung für Geschwister", Amount: "-22,50 €"},
	}},
}

const sampleTotal = "92,50 €"

// sampleRecipient is the address field of the sample letter.
var sampleRecipient = []string{"Familie Muster", "Erika Muster", "Lindenstraße 4", "49143 Bissendorf"}

// SampleLetter is a template filled with the sample document, with the
// address field and the numbers the letter prints beside it. The contact
// person of the Träger completes the numbers; the preview takes it from the
// Träger's master data.
type SampleLetter struct {
	Recipient []string `json:"recipient"`
	Facts     []Row    `json:"facts"`
	Filled
}

// FillSample fills a template of kind with the sample document in the
// template's own table of amounts.
func FillSample(kind Kind, template Template) SampleLetter {
	values := SampleValues(kind, template.Invoice)
	return SampleLetter{
		Recipient: append([]string{}, sampleRecipient...),
		Facts:     sampleFacts(kind, values),
		Filled:    Fill(template, values),
	}
}

// SampleValues are the made-up values the preview fills a template of kind
// with, its table of amounts in layout.
func SampleValues(kind Kind, layout InvoiceLayout) map[string]Value {
	values := make(map[string]Value, len(catalog[kind].placeholders))
	for name, value := range samples {
		if knows(kind, name) {
			values[name] = value
		}
	}
	if HasInvoice(kind) {
		values[pPositions.Name] = Value{Rows: InvoiceRows(layout, sampleChildren, sampleTotal)}
	}
	return values
}

// sampleFacts are the numbers beside the address field, as the letter of
// the kind prints them: the number of the document, the debtor number, the
// date and, where the document asks for money, the payment date.
func sampleFacts(kind Kind, values map[string]Value) []Row {
	var facts []Row
	for _, fact := range []struct{ label, name string }{
		{"Rechnungsnummer", pNumber.Name},
		{"Bescheidnummer", pNoticeNumber.Name},
	} {
		if value, ok := values[fact.name]; ok {
			facts = append(facts, Row{Label: fact.label, Value: value.Text})
		}
	}
	if len(facts) > 0 {
		facts = append(facts, Row{Label: "Debitorennummer", Value: sampleDebtorNumber})
	}
	facts = append(facts, Row{Label: "Datum", Value: samples[pDate.Name].Text})
	if _, ok := values[pDue.Name]; ok {
		facts = append(facts, Row{Label: "Zahlen bis", Value: samples[pDue.Name].Text})
	}
	return facts
}
