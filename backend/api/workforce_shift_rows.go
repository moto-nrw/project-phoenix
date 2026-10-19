package api

import (
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
)

// The retained models/schedule Dienstplan rows are served by the Timetable
// composition over the Workforce capability (#2689); the bindings below hand
// it the capability in that composition's vocabulary.

// The two vocabularies spell the same order columns; a drift must not
// compile: a false comparison repeats the false key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	string(workforceModule.StaffShiftOrderDate) == string(timetablePorts.WorkforceShiftOrderDate) &&
		string(workforceModule.StaffShiftOrderStaffID) == string(timetablePorts.WorkforceShiftOrderStaffID) &&
		string(workforceModule.StaffShiftOrderStartTime) == string(timetablePorts.WorkforceShiftOrderStartTime): {},
}
