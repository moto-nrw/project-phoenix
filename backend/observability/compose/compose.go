// Package compose names the Observability stores the typed root binds into
// other owners: the PWA standalone usage counts the Delivery usage report
// records and sweeps (ADR 0053 point 2).
package compose

import "github.com/moto-nrw/project-phoenix/database/repositories/pwausage"

// NewPWAStandaloneUsageRepository builds the PWA standalone usage store over
// the root database.
var NewPWAStandaloneUsageRepository = pwausage.NewPWAStandaloneUsageRepository
