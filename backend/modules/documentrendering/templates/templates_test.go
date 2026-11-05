package templates_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
)

func TestEveryStandardTemplateUsesOnlyItsOwnPlaceholders(t *testing.T) {
	t.Parallel()
	require.Len(t, templates.Kinds(), 7)
	for _, kind := range templates.Kinds() {
		assert.NoError(t, templates.Check(kind, templates.Default(kind)), kind)
		// The sample fills every placeholder of the kind, so the preview
		// shows each one.
		sample := templates.SampleValues(kind, templates.InvoiceLayout{})
		for _, placeholder := range templates.Placeholders(kind) {
			assert.Contains(t, sample, placeholder.Name, "%s: %s", kind, placeholder.Name)
		}
	}
}

func TestCheckRefusesUnknownPlaceholders(t *testing.T) {
	t.Parallel()
	template := templates.Template{
		Subject: "Rechnung {{ rechnungsnummer }}",
		Body:    "Hallo {{vorname}}, {{familienname}} {{ notfallnummern }} {{vorname}}",
		Closing: "{{unterschrift}}",
	}
	err := templates.Check(templates.KindInvoice, template)
	var unknown *templates.UnknownPlaceholdersError
	require.ErrorAs(t, err, &unknown)
	// notfallnummern is a placeholder of the confirmation letter, not of an
	// invoice; every name is named once and sorted.
	assert.Equal(t, []string{"notfallnummern", "unterschrift", "vorname"}, unknown.Names)
	assert.NoError(t, templates.Check(templates.KindConfirmation, templates.Template{Subject: "x", Body: "{{ notfallnummern }}"}))
}

func TestCheckRefusesPlaceholdersWrittenWrongly(t *testing.T) {
	t.Parallel()
	err := templates.Check(templates.KindInvoice, templates.Template{
		Subject: "Rechnung {{rechnungsnummer}",
		Body:    "Hallo {familienname}}, {{}} und {{kinder}}.",
	})
	var unknown *templates.UnknownPlaceholdersError
	require.ErrorAs(t, err, &unknown)
	assert.Equal(t, []string{"{familienname}},", "{{rechnungsnummer}", "{{}}"}, unknown.Names)
}

func TestCheckNeedsSubjectAndBodyWithinTheirLimits(t *testing.T) {
	t.Parallel()
	err := templates.Check(templates.KindInvoice, templates.Template{Closing: strings.Repeat("a", templates.MaxClosingLength+1)})
	var invalid *templates.InvalidError
	require.ErrorAs(t, err, &invalid)
	assert.True(t, errors.Is(err, templates.ErrInvalid))
	assert.Equal(t, map[string]string{"subject": "is required", "body": "is required", "closing": "is too long"}, invalid.Fields)
	assert.ErrorIs(t, templates.Check("brief", templates.Template{Subject: "x", Body: "x"}), templates.ErrUnknownKind)
}

func TestNormalizeTrimsAndKeepsSubjectOnOneLine(t *testing.T) {
	t.Parallel()
	got := templates.Normalize(templates.Template{Subject: "  Rechnung\r\n {{rechnungsnummer}} ", Body: "\r\nA\r\n\r\nB\r\n", Closing: " Gruß "})
	assert.Equal(t, templates.Template{Subject: "Rechnung {{rechnungsnummer}}", Body: "A\n\nB", Closing: "Gruß"}, got)
}

