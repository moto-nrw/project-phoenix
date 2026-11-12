package application

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// pastSessionRef names a session of the batch: one loaded from the store, or
// the planned one at an index of pastPresencePlan.sessions.
type pastSessionRef struct {
	loaded  int64
	planned int
}

type pastPlannedVisit struct {
	row     *studentpresence.Visit
	session pastSessionRef
}

// pastPresencePlan holds the rows the day-by-day path would insert, in the
// order it would insert them per table.
type pastPresencePlan struct {
	attendance []*ports.Attendance
	sessions   []studentpresence.LiveGroup
	visits     []pastPlannedVisit
}

// recordPastPresenceBatch writes the days with one insert per table (#5850).
// It reports false, having written nothing, when the day-by-day path must
// run instead: a day the plan cannot accept, or a failed insert. That path
// then returns the error of the first failing day in input order. A failed
// batch is rolled back to its savepoint; only the sequence values it drew
// stay consumed (decision E3 on #5850).
func (w *PresenceHistoryWriter) recordPastPresenceBatch(ctx context.Context, days []studentpresence.PastPresenceDay, records pastPresenceRecords) (bool, error) {
	plan, ok := planPastPresence(days, records)
	if !ok {
		return false, nil
	}
	err := tenant.WithSavepoint(ctx, func(ctx context.Context) error { return w.writePastPresencePlan(ctx, plan) })
	if errors.Is(err, tenant.ErrSavepointControl) {
		return false, err
	}
	return err == nil, nil
}

func (w *PresenceHistoryWriter) writePastPresencePlan(ctx context.Context, plan pastPresencePlan) error {
	if _, err := w.store.RecordAttendances(ctx, plan.attendance); err != nil {
		return err
	}
	sessions, _, err := w.store.RecordGroups(ctx, plan.sessions)
	if err != nil {
		return err
	}
	if len(sessions) != len(plan.sessions) {
		return errors.New("student presence: recorded sessions do not match the plan")
	}
	visits := make([]*studentpresence.Visit, 0, len(plan.visits))
	for _, visit := range plan.visits {
		visit.row.ActiveGroupID = visit.session.loaded
		if visit.session.loaded == 0 {
			visit.row.ActiveGroupID = sessions[visit.session.planned].ID
		}
		visits = append(visits, visit.row)
	}
	_, err = w.store.RecordVisits(ctx, visits)
	return err
}

// pastPresencePlanner tracks what the plan already holds beside the loaded
// records, which it leaves untouched for the day-by-day path.
type pastPresencePlanner struct {
	plan       pastPresencePlan
	records    pastPresenceRecords
	attendance map[pastAttendanceKey]bool
	sessions   map[pastSessionKey]pastSessionRef
	visits     map[pastSessionRef]map[int64][]studentpresence.Visit
}

// planPastPresence applies the checks of recordPastDay to every day without
// writing.
func planPastPresence(days []studentpresence.PastPresenceDay, records pastPresenceRecords) (pastPresencePlan, bool) {
	planner := pastPresencePlanner{records: records, attendance: map[pastAttendanceKey]bool{},
		sessions: map[pastSessionKey]pastSessionRef{}, visits: map[pastSessionRef]map[int64][]studentpresence.Visit{}}
	for _, day := range days {
		planner.addAttendance(day)
		if day.Visit != nil && !planner.addVisit(day) {
			return planner.plan, false
		}
	}
	return planner.plan, true
}

func (p *pastPresencePlanner) addAttendance(day studentpresence.PastPresenceDay) {
	key := pastAttendanceKey{studentID: day.StudentID, date: day.Date}
	if !p.records.attendance[key] && !p.attendance[key] {
		p.plan.attendance = append(p.plan.attendance, pastAttendanceRow(day))
		p.attendance[key] = true
	}
}

// addVisit plans the visit of a day like recordPastVisit; it reports false
// where recordPastVisit returns an error.
func (p *pastPresencePlanner) addVisit(day studentpresence.PastPresenceDay) bool {
	ref, ok := p.plan.session(day, p.records, p.sessions)
	if !ok {
		return false
	}
	existing := p.visits[ref][day.StudentID]
	if ref.loaded != 0 {
		existing = append(p.records.visits[pastVisitKey{studentID: day.StudentID, sessionID: ref.loaded}], existing...)
	}
	if matches, found := matchingPastVisit(existing, day.Visit); found {
		return matches
	}
	row := pastVisitRow(day)
	p.plan.visits = append(p.plan.visits, pastPlannedVisit{row: row, session: ref})
	if p.visits[ref] == nil {
		p.visits[ref] = map[int64][]studentpresence.Visit{}
	}
	p.visits[ref][day.StudentID] = append(p.visits[ref][day.StudentID], *row)
	return true
}

// session resolves the session of a day's visit like recordPastVisit: a
// loaded or already planned session must carry the visit's bounds.
func (plan *pastPresencePlan) session(day studentpresence.PastPresenceDay, records pastPresenceRecords, planned map[pastSessionKey]pastSessionRef) (pastSessionRef, bool) {
	visit := day.Visit
	key := pastSessionKey{date: day.Date, activityGroupID: visit.ActivityGroupID, roomID: visit.RoomID}
	session, loaded := records.sessions[key]
	ref, ok := planned[key]
	switch {
	case loaded:
		ref = pastSessionRef{loaded: session.ID}
	case ok:
		session = plan.sessions[ref.planned]
	default:
		ref = pastSessionRef{planned: len(plan.sessions)}
		plan.sessions = append(plan.sessions, pastSessionRow(day))
		planned[key] = ref
		return ref, true
	}
	return ref, session.StartTime.Equal(visit.SessionStart) && session.EndTime != nil && session.EndTime.Equal(visit.SessionEnd)
}
