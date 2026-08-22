package education

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModel "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// ClassArrivalTime carries the Unterrichtsschluss of one school class per
// weekday ({"mon":"11:45",...}), the baseline the arrival projection reads
// (#2414, ADR 0005). It deliberately mirrors the shape of
// enrollment.care_offerings.pickup_times so both baselines normalize, store
// and read the same way.
//
// The class is the free-text users.students.school_class value matched via
// LOWER(BTRIM(...)), like ClassTeacher — there is no class entity. SchoolClass
// keeps the display form as entered; comparisons go through
// schoolclass.Normalize.
//
// The row carries no school year. After a grade transition it keeps applying
// to the new cohort of the same class label, which is why the maintenance
// screen shows UpdatedAt. Upgrade path if that stops being enough: a validity
// per school year on this row (ADR 0005, "Bekannte Grenze").
type ClassArrivalTime struct {
	base.Model `bun:"schema:education,table:class_arrival_times"`
	base.TenantModel

	SchoolClass  string            `bun:"school_class,notnull" json:"school_class"`
	ArrivalTimes map[string]string `bun:"arrival_times,type:jsonb" json:"arrival_times,omitempty"`
	UpdatedBy    *int64            `bun:"updated_by,nullzero" json:"updated_by,omitempty"`
}

// Validate normalizes ArrivalTimes in place and rejects an empty class.
func (c *ClassArrivalTime) Validate() error {
	if strings.TrimSpace(c.SchoolClass) == "" {
		return errors.New("school class is required")
	}
	normalized, err := NormalizeClassArrivalTimes(c.ArrivalTimes)
	if err != nil {
		return err
	}
	c.ArrivalTimes = normalized
	return nil
}

// TimeForWeekday returns the wall-clock arrival time planned for an ISO
// weekday, if the class carries one.
func (c *ClassArrivalTime) TimeForWeekday(weekday int) (time.Time, bool) {
	if c == nil {
		return time.Time{}, false
	}
	for day, hhmm := range c.ArrivalTimes {
		if iso, ok := enrollmentModel.CanonicalDayToISOWeekday(day); ok && iso == weekday {
			parsed, err := time.Parse("15:04", hhmm)
			if err != nil {
				return time.Time{}, false
			}
			return timezone.WallClock(parsed), true
		}
	}
	return time.Time{}, false
}

// PlannedWeekdays lists the ISO weekdays the class carries a time for, sorted.
// Without a booking mode this is also the set of days a child of the class is
// expected (ADR 0005).
func (c *ClassArrivalTime) PlannedWeekdays() []int {
	if c == nil {
		return nil
	}
	out := make([]int, 0, len(c.ArrivalTimes))
	for day := range c.ArrivalTimes {
		if iso, ok := enrollmentModel.CanonicalDayToISOWeekday(day); ok {
			out = append(out, iso)
		}
	}
	slices.Sort(out)
	return out
}

// NormalizeClassArrivalTimes canonicalizes the weekday map: lowercased day
// codes, trimmed zero-padded HH:MM values, empty values dropped. An empty
// result becomes nil so the jsonb column stores an empty object rather than
// half-filled noise. Mirrors normalizePickupTimes, minus the available-days
// check — a class has no availability, only a timetable.
func NormalizeClassArrivalTimes(times map[string]string) (map[string]string, error) {
	if len(times) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(times))
	for day, hhmm := range times {
		key := strings.ToLower(strings.TrimSpace(day))
		value := strings.TrimSpace(hhmm)
		if value == "" {
			continue
		}
		iso, ok := enrollmentModel.CanonicalDayToISOWeekday(key)
		if !ok {
			return nil, fmt.Errorf("arrival_times key %q is not a known day abbreviation", day)
		}
		if iso > 5 {
			return nil, fmt.Errorf("arrival_times day %q must be Monday through Friday", key)
		}
		parsed, err := time.Parse("15:04", value)
		if err != nil {
			return nil, fmt.Errorf("arrival_times value for %q must be HH:MM, got %q", key, hhmm)
		}
		out[key] = parsed.Format("15:04")
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// ISOWeekdayToCanonicalDay maps Monday..Friday onto the day codes the weekday
// maps are keyed by.
func ISOWeekdayToCanonicalDay(weekday int) (string, bool) {
	for _, day := range []string{"mon", "tue", "wed", "thu", "fri"} {
		if iso, ok := enrollmentModel.CanonicalDayToISOWeekday(day); ok && iso == weekday {
			return day, true
		}
	}
	return "", false
}
