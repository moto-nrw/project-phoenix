package application

import (
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"

	"github.com/wlbr/feiertage"
)

// holidayRegion is one supported federal state: its holiday calendar and
// the German name the school sees (#4775).
type holidayRegion struct {
	name     string
	holidays func(int, ...bool) feiertage.Region
}

var holidayRegions = map[string]holidayRegion{
	"DE-BW": {"Baden-Württemberg", feiertage.BadenWürttemberg},
	"DE-BY": {"Bayern", feiertage.Bayern},
	"DE-BE": {"Berlin", feiertage.Berlin},
	"DE-BB": {"Brandenburg", feiertage.Brandenburg},
	"DE-HB": {"Bremen", feiertage.Bremen},
	"DE-HH": {"Hamburg", feiertage.Hamburg},
	"DE-HE": {"Hessen", feiertage.Hessen},
	"DE-MV": {"Mecklenburg-Vorpommern", feiertage.MecklenburgVorpommern},
	"DE-NI": {"Niedersachsen", feiertage.Niedersachsen},
	"DE-NW": {"Nordrhein-Westfalen", feiertage.NordrheinWestfalen},
	"DE-RP": {"Rheinland-Pfalz", feiertage.RheinlandPfalz},
	"DE-SL": {"Saarland", feiertage.Saarland},
	"DE-SN": {"Sachsen", feiertage.Sachsen},
	"DE-ST": {"Sachsen-Anhalt", feiertage.SachsenAnhalt},
	"DE-SH": {"Schleswig-Holstein", feiertage.SchleswigHolstein},
	"DE-TH": {"Thüringen", feiertage.Thüringen},
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

// HolidayRegionName is the German name of a supported region, "" otherwise.
func HolidayRegionName(region string) string {
	return holidayRegions[region].name
}

func HolidaysInRange(region, from, to string) ([]schoolcalendar.Holiday, error) {
	entry, ok := holidayRegions[region]
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

	var result []schoolcalendar.Holiday
	for year := fromDate.Year(); year <= toDate.Year(); year++ {
		for _, holiday := range entry.holidays(year).Feiertage {
			y, m, d := holiday.Date()
			date := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
			if date.Before(fromDate) || date.After(toDate) {
				continue
			}
			name := holiday.Text
			if display, found := holidayDisplayNames[name]; found {
				name = display
			}
			result = append(result, schoolcalendar.Holiday{Date: date.Format("2006-01-02"), Name: name})
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
