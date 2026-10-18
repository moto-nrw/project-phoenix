package dataimport

import (
	"fmt"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OpeningBalanceImportFactory builds a request-scoped import service: the
// Stichtag, note, and acting staff member come from the upload form, so the
// config cannot be a shared singleton like the student/staff ones.
type OpeningBalanceImportFactory func(effectiveDate timezone.Date, note string, decidedByStaffID int64) RowImporter[OpeningBalanceImportRow]

func (f OpeningBalanceImportFactory) ForUpload(raw, note string, staffID int64) (RowImporter[OpeningBalanceImportRow], error) {
	date, err := timezone.ParseDate(raw)
	if err != nil {
		return nil, err
	}
	return f(date, note, staffID), nil
}

func (OpeningBalanceImportFactory) ValidateDate(raw string) (string, error) {
	date, err := timezone.ParseDate(raw)
	if err != nil {
		return "", fmt.Errorf("ungültiger Stichtag (erwartet JJJJ-MM-TT)")
	}
	today := timezone.TodayDate()
	if !date.Before(today) {
		return "", fmt.Errorf("stichtag muss vor dem heutigen Tag liegen")
	}
	if date.Year() != today.Year() {
		return "", fmt.Errorf("stichtag muss im aktuellen Urlaubsjahr liegen")
	}
	return date.String(), nil
}

// ImportObservation is emitted once per import run: the row counters of the
// batch and how long the run took. It carries no personal data.
type ImportObservation struct {
	Entity           string
	DryRun           bool
	Rows             int
	Accepted         int
	Rejected         int
	Created          int
	Updated          int
	Duration         time.Duration
	BatchesCommitted int
	BatchesRetried   int
	CheckpointLag    int
	Deadlocks        int
	PoolWait         time.Duration
	LockWait         time.Duration
	Commands         []CommandObservation
}

// CommandObservation describes one owner call, including attempts rolled back
// by a retry. Labels name capabilities, never uploaded data or identifiers.
type CommandObservation struct {
	Owner     string
	Operation string
	Duration  time.Duration
	Failed    bool
}
