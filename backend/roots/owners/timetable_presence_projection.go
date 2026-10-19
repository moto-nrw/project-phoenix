package owners

import (
	projection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

func NewTimetablePresenceProjection(db transactionruntime.Database) ports.PresenceProjection {
	if db.Pool() == nil {
		return nil
	}
	return projection.New().TimetableReads(timetableCompose.PresenceDatabase(db.Pool()))
}
