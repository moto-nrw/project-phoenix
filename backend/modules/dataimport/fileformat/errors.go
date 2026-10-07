package fileformat

import importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"

func rowError(row int, err error) error { return &importModels.FileRowError{Row: row, Err: err} }

func missingColumns(columns []string) error {
	return &importModels.FileMissingColumnsError{Columns: columns}
}

func noCSVDataRows() error { return &importModels.FileNoDataRowsError{Kind: "CSV"} }

func noExcelDataRows() error { return &importModels.FileNoDataRowsError{Kind: "Excel"} }
