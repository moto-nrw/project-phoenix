package users

import "fmt"

const (
	BusDayMonday    = "mon"
	BusDayTuesday   = "tue"
	BusDayWednesday = "wed"
	BusDayThursday  = "thu"
	BusDayFriday    = "fri"
)

var BusDayOrder = []string{
	BusDayMonday,
	BusDayTuesday,
	BusDayWednesday,
	BusDayThursday,
	BusDayFriday,
}

var validBusDays = map[string]bool{
	BusDayMonday:    true,
	BusDayTuesday:   true,
	BusDayWednesday: true,
	BusDayThursday:  true,
	BusDayFriday:    true,
}

// BusDays stores the recurring weekdays on which a child is a Buskind.
// Missing keys are equivalent to false so older/partial payloads remain small.
type BusDays map[string]bool

func (b BusDays) Validate() error {
	for day := range b {
		if !validBusDays[day] {
			return fmt.Errorf("bus_days weekday %q must be one of mon/tue/wed/thu/fri", day)
		}
	}
	return nil
}

func (b BusDays) HasAny() bool {
	for _, day := range BusDayOrder {
		if b[day] {
			return true
		}
	}
	return false
}

func (b BusDays) Normalize() BusDays {
	out := BusDays{}
	for _, day := range BusDayOrder {
		if b[day] {
			out[day] = true
		}
	}
	return out
}

func BusDaysFromLegacyFlag(enabled bool) BusDays {
	if !enabled {
		return BusDays{}
	}
	return BusDays{
		BusDayMonday:    true,
		BusDayTuesday:   true,
		BusDayWednesday: true,
		BusDayThursday:  true,
		BusDayFriday:    true,
	}
}
