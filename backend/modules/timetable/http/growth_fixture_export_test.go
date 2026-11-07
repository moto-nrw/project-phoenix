package timetablehttp

import (
	"context"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

// GrowthFixture exposes existing package-owned fixtures to external DB tests.
// This file is compiled only into tests; it adds no production port or wiring.
type GrowthFixture struct {
	DB                                                 *bun.DB
	Context                                            context.Context
	Router                                             func(context.Context) chi.Router
	WriteRouter                                        chi.Router
	InstanceID, StudentID, StaffID, RoomID, ActivityID int64
	StudentIDs                                         []int64
	Date                                               calendar.Date
}

func NewParticipantsGrowthFixture(t *testing.T) *GrowthFixture {
	t.Helper()
	s := buildParticipantsSetup(t)
	f := &GrowthFixture{DB: s.db, Context: s.ctx, InstanceID: s.instanceID, StaffID: s.staffID}
	for _, child := range s.students {
		f.StudentIDs = append(f.StudentIDs, child.studentID)
	}
	f.Router = func(ctx context.Context) chi.Router { return participantsRouter(ctx, s.res, s.db, []string{"admin:*"}) }
	return f
}

func NewConflictsGrowthFixture(t *testing.T) *GrowthFixture {
	t.Helper()
	s := buildPlannedConflictsSetup(t)
	return &GrowthFixture{DB: s.db, Context: s.ctx, RoomID: s.roomID, Date: s.date,
		Router: func(ctx context.Context) chi.Router {
			router := plannedConflictsRouter(ctx, s.res)
			router.Get("/instances/{id}/staff-pool", s.res.getStaffPool)
			return router
		}}
}

func NewStudentDayGrowthFixture(t *testing.T) *GrowthFixture {
	t.Helper()
	s := buildStudentDaySetup(t)
	return &GrowthFixture{DB: s.db, Context: s.ctx, RoomID: s.roomID, StaffID: s.staffID, StudentID: s.studentID, ActivityID: s.activityID,
		Router: func(ctx context.Context) chi.Router { return adminRouter(t, ctx, s.res) }}
}

func NewHistoryGrowthFixture(t *testing.T) *GrowthFixture {
	t.Helper()
	s := buildDevModule(t)
	return &GrowthFixture{DB: s.db, Context: s.ctx, RoomID: s.roomID, StaffID: s.staffY, WriteRouter: devRouter(s.ctx, s.res, s.db),
		Router: func(ctx context.Context) chi.Router { scoped := *s; scoped.ctx = ctx; return historyRouter(&scoped) }}
}