func TestFillBuildsParagraphsHeadingsBoldAndTables(t *testing.T) {
	t.Parallel()
	template := templates.Template{
		Subject:    "Rechnung {{rechnungsnummer}}",
		Salutation: "Hallo {{familienname}},",
		Body: "Für {{kinder}}:\n{{positionen}}\n\n" +
			"**Bitte zahlen Sie {{summe}}.**\n\n" +
			"# Hinweise\n{{hinweise}}\n\n" +
			"# Notfall\nRufen Sie an:\n{{notfallnummern}}\n\n" +
			"Notfall im Satz: {{notfallnummern}}\n\n" +
			"Fester Satz ohne Platzhalter.",
		Closing: "Gruß\n\n{{traeger}}\n{{ansprechperson}}",
	}
	rows := []templates.Row{{Label: "Lina", Value: "50,00 €"}, {Label: "Gesamt", Value: "50,00 €", Bold: true}}
	got := templates.Fill(template, map[string]templates.Value{
		"rechnungsnummer": {Text: "FE-1"},
		"familienname":    {Text: "Muster"},
		"kinder":          {Text: "Lina"},
		"positionen":      {Rows: rows},
		"summe":           {Text: "50,00 €"},
		"traeger":         {Text: "Träger e.V."},
		"notfallnummern":  {Rows: []templates.Row{{Label: "OGS", Value: "0170 1"}}},
	})
	assert.Equal(t, templates.Filled{
		Subject:    "Rechnung FE-1",
		Salutation: "Hallo Muster,",
		Blocks: []templates.Block{
			{Text: "Für Lina:", Rows: rows},
			{Text: "Bitte zahlen Sie 50,00 €.", Bold: true},
			// "# Hinweise" is left out: its only placeholder is empty.
			{Heading: "Notfall", Text: "Rufen Sie an:", Rows: []templates.Row{{Label: "OGS", Value: "0170 1"}}},
			{Text: "Notfall im Satz: OGS 0170 1"},
			{Text: "Fester Satz ohne Platzhalter."},
		},
		Closing: []string{"Gruß", "Träger e.V."},
	}, got)
}

func TestFillDropsAParagraphWhoseTableIsEmpty(t *testing.T) {
	t.Parallel()
	got := templates.Fill(templates.Template{Subject: "x", Body: "# Notfall\nRufen Sie an:\n{{notfallnummern}}\n\n{{positionen}}"},
		map[string]templates.Value{"positionen": {Rows: []templates.Row{{Label: "A", Value: "1"}}}})
	assert.Equal(t, []templates.Block{{Rows: []templates.Row{{Label: "A", Value: "1"}}}}, got.Blocks)
}

func TestSampleFillsTheStandardTemplateWithoutLeftoverPlaceholders(t *testing.T) {
	t.Parallel()
	leftover := regexp.MustCompile(`\{\{|\}\}`)
	for _, kind := range templates.Kinds() {
		filled := templates.Fill(templates.Default(kind), templates.SampleValues(kind, templates.InvoiceLayout{}))
		assert.NotEmpty(t, filled.Subject, kind)
		assert.NotEmpty(t, filled.Blocks, kind)
		for _, block := range filled.Blocks {
			assert.False(t, leftover.MatchString(block.Heading+block.Text), "%s: %q", kind, block.Text)
		}
	}
}

func TestFillPreservesStandaloneTablesAnywhereInAParagraph(t *testing.T) {
	t.Parallel()
	rows := []templates.Row{{Label: "Gesamt", Value: "50,00 €", Bold: true}}
	for _, tt := range []struct {
		name string
		body string
		want []templates.Block
	}{
		{"first", "{{positionen}}\nBitte überweisen Sie den Betrag.", []templates.Block{{Rows: rows}, {Text: "Bitte überweisen Sie den Betrag."}}},
		{"middle", "# Rechnung\nFür Lina:\n{{positionen}}\n**Bitte zahlen Sie.**", []templates.Block{{Heading: "Rechnung", Text: "Für Lina:", Rows: rows}, {Text: "Bitte zahlen Sie.", Bold: true}}},
		{"multiple", "{{positionen}}\nDazwischen\n{{positionen}}\nDanach", []templates.Block{{Rows: rows}, {Text: "Dazwischen", Rows: rows}, {Text: "Danach"}}},
		{"empty", "# Rechnung\n{{positionen}}\nHinweis: {{familienname}}", []templates.Block{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			values := map[string]templates.Value{"positionen": {Rows: rows}}
			if tt.name == "empty" {
				values = nil
			}
			template := templates.Template{Subject: "Rechnung", Body: tt.body}
			require.NoError(t, templates.Check(templates.KindInvoice, template))
			assert.Equal(t, tt.want, templates.Fill(template, values).Blocks)
		})
	}
}
