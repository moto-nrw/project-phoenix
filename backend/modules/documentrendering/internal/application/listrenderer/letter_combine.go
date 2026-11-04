package listexport

// Combining letters (#4670): the stored letters of a run in one PDF, so the
// Träger-Büro prints them in one go for the families without internet. The
// pages are imported as they are; nothing is laid out again, so the print
// shows what each family got.

import (
	"bytes"
	"errors"
	"fmt"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
	"github.com/signintech/gopdf"
)

// ErrNoLetters reports a combination without any letter.
var ErrNoLetters = errors.New("there is no letter to combine")

// combineLetterPDFs appends the pages of every letter, in order, to one A4
// document. The importer panics on a PDF it cannot read; that becomes an
// error.
func combineLetterPDFs(title string, letters [][]byte) (data []byte, err error) {
	if len(letters) == 0 {
		return nil, ErrNoLetters
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			data, err = nil, fmt.Errorf("combine letters: unreadable pdf: %v", recovered)
		}
	}()
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4})
	pdf.SetInfo(gopdf.PdfInfo{Title: norms(title)})
	for i, letter := range letters {
		if len(letter) == 0 {
			return nil, fmt.Errorf("combine letters: letter %d is empty", i+1)
		}
		if err := pdf.ImportPagesFromSource(letter, "/MediaBox"); err != nil {
			return nil, fmt.Errorf("combine letters: letter %d: %w", i+1, err)
		}
	}
	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write combined letters: %w", err)
	}
	return buf.Bytes(), nil
}

// CombineLetters returns the letters as one PDF in the order given.
func (s *RendererService) CombineLetters(letters [][]byte, title, filenameBase string) (listrecords.File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "schreiben"
	}
	data, err := combineLetterPDFs(title, letters)
	if err != nil {
		return listrecords.File{}, err
	}
	return listrecords.File{Data: data, ContentType: "application/pdf", Filename: base + ".pdf"}, nil
}
