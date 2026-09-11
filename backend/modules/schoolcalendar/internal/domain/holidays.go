package domain

import (
	"fmt"
	"sort"
	"time"

	"github.com/wlbr/feiertage"
)

type Holiday struct {
	Date string
	Name string
}

var holidayRegions = map[string]func(int, ...bool) feiertage.Region{
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

var holidayDisplayNames = map[string]string{
	"Neujahr":                   "Neujahrstag",
	"Epiphanias":                "Heilige Drei Könige",
	"Ostern":                    "Ostersonntag",
	"Pfingsten":                 "Pfingstsonntag",
	"Tag der deutschen Einheit": "Tag der Deutschen Einheit",
	"Weihnachten":               "Erster Weihnachtsfeiertag",
}

func ValidHolidayRegion(region string) bool {
	_, ok := holidayRegions[region]
	return ok
}

func HolidaysInRange(region, from, to string) ([]Holiday, error) {
	fn, ok := holidayRegions[region]
	if !ok {
		return nil, fmt.Errorf("unknown federal state region %q", region)
	}
	fromDate, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil, fmt.Errorf("parse holiday range start: %w", err)
	}
	toDate, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil, fmt.Errorf("parse holiday range end: %w", err)
	}

	var result []Holiday
	for year := fromDate.Year(); year <= toDate.Year(); year++ {
		for _, holiday := range fn(year).Feiertage {
			y, m, d := holiday.Date()
			date := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			if date.Before(fromDate) || date.After(toDate) {
				continue
			}
			name := holiday.Text
			if display, found := holidayDisplayNames[name]; found {
				name = display
			}
			result = append(result, Holiday{Date: date.Format("2006-01-02"), Name: name})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result, nil
}

func HolidayDates(region, from, to string) (map[string]bool, error) {
	list, err := HolidaysInRange(region, from, to)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(list))
	for _, holiday := range list {
		result[holiday.Date] = true
	}
	return result, nil
}
