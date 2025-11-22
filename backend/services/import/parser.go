package importpkg

import (
	"io"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
)

// FileParser defines the interface for parsing different file formats
type FileParser interface {
	// ParseStudents parses a file into student import rows
	ParseStudents(reader io.Reader) ([]importModels.StudentImportRow, error)

	// ValidateHeader checks if the file has required columns
	ValidateHeader(reader io.Reader) error

	// GetColumnMapping returns the detected column mapping (for debugging)
	GetColumnMapping() map[string]int
}
