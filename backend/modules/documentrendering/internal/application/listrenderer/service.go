package listexport

import (
	"fmt"
	"strings"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
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

func safeFilename(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	replacer := strings.NewReplacer(
		"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
		" ", "-", "/", "-", "\\", "-", ":", "-", ";", "-",
	)
	input = replacer.Replace(input)
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
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
