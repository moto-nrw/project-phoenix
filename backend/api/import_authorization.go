package api

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
)

// NewImportAuthorization preserves the existing authorization carrier until the immutable zero-key regression gate is retired.
func NewImportAuthorization() dataimport.Authorization { return dataImportCompose.NewAuthorization() }
