package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type ManualPartialAbsenceReader interface {
	ManualPartialAbsenceDates(context.Context, int64, timezone.Date, timezone.Date) ([]timezone.Date, error)
}
