package api

import (
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

const seedDateLayout = "2006-01-02"

type seedDate struct{ time.Time }

func todaySeedDate() seedDate {
	return seedDateAt(time.Now())
}

func seedDateAt(instant time.Time) seedDate {
	now := instant.In(seedBerlinLocation())
	return seedDate{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}
}

func parseSeedDate(value string) (seedDate, error) {
	parsed, err := time.Parse(seedDateLayout, value)
	if err != nil {
		return seedDate{}, fmt.Errorf("parse date %q: %w", value, err)
	}
	return seedDate{Time: parsed}, nil
}

func (d seedDate) AddDays(days int) seedDate   { return seedDate{Time: d.AddDate(0, 0, days)} }
func (d seedDate) String() string              { return d.Format(seedDateLayout) }
func (d seedDate) UTCMidnight() time.Time      { return d.Time }
func (d seedDate) CalendarDate() calendar.Date { return calendar.Date(d.String()) }

// historyToday is the day a history of the seed ends before: the day the
// school's seed started. Steps before the history may already have recorded
// today's attendance; that happened on this day or later, so a run that
// crosses Berlin midnight never writes a past day over it (#5511).
func (rt *Runtime) historyToday() (calendar.Date, error) {
	if rt.FixedSeeder == nil || rt.FixedSeeder.seedDay.IsZero() {
		return "", fmt.Errorf("the seed day of the school is not available")
	}
	return rt.FixedSeeder.seedDay.CalendarDate(), nil
}

func seedBerlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(fmt.Sprintf("load Europe/Berlin timezone: %v", err))
	}
	return location
}

func (s seedCareExitsStep) today() seedDate {
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	return seedDateAt(now())
}
