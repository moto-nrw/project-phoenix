package templates

import "strings"

// The presentation of the table of amounts (#5691): a Träger chooses how
// the table {{positionen}} lists the positions of an invoice. The amount of
// each position, every discount and correction and the total stay fixed:
// they are what the invoice asks for. The zero value is the standard table
// of ADR 0121, one line per position with the child's first name, the week
// and the time model, so templates saved before #5691 print as they did.

// InvoiceLayout is how the table of amounts shows the positions.
type InvoiceLayout struct {
	// GroupByChild prints the first name of each child once above its
	// positions and closes them with the child's subtotal, instead of
	// naming the child on every line.
	GroupByChild bool `json:"group_by_child"`
	// HideWeeks leaves the week of a booking out of its line.
	HideWeeks bool `json:"hide_weeks"`
	// HideTimeModels leaves the time model of a booking out of its line.
	HideTimeModels bool `json:"hide_time_models"`
}

// InvoiceChild is one child of an invoice with its positions and its
// subtotal, formatted for print.
type InvoiceChild struct {
	Name     string
	Lines    []InvoiceLine
	Subtotal string
}

// InvoiceLine is one position. A booking names its Week and TimeModel; any
// other position (a discount, an exemption, a correction) names only Text,
// which every layout prints.
type InvoiceLine struct {
	Week      string
	TimeModel string
	Text      string
	Amount    string
}

// fallbackBookingLabel names a booking whose week and time model are both
// left out, so no line of the table is without a label.
const fallbackBookingLabel = "Betreuung"

// HasInvoice reports whether a template of kind prints the table of
// amounts and so has an invoice layout to choose.
func HasInvoice(kind Kind) bool {
	return knows(kind, pPositions.Name)
}

// InvoiceRows lays out the table of amounts: the positions of every child in
// the chosen layout and, last and bold, the total.
func InvoiceRows(layout InvoiceLayout, children []InvoiceChild, total string) []Row {
	rows := []Row{}
	for _, child := range children {
		name := strings.TrimSpace(child.Name)
		if layout.GroupByChild && name != "" {
			rows = append(rows, Row{Label: name, Bold: true})
		}
		for _, line := range child.Lines {
			label := line.label(layout)
			if !layout.GroupByChild && name != "" {
				label = name + ": " + label
			}
			rows = append(rows, Row{Label: label, Value: line.Amount})
		}
		if layout.GroupByChild && name != "" && len(child.Lines) > 0 {
			rows = append(rows, Row{Label: "Summe für " + name, Value: child.Subtotal})
		}
	}
	return append(rows, Row{Label: "Gesamtbetrag", Value: total, Bold: true})
}

func (l InvoiceLine) label(layout InvoiceLayout) string {
	if text := strings.TrimSpace(l.Text); text != "" {
		return text
	}
	var parts []string
	if week := strings.TrimSpace(l.Week); week != "" && !layout.HideWeeks {
		parts = append(parts, week)
	}
	if model := strings.TrimSpace(l.TimeModel); model != "" && !layout.HideTimeModels {
		parts = append(parts, model)
	}
	if len(parts) == 0 {
		return fallbackBookingLabel
	}
	return strings.Join(parts, ", ")
}
