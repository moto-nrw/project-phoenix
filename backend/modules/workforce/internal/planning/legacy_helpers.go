package planning

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// The package-private helpers the moved services shared with the retained
// timetable services. modules/timetable/legacy/timetableplanning (#3218) keeps
// its own copies for the instance, deviation and materialization paths; these
// go with the services that use them (#3219).

// isoWeekday returns the ISO 8601 weekday number for d (1=Mon … 7=Sun),
// matching the storage convention of activities.schedules.weekday.
func isoWeekday(d timezone.Date) int {
	wd := d.Weekday()
	if wd == time.Sunday {
		return 7
	}
	return int(wd)
}

func marshalDeviationValue(v any) (json.RawMessage, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

// normalizeActor maps a missing/zero actor to nil so the row stores NULL
// instead of a fabricated id.
func normalizeActor(actorAccountID *int64) *int64 {
	if actorAccountID == nil || *actorAccountID <= 0 {
		return nil
	}
	return actorAccountID
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
