package fileformat

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

func newCSVReader(reader io.Reader) (*csv.Reader, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read CSV: %w", err)
	}
	text, _, err := textnorm.DecodeCSVText(content)
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	delimiter, err := textnorm.CSVDelimiter(text, "")
	if err != nil {
		return nil, err
	}
	result := csv.NewReader(strings.NewReader(text))
	result.Comma = delimiter
	return result, nil
}
