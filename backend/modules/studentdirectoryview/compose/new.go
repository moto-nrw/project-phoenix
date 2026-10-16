// Package compose builds the student directory projection for the roots,
// which bind it to the reader ports of the owners that need its rows
// (ADR 0058).
package compose

import "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"

// NewCareReader builds the reader the root binds to Care Plan.
func NewCareReader() studentdirectoryview.CareReader { return studentdirectoryview.CareReader{} }
