package listexport

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"slices"
	"strings"
	"testing"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

func sampleLetter() listrecords.Letter {
	return listrecords.Letter{
		Title:       "Bestätigung FE-2027-0001",
		AccentColor: "#217A78",
		Sender:      []string{"Ganztag im Quartier e.V.", "Am Markt 4", "49143 Bissendorf"},
		Recipient:   []string{"Familie Beispiel", "z. Hd. Erika Beispiel"},
		Facts: []listrecords.LetterFact{
			{Label: "Rechnungsnummer", Value: "FE-2027-0001"},
			{Label: "Datum", Value: "10.12.2026"},
		},
		Subject:    "Ferienbetreuung in den Winterferien 2027",
		Salutation: "Sehr geehrte Familie Beispiel,",
		Blocks: []listrecords.LetterBlock{
			{Text: "hiermit bestätigen wir die Anmeldung von Lina."},
			{Facts: []listrecords.LetterFact{
				{Label: "Lina, Woche 04.01.–08.01.2027, Vormittag", Value: "50,00 €"},
				{Label: "Gesamtbetrag", Value: "50,00 €", Bold: true},
			}},
			{Heading: "Bitte denken Sie an", Text: "Frühstück\nWechselkleidung"},
			{Text: "Kinder mit ansteckenden Erkrankungen bleiben zu Hause.", Bold: true},
		},
		Closing: []string{"Mit freundlichen Grüßen", "Ganztag im Quartier e.V."},
		Footer:  []string{"Bankverbindung: IBAN DE02 1203 0000 0000 2020 51"},
	}
}

func traceLetter(t *testing.T, letter listrecords.Letter) (int, []string) {
	t.Helper()
	chrome, err := newLetterChrome(letter.Title)
	if err != nil {
		t.Fatalf("newLetterChrome: %v", err)
	}
	layout, err := newLetterLayout(chrome, letter)
	if err != nil {
		t.Fatalf("newLetterLayout: %v", err)
	}
	if err := layout.run(); err != nil {
		t.Fatalf("run: %v", err)
	}
	return layout.page, layout.trace
}

// Every part of the letter reaches the page, in reading order: letterhead,
// return line, address field, facts, subject, salutation, body, closing
// and footer.
func TestLetterPrintsEveryPartInOrder(t *testing.T) {
	t.Parallel()

	pages, trace := traceLetter(t, sampleLetter())
	if pages != 1 {
		t.Fatalf("pages = %d, want 1", pages)
	}
	want := []string{
		"Bankverbindung: IBAN DE02 1203 0000 0000 2020 51",
		"Ganztag im Quartier e.V.", "Am Markt 4", "49143 Bissendorf",
		"Ganztag im Quartier e.V. · Am Markt 4 · 49143 Bissendorf",
		"Familie Beispiel", "z. Hd. Erika Beispiel",
		"Rechnungsnummer", "FE-2027-0001", "Datum", "10.12.2026",
		"Ferienbetreuung in den Winterferien 2027",
		"Sehr geehrte Familie Beispiel,",
		"hiermit bestätigen wir die Anmeldung von Lina.",
		"50,00 €", "Lina, Woche 04.01.–08.01.2027, Vormittag",
		"50,00 €", "Gesamtbetrag",
		"Bitte denken Sie an", "Frühstück", "Wechselkleidung",
		"Kinder mit ansteckenden Erkrankungen bleiben zu Hause.",
		"Mit freundlichen Grüßen", "Ganztag im Quartier e.V.",
	}
	if !slices.Equal(trace, want) {
		t.Fatalf("trace =\n%q\nwant\n%q", trace, want)
	}
}

// A long body continues on the next page; the footer is on every page.
func TestLetterContinuesOnTheNextPage(t *testing.T) {
	t.Parallel()

	letter := sampleLetter()
	letter.Blocks = append(letter.Blocks, listrecords.LetterBlock{Text: strings.Repeat("Ein langer Hinweis für die Eltern.\n", 70)})
	pages, trace := traceLetter(t, letter)
	if pages != 2 {
		t.Fatalf("pages = %d, want 2", pages)
	}
	footers := 0
	for _, line := range trace {
		if line == letter.Footer[0] {
			footers++
		}
	}
	if footers != 2 {
		t.Fatalf("footer printed %d times, want once per page", footers)
	}
	if trace[len(trace)-1] != "Ganztag im Quartier e.V." {
		t.Fatalf("the closing must end the letter, got %q", trace[len(trace)-1])
	}
}

func TestRenderLetterWritesAPDFWithLogo(t *testing.T) {
	t.Parallel()

	img := image.NewRGBA(image.Rect(0, 0, 300, 100))
	img.Set(1, 1, color.RGBA{R: 0x21, G: 0x7A, B: 0x78, A: 0xFF})
	var pngLogo, jpegLogo bytes.Buffer
	if err := png.Encode(&pngLogo, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegLogo, img, nil); err != nil {
		t.Fatal(err)
	}
	for name, logo := range map[string][]byte{"png": pngLogo.Bytes(), "jpeg": jpegLogo.Bytes()} {
		letter := sampleLetter()
		letter.Logo = logo
		_, trace := traceLetter(t, letter)
		if !slices.Contains(trace, "[Logo]") {
			t.Fatalf("%s: logo missing from the trace", name)
		}
		file, err := NewService().RenderLetter(letter, "Bestätigung FE-2027-0001")
		if err != nil {
			t.Fatalf("%s: RenderLetter: %v", name, err)
		}
		if !bytes.HasPrefix(file.Data, []byte("%PDF")) || file.ContentType != "application/pdf" {
			t.Fatalf("%s: not a PDF: %q", name, file.ContentType)
		}
		if file.Filename != "bestaetigung-fe-2027-0001.pdf" {
			t.Fatalf("%s: filename = %q", name, file.Filename)
		}
		if pages := countRenderedPDFPages(file.Data); pages != 1 {
			t.Fatalf("%s: pages = %d, want 1", name, pages)
		}
	}
}

