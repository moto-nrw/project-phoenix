package fileformat

import (
	"io"

	"github.com/xuri/excelize/v2"
)

// The upload cap bounds only the compressed file. excelize defaults to a
// 16 GB unzip limit, so a small, highly compressed workbook could exhaust the
// shared API process. These limits sit far above any real import template.
const (
	xlsxUnzipSizeLimit    = 64 << 20
	xlsxUnzipXMLSizeLimit = 16 << 20
)

// openXLSX opens an import workbook with bounded decompression. A file over
// the limit fails like any other unreadable file.
func openXLSX(reader io.Reader, opts excelize.Options) (*excelize.File, error) {
	opts.UnzipSizeLimit = xlsxUnzipSizeLimit
	opts.UnzipXMLSizeLimit = xlsxUnzipXMLSizeLimit
	return excelize.OpenReader(reader, opts)
}
