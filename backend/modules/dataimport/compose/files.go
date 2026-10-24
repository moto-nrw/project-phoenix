package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/internal/adapters/fileformat"
)

// NewFileDecoder decodes uploaded CSV and Excel files and renders the import
// templates through the Data Import file adapter. The root hands it to the
// import routes and does not import the adapter (#4003).
func NewFileDecoder() dataimport.FileDecoder { return fileformat.Decoder{} }
