package schedule

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// dateParam formats a Berlin calendar day for PostgreSQL DATE predicates.
// Binding time.Time against DATE can route through timestamptz comparison and
// shift under the DB session timezone, so callers pair this with ?::date.
func dateParam(t time.Time) string {
	return timezone.DateOf(t).Format(time.DateOnly)
}
