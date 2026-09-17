package timetableplanning

import scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"

// ScheduleError is the operation-wrapping error the timetable services share
// with the care, arrival and pickup services that stay in services/schedule
// until #3220. The alias keeps one type, so errors.As matches either name.
type ScheduleError = scheduleSvc.ScheduleError
