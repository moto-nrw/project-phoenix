package listexport

import (
	"errors"
	"strings"
	"testing"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

func renderSampleLetter(t *testing.T, letter listrecords.Letter) []byte {
	t.Helper()
	data, err := renderLetterPDF(letter)
	if err != nil {
		t.Fatalf("renderLetterPDF: %v", err)
	}
	return data
}

// The letters of a run print as one file: every page of every letter, in
// order, and the result is a PDF that reads again.
func TestCombineLettersKeepsEveryPageInOrder(t *testing.T) {
	t.Parallel()

	short := renderSampleLetter(t, sampleLetter())
	long := sampleLetter()
	long.Blocks = append(long.Blocks, listrecords.LetterBlock{Text: strings.Repeat("Ein langer Hinweis für die Eltern.\n", 70)})
	twoPages := renderSampleLetter(t, long)

	service := NewService()
	file, err := service.CombineLetters([][]byte{short, twoPages}, "Bestätigungen", "Bestätigungen Herbstferien")
	if err != nil {
		t.Fatalf("CombineLetters: %v", err)
	}
	if file.ContentType != "application/pdf" || file.Filename != "bestaetigungen-herbstferien.pdf" {
		t.Fatalf("file = %q %q", file.ContentType, file.Filename)
	}
	if !strings.HasPrefix(string(file.Data), "%PDF-") {
		t.Fatal("the combination is no PDF")
	}
	if pages := countRenderedPDFPages(file.Data); pages != 3 {
		t.Fatalf("pages = %d, want 3", pages)
	}
	again, err := service.CombineLetters([][]byte{file.Data, short}, "", "")
	if err != nil {
		t.Fatalf("the combination does not read again: %v", err)
	}
	if pages := countRenderedPDFPages(again.Data); pages != 4 {
		t.Fatalf("pages = %d, want 4", pages)
	}
}

func TestCombineLettersRefusesNothingAndUnreadableInput(t *testing.T) {
	t.Parallel()

	service := NewService()
	if _, err := service.CombineLetters(nil, "", "x"); !errors.Is(err, ErrNoLetters) {
		t.Fatalf("no letters: err = %v, want ErrNoLetters", err)
	}
	if _, err := service.CombineLetters([][]byte{[]byte("no pdf")}, "", "x"); err == nil {
		t.Fatal("an unreadable letter must be an error")
	}
	if _, err := service.CombineLetters([][]byte{{}}, "", "x"); err == nil {
		t.Fatal("an empty letter must be an error")
	}
}
