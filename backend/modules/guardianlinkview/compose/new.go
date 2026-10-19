// Package compose builds the guardian link projection for the roots, which
// bind it to People Directory's reader ports (ADR 0058).
package compose

import guardianlinkview "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/internal/adapters/postgres"

// NewReader builds the reader the root binds to People Directory.
func NewReader() guardianlinkview.Reader { return guardianlinkview.Reader{} }
