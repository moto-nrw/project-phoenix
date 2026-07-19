package schedule

// MaxTimetableReadRangeDays caps the inclusive date window accepted by the
// timetable read endpoints (/gaps, /student/{id}/week, /exception-conflicts) at
// 14 days. Longer horizons have no frontend consumer today and would enlarge
// the bulk lookups linearly with the number of dates. Single source of truth
// for what used to be three identical per-file constants.
const MaxTimetableReadRangeDays = 14
