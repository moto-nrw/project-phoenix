// Package holidays computes German public holidays (gesetzliche Feiertage)
// per Bundesland. The dates are pure calendar facts (fixed days plus
// Easter-derived offsets), so they are computed locally via wlbr/feiertage
// instead of being synced from an external API — no runtime dependency,
// no licensing question, no stale cache (#1418 3a). The library covers the
// state-specific edge cases rickar/cal missed: Frauentag in MV since 2023,
// Ostersonntag/Pfingstsonntag in Brandenburg, and Berlin's one-off
// Tag der Befreiung on 2025-05-08.
//
// Regions are ISO 3166-2 codes (DE-NW, DE-BY, ...) matching the
// operations.federal_state setting. Municipality-dependent holidays that
// are not state-wide (e.g. Mariä Himmelfahrt in parts of Bayern, the
// Augsburger Friedensfest) are deliberately not included.
package holidays

import (
	"fmt"
	"sort"

	"github.com/wlbr/feiertage"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Holiday is one public holiday on a concrete calendar day.
type Holiday struct {
	Date timezone.Date `json:"date"`
	Name string        `json:"name"`
}

// DefaultRegion is the fallback Bundesland (the product's home state).
const DefaultRegion = "DE-NW"

// regionFuncs maps ISO 3166-2 codes to the state-wide holiday functions.
// Keep in sync with the operations.federal_state options. The functions
// are called WITHOUT the optional inklSonntage argument: that default
// includes Ostersonntag/Pfingstsonntag for Brandenburg (where they are
// statutory) and adds nothing extra for any other state.
var regionFuncs = map[string]func(int, ...bool) feiertage.Region{
	"DE-BW": feiertage.BadenWürttemberg,
	"DE-BY": feiertage.Bayern,
	"DE-BE": feiertage.Berlin,
	"DE-BB": feiertage.Brandenburg,
	"DE-HB": feiertage.Bremen,
	"DE-HH": feiertage.Hamburg,
	"DE-HE": feiertage.Hessen,
	"DE-MV": feiertage.MecklenburgVorpommern,
	"DE-NI": feiertage.Niedersachsen,
	"DE-NW": feiertage.NordrheinWestfalen,
	"DE-RP": feiertage.RheinlandPfalz,
	"DE-SL": feiertage.Saarland,
	"DE-SN": feiertage.Sachsen,
	"DE-ST": feiertage.SachsenAnhalt,
	"DE-SH": feiertage.SchleswigHolstein,
	"DE-TH": feiertage.Thüringen,
}

// displayNames overrides library holiday names where the official (and
// previously shipped) spelling differs; names not listed pass through.
var displayNames = map[string]string{
	"Neujahr":                   "Neujahrstag",
	"Epiphanias":                "Heilige Drei Könige",
	"Ostern":                    "Ostersonntag",
	"Pfingsten":                 "Pfingstsonntag",
	"Tag der deutschen Einheit": "Tag der Deutschen Einheit",
	"Weihnachten":               "Erster Weihnachtsfeiertag",
}

// ValidRegion reports whether code is a supported Bundesland code.
func ValidRegion(code string) bool {
	_, ok := regionFuncs[code]
	return ok
}

// ForYear returns all public holidays of the region in the given year,
// sorted by date.
func ForYear(region string, year int) ([]Holiday, error) {
	fn, ok := regionFuncs[region]
	if !ok {
		return nil, fmt.Errorf("unknown federal state region %q", region)
	}
	days := fn(year).Feiertage
	result := make([]Holiday, 0, len(days))
	for _, f := range days {
		name := f.Text
		if display, ok := displayNames[name]; ok {
			name = display
		}
		y, m, d := f.Date()
		result = append(result, Holiday{Date: timezone.NewDate(y, m, d), Name: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date.Before(result[j].Date) })
	return result, nil
}

// InRange returns the region's public holidays within [from, to]
// (inclusive), sorted by date.
func InRange(region string, from, to timezone.Date) ([]Holiday, error) {
	if to.Before(from) {
		return nil, fmt.Errorf("invalid holiday range: to %s before from %s", to, from)
	}
	var result []Holiday
	for year := from.Year(); year <= to.Year(); year++ {
		yearHolidays, err := ForYear(region, year)
		if err != nil {
			return nil, err
		}
		for _, h := range yearHolidays {
			if h.Date.Before(from) || h.Date.After(to) {
				continue
			}
			result = append(result, h)
		}
	}
	return result, nil
}

// DateSet returns the holiday dates in [from, to] as a set for O(1)
// membership checks in Soll computations.
func DateSet(region string, from, to timezone.Date) (map[timezone.Date]bool, error) {
	list, err := InRange(region, from, to)
	if err != nil {
		return nil, err
	}
	set := make(map[timezone.Date]bool, len(list))
	for _, h := range list {
		set[h.Date] = true
	}
	return set, nil
}
