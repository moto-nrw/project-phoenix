package devices

import "github.com/moto-nrw/project-phoenix/modules/delivery/internal/application/locales"

// LocaleCatalog is Delivery's language catalog for portal preferences.
type LocaleCatalog struct{}

func (LocaleCatalog) IsSupported(locale string) bool { return locales.IsSupported(locale) }
func (LocaleCatalog) Normalize(locale string) string { return locales.Normalize(locale) }
func (LocaleCatalog) Default() string                { return locales.Default() }
