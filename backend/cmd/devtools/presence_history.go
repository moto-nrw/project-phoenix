package main

import (
	"context"
	"os"
	"strings"

	seedapi "github.com/moto-nrw/project-phoenix/internal/devtools/seedapi"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// openPresenceHistory binds the seeder's past attendance and room visits to
// Student Presence (#5031). No route records presence after the fact, so the
// seed writes it on the database the server uses (DB_DSN), inside the
// school's tenant transaction. Without DB_DSN, as in the sidecar of the
// public demo, the seed leaves the past without presence; close is then a
// no-op.
func openPresenceHistory() (seedapi.PresenceHistoryWriter, func(), error) {
	if strings.TrimSpace(os.Getenv("DB_DSN")) == "" {
		return nil, func() {}, nil
	}
	db, err := transactionruntime.OpenDatabase()
	if err != nil {
		return nil, nil, err
	}
	history := presencecompose.NewPresenceHistory(presencecompose.Dependencies{DB: db.Pool()})
	write := func(ctx context.Context, schoolID int64, days []seedapi.PastPresenceDay) (int, error) {
		ctx = tenant.WithTenantID(tenant.WithUnitOfWork(ctx, db.UnitOfWork()), schoolID)
		return history.RecordPresenceHistory(ctx, ownerPresenceDays(days))
	}
	return write, func() { _ = db.Close() }, nil
}

func ownerPresenceDays(days []seedapi.PastPresenceDay) []presencecompose.PastPresenceDay {
	result := make([]presencecompose.PastPresenceDay, 0, len(days))
	for _, day := range days {
		owned := presencecompose.PastPresenceDay{
			StudentID: day.StudentID, Date: day.Date, DeviceID: day.DeviceID,
			CheckIn: day.CheckIn, CheckOut: day.CheckOut,
		}
		if visit := day.Visit; visit != nil {
			owned.Visit = &presencecompose.PastVisit{
				ActivityGroupID: visit.ActivityGroupID, RoomID: visit.RoomID,
				SessionStart: visit.SessionStart, SessionEnd: visit.SessionEnd,
				Entry: visit.Entry, Exit: visit.Exit,
			}
		}
		result = append(result, owned)
	}
	return result
}
