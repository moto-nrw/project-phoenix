// Package settingstest scripts the tenant settings that tests drive through
// their real services. It owns the mapping from the values a school fills in
// to the registry keys they are stored under, so a consuming test names the
// configuration and not the key vocabulary.
package settingstest

import (
	"context"

	config "github.com/moto-nrw/project-phoenix/models/config"
)

// The unit values a day-valued wage type accepts.
const (
	UnitHours = config.PayrollUnitHours
	UnitDays  = config.PayrollUnitDays
)

// Values is a tenant's payroll configuration, one field per setting. An empty
// field is an unconfigured setting, which is how export tests drive the
// preflight refusals.
type Values struct {
	LohnartRegelarbeit       string
	LohnartPlusStunden       string
	LohnartAuszahlung        string
	LohnartFreizeitausgleich string
	LohnartKrank             string
	LohnartUrlaub            string
	LohnartFortbildung       string
	EinheitKrank             string
	EinheitUrlaub            string
	EinheitFortbildung       string
	DatevBeraternummer       string
	DatevMandantennummer     string
}

// Settings resolves the payroll registry keys to a fixed configuration. It
// serves the narrow settings surface the payroll status service reads.
type Settings struct{ resolved map[string]string }

// New returns the settings source for one payroll configuration.
func New(values Values) *Settings {
	return &Settings{resolved: map[string]string{
		config.KeyPayrollLohnartRegelarbeit:       values.LohnartRegelarbeit,
		config.KeyPayrollLohnartPlusStunden:       values.LohnartPlusStunden,
		config.KeyPayrollLohnartAuszahlung:        values.LohnartAuszahlung,
		config.KeyPayrollLohnartFreizeitausgleich: values.LohnartFreizeitausgleich,
		config.KeyPayrollLohnartKrank:             values.LohnartKrank,
		config.KeyPayrollLohnartUrlaub:            values.LohnartUrlaub,
		config.KeyPayrollLohnartFortbildung:       values.LohnartFortbildung,
		config.KeyPayrollEinheitKrank:             values.EinheitKrank,
		config.KeyPayrollEinheitUrlaub:            values.EinheitUrlaub,
		config.KeyPayrollEinheitFortbildung:       values.EinheitFortbildung,
		config.KeyPayrollDatevBeraternummer:       values.DatevBeraternummer,
		config.KeyPayrollDatevMandantennummer:     values.DatevMandantennummer,
	}}
}

// ResolveString answers a payroll key; every other key resolves empty, the way
// an unconfigured tenant reads.
func (s *Settings) ResolveString(_ context.Context, key string) (string, error) {
	return s.resolved[key], nil
}
