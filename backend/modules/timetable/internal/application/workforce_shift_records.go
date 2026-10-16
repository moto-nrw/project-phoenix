package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The retained models/schedule Dienstplan rows are Workforce records
// (#2689). This composition serves those rows to the retained consumers that
// still speak them (the coverage probe, the staff calendar feed, the plan
// export and the legacy suites); Workforce's capability arrives through the
// WorkforceShiftRecords seam below, in this composition's own vocabulary,
// because the Timetable owner may not name Workforce. The composition root
// binds the capability behind it.

// WorkforceShiftRecords is the Workforce capability the Dienstplan rows run
// over.
type WorkforceShiftRecords interface {
	CreateStaffShift(context.Context, ports.WorkforceShift) (ports.WorkforceShift, error)
	FindStaffShift(context.Context, int64) (ports.WorkforceShift, error)
	UpdateStaffShift(context.Context, ports.WorkforceShift) (ports.WorkforceShift, error)
	DeleteStaffShift(context.Context, int64) error
	ListStaffShifts(context.Context, ports.WorkforceShiftFilter) ([]ports.WorkforceShift, error)
	UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error)

	CreateShiftType(context.Context, ports.WorkforceShiftType) (ports.WorkforceShiftType, error)
	DeleteShiftType(context.Context, int64) error
	ListShiftTypes(context.Context) ([]ports.WorkforceShiftType, error)
}
