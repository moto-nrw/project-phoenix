package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	shiftplansyncCompose "github.com/moto-nrw/project-phoenix/workflows/shiftplansync/compose"
	"github.com/uptrace/bun"
)

// NewSickCascadeTimetableRows binds the sick cascade's Betreuungsplan rows to
// the retained repositories and its day lock to the day-wide staffing lock
// the retained deviation writes take (#1840, #3551). The shift-plan-sync
// workflow tests compose the cascade with it too.
func NewSickCascadeTimetableRows(rows repositories.TimetableOwnerRows, db *bun.DB) shiftplansyncCompose.TimetableRows {
	return rows.SickCascadeRows(timetableplanning.SubstituteDayLock(db))
}
