// Package compose builds the student directory projection for the roots,
// which bind it to the reader ports of the owners that need its rows
// (ADR 0058).
package compose

import studentdirectoryview "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/internal/adapters/postgres"

// HealthCipher seals and opens the health field of the rows (#5472); the
// roots bind Security Runtime's field cipher under HEALTH_DATA_KEYS.
type HealthCipher = studentdirectoryview.HealthCipher

// ErrStudentHealthUnreadable reports a sealed health field that does not open.
var ErrStudentHealthUnreadable = studentdirectoryview.ErrStudentHealthUnreadable

// NewReader builds the reader the root binds to Care Plan and People Directory.
// health seals and opens the children's health field; a reader without it
// refuses every non-empty health field.
func NewReader(health HealthCipher) studentdirectoryview.Reader {
	return studentdirectoryview.NewReader(health)
}

// Reader is the projection reader the roots bind.
type Reader = studentdirectoryview.Reader
