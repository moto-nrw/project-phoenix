package api

import (
	"fmt"
	"time"
)

func parseSeedDate(value string) (seedDate, error) {
	parsed, err := time.Parse(seedDateLayout, value)
	if err != nil {
		return seedDate{}, fmt.Errorf("parse date %q: %w", value, err)
	}
	return seedDate{Time: parsed}, nil
}

func (d seedDate) Compare(other seedDate) int { return d.Time.Compare(other.Time) }
