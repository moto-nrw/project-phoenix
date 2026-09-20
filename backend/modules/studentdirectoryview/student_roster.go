package studentdirectoryview

import (
	"context"
	"fmt"
	"time"

	calendar "github.com/moto-nrw/project-phoenix/internal/timezone"
	domain "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// studentRosterRow is a child together with the identity it renders under.
// Both tables are this owner's, so the roster is one statement rather than a
// read plus a name lookup.
type studentRosterRow struct {
	studentRecordRow
	FirstName string  `bun:"first_name"`
	LastName  string  `bun:"last_name"`
	TagID     *string `bun:"tag_id"`
	AccountID *int64  `bun:"account_id"`
}

func (r studentRosterRow) toRosterEntry() domain.StudentRosterEntry {
	return domain.StudentRosterEntry{
		Record:    r.toDomain(),
		FirstName: r.FirstName,
		LastName:  r.LastName,
		TagID:     r.TagID,
		AccountID: r.AccountID,
	}
}

// selectStudentRoster is the shared shape of every roster read: the child's own
// columns plus its identity, graduates excluded, ordered by name.
//
// Graduates are soft-deleted and invisible to every staff-facing and kiosk
// roster; the statistics aggregate leaves them out too, so counting them here
// would give the child table a different population than the room table.
func (s *Projection) selectStudentRoster(
	ctx context.Context,
	operation string,
	narrow func(*bun.SelectQuery) *bun.SelectQuery,
) ([]domain.StudentRosterEntry, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	rows := []studentRosterRow{}
	query := withStudentTenant(db.NewSelect().
		TableExpr(studentSource).
		ColumnExpr(studentRecordColumns).
		ColumnExpr(`"person".first_name, "person".last_name, "person".tag_id, "person".account_id`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)

	query = narrow(query).
		Distinct().
		OrderExpr(`"person".last_name, "person".first_name, "student".id`)

	stats := OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: %s: %w", operation, err)
	}
	stats.Rows = int64(len(rows))
	result := make([]domain.StudentRosterEntry, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toRosterEntry())
	}
	return result, stats, nil
}

// currentCare keeps a child whose care has ended off the live rosters. The
// tablet list, the bracelet assignment and the calendar picker all answer
// "which children does this school care for", and from the day after the last
// care day they do not. Filtered on the enrolment interval rather than the
// status, because the status only follows once the scheduler ticks.
func currentCare(query *bun.SelectQuery, today string) *bun.SelectQuery {
	return query.Where(
		`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, studentDateParam(today))
}

// ListRosterByGroups is the live roster of the given groups.
func (s *Projection) ListRosterByGroups(
	ctx context.Context,
	groupIDs []int64,
	today string,
) ([]domain.StudentRosterEntry, OperationStats, error) {
	return s.selectStudentRoster(ctx, "list student roster by group", func(query *bun.SelectQuery) *bun.SelectQuery {
		return currentCare(query.Where(`"student".group_id IN (?)`, bun.List(groupIDs)), today)
	})
}

// ListRoster is the school's whole live roster.
func (s *Projection) ListRoster(
	ctx context.Context,
	today string,
) ([]domain.StudentRosterEntry, OperationStats, error) {
	return s.selectStudentRoster(ctx, "list student roster", func(query *bun.SelectQuery) *bun.SelectQuery {
		return currentCare(query, today)
	})
}

// ListRosterOverlapping returns the children enrolled on at least one day of
// [from, to]. Unlike the live roster it keeps children whose care has since
// ended and omits later enrolments.
//
// The membership rule is expressed as an interval overlap so one statement
// answers it for the whole window: the end must not lie before the window, the
// start must not lie after it — unless immediate activation lifts that bound
// for an active child on a window containing today — and a row with neither
// bound carries no interval at all, so an inactive status is the only
// remaining signal and means "no longer enrolled".
func (s *Projection) ListRosterOverlapping(
	ctx context.Context,
	from, to, today string,
) ([]domain.StudentRosterEntry, OperationStats, error) {
	return s.selectStudentRoster(ctx, "list student roster overlapping", func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.
			Where(`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, studentDateParam(from)).
			Where(`NOT ("student".enrolled_from IS NULL AND "student".enrolled_until IS NULL AND "student".status = ?)`,
				"inactive")

		if afterDay(today, to) {
			return query.Where(
				`("student".enrolled_from IS NULL OR "student".enrolled_from <= ?)`, studentDateParam(to))
		}
		// Today lies inside the window, so an active child whose enrolment has
		// not formally started is nonetheless enrolled on that day.
		return query.Where(
			`("student".enrolled_from IS NULL OR "student".enrolled_from <= ? OR "student".status = ?)`,
			studentDateParam(to), domain.StudentStatusActive)
	})
}

// afterDay reports whether one calendar day lies after another. Both are
// YYYY-MM-DD, which orders lexicographically, but they are parsed rather than
// compared as text so a malformed value cannot read as a valid ordering.
func afterDay(day, other string) bool {
	left, err := calendar.ParseDate(day)
	if err != nil {
		return false
	}
	right, err := calendar.ParseDate(other)
	if err != nil {
		return false
	}
	return left.After(right)
}
