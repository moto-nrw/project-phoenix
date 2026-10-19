// Package compose builds the student directory projection for the roots,
// which bind it to the reader ports of the owners that need its rows
// (ADR 0058).
package compose

import studentdirectoryview "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/internal/adapters/postgres"

// NewReader builds the reader the root binds to Care Plan and People Directory.
func NewReader() studentdirectoryview.Reader { return studentdirectoryview.Reader{} }
