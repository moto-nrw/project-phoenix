// Package locales serves Delivery's existing language catalog.
package locales

import "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/localization"

func IsSupported(locale string) bool { return localization.IsSupported(locale) }
func Normalize(locale string) string { return localization.NormalizeLocale(locale) }
func Default() string                { return localization.DefaultLocale() }
