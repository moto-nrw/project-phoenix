// Package holidays computes German public holidays (gesetzliche Feiertage)
// per Bundesland. The dates are pure calendar facts (fixed days plus
// Easter-derived offsets), so they are computed locally via rickar/cal
// instead of being synced from an external API — no runtime dependency,
// no licensing question, no stale cache (#1418 3a).
//
// Regions are ISO 3166-2 codes (DE-NW, DE-BY, ...) matching the
// operations.federal_state setting. Municipality-dependent holidays that
// are not state-wide (e.g. Mariä Himmelfahrt in parts of Bayern, the
// Augsburger Friedensfest) are deliberately not included.
package holidays

import (
	"fmt"
	"sort"

	"github.com/rickar/cal/v2"
	"github.com/rickar/cal/v2/de"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// Holiday is one public holiday on a concrete calendar day.
type Holiday struct {
	Date timezone.Date `json:"date"`
	Name string        `json:"name"`
}

// DefaultRegion is the fallback Bundesland (the product's home state).
const DefaultRegion = "DE-NW"

// regionHolidays maps ISO 3166-2 codes to the state-wide holiday
// definitions. Keep in sync with the operations.federal_state options.
var regionHolidays = map[string][]*cal.Holiday{
	"DE-BW": de.HolidaysBW,
	"DE-BY": de.HolidaysBY,
	"DE-BE": de.HolidaysBE,
	"DE-BB": de.HolidaysBB,
	"DE-HB": de.HolidaysHB,
	"DE-HH": de.HolidaysHH,
	"DE-HE": de.HolidaysHE,
	"DE-MV": de.HolidaysMV,
	"DE-NI": de.HolidaysNI,
	"DE-NW": de.HolidaysNW,
	"DE-RP": de.HolidaysRP,
	"DE-SL": de.HolidaysSL,
	"DE-SN": de.HolidaysSN,
	"DE-ST": de.HolidaysST,
	"DE-SH": de.HolidaysSH,
	"DE-TH": de.HolidaysTH,
}

// Regions returns the supported region codes (sorted, for validation and
// option lists).
func Regions() []string {
	codes := make([]string, 0, len(regionHolidays))
	for code := range regionHolidays {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

// ValidRegion reports whether code is a supported Bundesland code.
func ValidRegion(code string) bool {
	_, ok := regionHolidays[code]
	return ok
}

// ForYear returns all public holidays of the region in the given year,
// sorted by date.
func ForYear(region string, year int) ([]Holiday, error) {
	defs, ok := regionHolidays[region]
	if !ok {
		return nil, fmt.Errorf("unknown federal state region %q", region)
	}
	result := make([]Holiday, 0, len(defs))
	for _, def := range defs {
		actual, _ := def.Calc(year)
		if actual.IsZero() {
			continue
		}
		y, m, d := actual.Date()
		result = append(result, Holiday{Date: timezone.NewDate(y, m, d), Name: def.Name})
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
	for year := from.Year; year <= to.Year; year++ {
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
