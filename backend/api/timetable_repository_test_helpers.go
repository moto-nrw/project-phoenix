//go:build testhooks

package api

import (
	"context"
	"fmt"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableOwnerRows are the retained rows the Timetable owner's reads run
// over (#3551), named for the suites of other owners that bind them.
type TimetableOwnerRows = timetableCompose.NativeMirrorRows

// TimetablePresenceRows are Student Presence's live sessions and
// supervisions over the Timetable owner's activities.
type TimetablePresenceRows struct {
	ActiveGroup     studentpresence.SessionRecords
	GroupSupervisor studentpresence.SupervisionRecords
}

// NewTimetablePresenceRows composes the live sessions and supervisions over
// the activities of timetable and the people in members.
func NewTimetablePresenceRows(db transactionruntime.Database, timetable owners.TimetableTestRepositories, members owners.MembershipTestRepositories, clocks ...func() time.Time) (TimetablePresenceRows, error) {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return TimetablePresenceRows{}, err
	}
	groups, err := owners.NewSchoolStructure(db)
	if err != nil {
		return TimetablePresenceRows{}, err
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return TimetablePresenceRows{}, err
	}
	sessions := owners.NewSessionRecords(db, now, nil, rooms, timetable.Timetable, membership, members.People)
	return TimetablePresenceRows{ActiveGroup: sessions, GroupSupervisor: owners.NewGroupSupervisionRecords(sessions, groups)}, nil
}

// TimetableDeviationAudit is the Audit Platform's Änderungsprotokoll the
// timetable suites read back.
type TimetableDeviationAudit struct {
	DeviationEvent auditModels.DeviationEventRepository
}

// NewTimetableDeviationAudit composes the deviation events over the test
// database, the composition that owns Audit persistence (ADR 0049).
func NewTimetableDeviationAudit(db transactionruntime.Database) TimetableDeviationAudit {
	return TimetableDeviationAudit{DeviationEvent: owners.NewTestAuditPersistence(db, nil).DeviationEvent}
}

// DeviationEventsBetween lists the Änderungsprotokoll entries with an
// occurrence in [from, to], for suites that read them back without naming the
// Audit Platform's date.
func (r TimetableDeviationAudit) DeviationEventsBetween(ctx context.Context, from, to timezone.Date) ([]*auditModels.DeviationEvent, error) {
	return r.DeviationEvent.ListByRange(ctx, auditModels.Date(from), auditModels.Date(to), nil, nil)
}

// TimetableSchoolCalendar is the School Calendar owner the timetable's
// period, closing-day and dateframe adapters delegate to.
type TimetableSchoolCalendar struct {
	schoolCalendar schoolcalendar.Calendar
}

// NewTimetableSchoolCalendar composes the School Calendar owner over db.
func NewTimetableSchoolCalendar(db transactionruntime.Database) (TimetableSchoolCalendar, error) {
	calendar, err := owners.NewSchoolCalendar(db)
	if err != nil {
		return TimetableSchoolCalendar{}, err
	}
	return TimetableSchoolCalendar{schoolCalendar: calendar}, nil
}

// MustNewTimetablePresenceRows are the live sessions and supervisions over
// the unobserved Timetable rows and People Directory rows of db.
func MustNewTimetablePresenceRows(db transactionruntime.Database, clocks ...func() time.Time) TimetablePresenceRows {
	members := owners.MustNewMembershipTestRepositories(db)
	tt, err := owners.NewTimetableTestRepositoriesWithMembers(db, members)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose timetable: %v", err))
	}
	rows, err := NewTimetablePresenceRows(db, tt, members, clocks...)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose presence: %v", err))
	}
	return rows
}

// MustNewTimetableRoomRows is NewTimetableRoomRows for suites that cannot
// handle a composition error.
func MustNewTimetableRoomRows(db transactionruntime.Database) owners.TimetableRoomRows {
	rows, err := owners.NewTimetableRoomRows(db)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose rooms: %v", err))
	}
	return rows
}

// MustNewTimetableSchoolCalendar is NewTimetableSchoolCalendar for suites
// that cannot handle a composition error.
func MustNewTimetableSchoolCalendar(db transactionruntime.Database) TimetableSchoolCalendar {
	calendar, err := NewTimetableSchoolCalendar(db)
	if err != nil {
		panic(fmt.Sprintf("timetable test rows: compose school calendar: %v", err))
	}
	return calendar
}

// SchoolCalendar returns the calendar owner.
func (c TimetableSchoolCalendar) SchoolCalendar() schoolcalendar.Calendar { return c.schoolCalendar }
