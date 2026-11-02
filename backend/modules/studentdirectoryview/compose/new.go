// Package compose builds the student directory projection the roots bind (ADR 0058).
package compose

import view "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/internal/adapters/postgres"

// Reader is the projection; NewReader binds the children's health cipher (#5472).
type Reader = view.Reader

func NewReader(health view.HealthCipher) Reader { return view.NewReader(health) }
