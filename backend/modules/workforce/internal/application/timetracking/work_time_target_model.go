package timetracking

import "github.com/moto-nrw/project-phoenix/internal/timezone"

// WorkTimeTargetModel exposes the target calculation without a writable model.
type WorkTimeTargetModel struct {
	ID                 int64
	RotationAnchorDate timezone.Date
	DailyTarget        func(anchor, date timezone.Date) (int, bool)
}