func TestLetterLayoutRejectsAnUnreadableLogo(t *testing.T) {
	t.Parallel()

	letter := sampleLetter()
	letter.Logo = []byte("not an image")
	if _, err := NewService().RenderLetter(letter, "x"); err == nil {
		t.Fatal("an unreadable logo must fail the letter")
	}
}

func TestLetterFallsBackToInkForAnUnusableColour(t *testing.T) {
	t.Parallel()

	chrome, err := newLetterChrome("x")
	if err != nil {
		t.Fatal(err)
	}
	letter := sampleLetter()
	letter.AccentColor = "teal"
	layout, err := newLetterLayout(chrome, letter)
	if err != nil {
		t.Fatal(err)
	}
	if layout.accent != colorInk {
		t.Fatalf("accent = %v, want ink", layout.accent)
	}
}

// A long block of facts pushes the subject below it instead of running
// into it.
func TestLetterSubjectStaysBelowALongHead(t *testing.T) {
	t.Parallel()

	chrome, err := newLetterChrome("x")
	if err != nil {
		t.Fatal(err)
	}
	letter := sampleLetter()
	for _, label := range []string{"Zahlen bis", "Ansprechperson", "Telefon", "E-Mail", "Sprechzeit", "Zimmer"} {
		letter.Facts = append(letter.Facts, listrecords.LetterFact{Label: label, Value: "Ein Wert"})
	}
	layout, err := newLetterLayout(chrome, letter)
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.run(); err != nil {
		t.Fatal(err)
	}
	if layout.headBottom <= letterBodyTop {
		t.Fatalf("the head ends at %.1f; the test needs one longer than the default body top", layout.headBottom)
	}
	short, err := newLetterLayout(chrome, sampleLetter())
	if err != nil {
		t.Fatal(err)
	}
	if err := short.run(); err != nil {
		t.Fatal(err)
	}
	if short.headBottom >= letterBodyTop {
		t.Fatalf("a short head ends at %.1f, below the body top", short.headBottom)
	}
}

// A long footer line, such as a bank account with a long holder name,
// wraps and keeps its end instead of being cut.
func TestLetterFooterWrapsLongLines(t *testing.T) {
	t.Parallel()

	letter := sampleLetter()
	letter.Footer = []string{"Bankverbindung: " + strings.Repeat("Förderverein der Offenen Ganztagsschulen ", 4) + "· IBAN DE02 1203 0000 0000 2020 51 · BIC BYLADEM1001"}
	pages, trace := traceLetter(t, letter)
	if pages != 1 {
		t.Fatalf("pages = %d, want 1", pages)
	}
	joined := strings.Join(trace, " ")
	if !strings.Contains(joined, "BIC BYLADEM1001") || !strings.Contains(joined, "IBAN DE02 1203 0000 0000 2020 51") {
		t.Fatalf("the footer lost its end: %q", trace)
	}
	if trace[0] == letter.Footer[0] {
		t.Fatal("the long footer line was printed in one piece")
	}
}

// A long letterhead pushes the facts below it instead of running into
// them.
func TestLetterFactsStartBelowALongLetterhead(t *testing.T) {
	t.Parallel()

	chrome, err := newLetterChrome("x")
	if err != nil {
		t.Fatal(err)
	}
	letter := sampleLetter()
	letter.Facts = letter.Facts[:1]
	for i := range 12 {
		letter.Sender = append(letter.Sender, fmt.Sprintf("Zeile %d", i))
	}
	layout, err := newLetterLayout(chrome, letter)
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.run(); err != nil {
		t.Fatal(err)
	}
	if layout.senderBottom <= letterFactsTop {
		t.Fatalf("the letterhead ends at %.1f; the test needs one below the facts top", layout.senderBottom)
	}
	if layout.headBottom <= layout.senderBottom+letterFactsGap {
		t.Fatalf("the facts end at %.1f, inside the letterhead ending at %.1f", layout.headBottom, layout.senderBottom)
	}
}

// A long value of a table, such as an emergency number with a note, wraps
// in its column instead of running over the label or off the page.
func TestLetterTableWrapsALongValue(t *testing.T) {
	t.Parallel()

	letter := sampleLetter()
	long := "0170 0000001 oder 0170 0000002, montags bis freitags zwischen 8 und 16 Uhr, sonst über die Zentrale"
	letter.Blocks = []listrecords.LetterBlock{{Facts: []listrecords.LetterFact{{Label: "OGS Am Lindenhof", Value: long}}}}
	_, trace := traceLetter(t, letter)
	var parts []string
	for _, line := range trace {
		if strings.Contains(long, line) && line != long {
			parts = append(parts, line)
		}
	}
	if len(parts) < 2 || !strings.Contains(strings.Join(parts, " "), "über die Zentrale") {
		t.Fatalf("the long value was not wrapped into its column: %q", trace)
	}
	chrome, err := newLetterChrome("x")
	if err != nil {
		t.Fatal(err)
	}
	layout, err := newLetterLayout(chrome, letter)
	if err != nil {
		t.Fatal(err)
	}
	if err := layout.setFont(styleNormal, letterBodySize); err != nil {
		t.Fatal(err)
	}
	for _, part := range parts {
		width, err := layout.measure(part)
		if err != nil {
			t.Fatal(err)
		}
		if width > layout.contentWidth()*letterValueShare+0.5 {
			t.Fatalf("%q is %.1f wide, more than its column", part, width)
		}
	}
}
