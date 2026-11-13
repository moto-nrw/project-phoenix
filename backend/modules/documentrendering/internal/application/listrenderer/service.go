package listexport

import (
	"fmt"
	"strings"
	"unicode"

	listrecords "github.com/moto-ogs/project-phoenix/modules/documentrendering/records"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// RendererService renders list/record documents to PDF, XLSX, DOCX, and CSV files.
type RendererService struct{}

// NewService creates the document renderer.
func NewService() *RendererService {
	return &RendererService{}
}

func (s *RendererService) Render(doc listrecords.Document, format listrecords.Format, filenameBase string) (listrecords.File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "export"
	}
	switch format {
	case listrecords.FormatPDF:
		data, err := renderPDFDesigned(doc)
		if err != nil {
			return listrecords.File{}, err
		}
		return listrecords.File{Data: data, ContentType: "application/pdf", Filename: base + ".pdf"}, nil
	case listrecords.FormatDOCX:
		data, err := renderDOCX(doc)
		if err != nil {
			return listrecords.File{}, err
		}
		return listrecords.File{Data: data, ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Filename: base + ".docx"}, nil
	case listrecords.FormatXLSX:
		data, err := renderXLSX(doc)
		if err != nil {
			return listrecords.File{}, err
		}
		return listrecords.File{Data: data, ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Filename: base + ".xlsx"}, nil
	case listrecords.FormatCSV:
		data, err := renderCSV(doc)
		if err != nil {
			return listrecords.File{}, err
		}
		return listrecords.File{Data: data, ContentType: "text/csv; charset=utf-8", Filename: base + ".csv"}, nil
	default:
		return listrecords.File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

// RenderLetter renders one letter in the layout of its sender as a PDF
// (#4669).
func (s *RendererService) RenderLetter(letter listrecords.Letter, filenameBase string) (listrecords.File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "schreiben"
	}
	data, err := renderLetterPDF(letter)
	if err != nil {
		return listrecords.File{}, err
	}
	return listrecords.File{Data: data, ContentType: "application/pdf", Filename: base + ".pdf"}, nil
}

// safeFilename turns a title into a file name: lower case, letters and
// digits of any script, everything else one dash. Accents stay (#5791); the
// HTTP edge adds the ASCII spelling for clients that need one.
func safeFilename(input string) string {
	input = strings.ToLower(textnorm.Name(input))
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		// A combining mark NFC could not compose stays with the letter before
		// it; after a dash or at the start it has nothing to belong to.
		keepsMark := unicode.Is(unicode.Mn, r) && !lastDash && b.Len() > 0
		if unicode.IsLetter(r) || unicode.IsDigit(r) || keepsMark {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
