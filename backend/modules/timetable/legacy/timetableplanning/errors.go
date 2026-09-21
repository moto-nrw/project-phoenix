package timetableplanning

import "github.com/moto-nrw/project-phoenix/modules/careplan"

// ScheduleError is the operation-wrapping error the timetable services share
// with the care, arrival and pickup services in
// modules/careplan. The alias keeps one type, so
// errors.As matches either name.
type ScheduleError = careplan.ScheduleError
