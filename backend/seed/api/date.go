package api

import (
	"fmt"
	"time"
)

const seedDateLayout = "2006-01-02"

type seedDate struct{ time.Time }

func todaySeedDate() seedDate {
	now := time.Now().In(seedBerlinLocation())
	return seedDate{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)}
}

func (d seedDate) AddDays(days int) seedDate { return seedDate{Time: d.AddDate(0, 0, days)} }
func (d seedDate) String() string            { return d.Format(seedDateLayout) }
func (d seedDate) UTCMidnight() time.Time    { return d.Time }

func seedBerlinLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		panic(fmt.Sprintf("load Europe/Berlin timezone: %v", err))
	}
	return location
}
