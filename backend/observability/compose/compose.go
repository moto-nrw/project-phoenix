// Package compose names the Observability stores the typed root binds into
// other owners: the PWA standalone usage counts the Delivery usage report
// records and sweeps (ADR 0053 point 2). It also builds the process logger
// the roots install.
package compose

import "github.com/moto-nrw/project-phoenix/observability/internal/adapters/pwausage"

// NewPWAStandaloneUsageRepository builds the PWA standalone usage store over
// the root database.
var NewPWAStandaloneUsageRepository = pwausage.NewPWAStandaloneUsageRepository
