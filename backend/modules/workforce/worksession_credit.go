package workforce

import "slices"

// Rounding directions of the credited duration of a work block (#6851).
const (
	// WorkTimeRoundingNearest rounds half up to the nearest interval
	// ("kaufmännisch").
	WorkTimeRoundingNearest = "nearest"
	// WorkTimeRoundingUp always rounds up to the next interval.
	WorkTimeRoundingUp = "up"
	// WorkTimeRoundingDown always rounds down to the previous interval.
	WorkTimeRoundingDown = "down"

	// MaxWorkTimeCreditMinutes bounds both the pack-up minutes and the
	// rounding interval; the database checks the same range.
	MaxWorkTimeCreditMinutes = 60
)

// WorkTimeRoundingModes lists the valid rounding directions.
var WorkTimeRoundingModes = []string{WorkTimeRoundingNearest, WorkTimeRoundingUp, WorkTimeRoundingDown}

// WorkSessionCreditRule is the credit rule a completed work block carries
// (#6851, #6852): PackUpMinutes are added once for a block ended by a regular
// checkout, then the block's duration is rounded to RoundingIntervalMinutes
// in RoundingMode. The zero value credits nothing.
type WorkSessionCreditRule struct {
	PackUpMinutes           int
	RoundingIntervalMinutes int
	RoundingMode            string
}

// IsZero reports whether the rule credits nothing.
func (r WorkSessionCreditRule) IsZero() bool {
	return r.PackUpMinutes == 0 && r.RoundingIntervalMinutes == 0
}

// Validate checks the ranges the database enforces.
func (r WorkSessionCreditRule) Validate() error {
	if r.PackUpMinutes < 0 || r.PackUpMinutes > MaxWorkTimeCreditMinutes {
		return invalidWorkSession("pack-up minutes out of range")
	}
	if r.RoundingIntervalMinutes < 0 || r.RoundingIntervalMinutes > MaxWorkTimeCreditMinutes {
		return invalidWorkSession("rounding interval out of range")
	}
	if r.RoundingMode != "" && !slices.Contains(WorkTimeRoundingModes, r.RoundingMode) {
		return invalidWorkSession("invalid rounding mode")
	}
	return nil
}
