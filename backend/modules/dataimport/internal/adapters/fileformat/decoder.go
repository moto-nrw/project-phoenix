package fileformat

import (
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

// Decoder uses a fresh parser for every upload, including concurrent requests.
type Decoder struct{}

var _ dataimport.FileDecoder = Decoder{}

func (Decoder) Students(reader io.Reader, format dataimport.FileFormat) ([]dataimport.StudentImportRow, error) {
	switch format {
	case dataimport.CSV:
		return NewCSVParser().ParseStudents(reader)
	case dataimport.XLSX:
		return NewXLSXParser().ParseStudents(reader)
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
}

func (Decoder) Staff(reader io.Reader, format dataimport.FileFormat) ([]dataimport.StaffImportRow, error) {
	switch format {
	case dataimport.CSV:
		return ParseStaffCSV(reader)
	case dataimport.XLSX:
		return ParseStaffXLSX(reader)
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
}

func (Decoder) ClassList(reader io.Reader, format dataimport.FileFormat) ([]dataimport.ClassListEntryImportRow, error) {
	switch format {
	case dataimport.CSV:
		return ParseClassListCSV(reader)
	case dataimport.XLSX:
		return ParseClassListXLSX(reader)
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
}

func (Decoder) OpeningBalances(reader io.Reader, format dataimport.FileFormat) ([]dataimport.OpeningBalanceImportRow, error) {
	switch format {
	case dataimport.CSV:
		return ParseOpeningBalanceCSV(reader)
	case dataimport.XLSX:
		return ParseOpeningBalanceXLSX(reader)
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
}
