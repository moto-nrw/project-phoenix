package careplan

import "github.com/moto-ogs/project-phoenix/modules/careplan/carerequests"

type CareRequestDiffEntry = carerequests.DiffEntry
type CareWeekdayChange = carerequests.WeekdayChange
type CareWeeklyChange = carerequests.WeeklyChange
type CareWeeklyPlan = carerequests.WeeklyPlan

func WeeklyCareDiff(changes []CareWeekdayChange, current CareWeeklyPlan) []CareRequestDiffEntry {
	return carerequests.WeeklyDiff(changes, current)
}
