// backend/database/repositories/users/birthday.go
package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// birthdayRow is the ad-hoc scan target for both birthday queries. Its Birthday
// column is a timezone.Date (a DATE column, never time.Time — see
// .claude/rules/calendar-dates.md); the registry test does not see ad-hoc
// structs, so this is the reviewer's checkpoint, not the compiler's.
type birthdayRow struct {
	ID          int64         `bun:"id"`
	GroupID     *int64        `bun:"group_id"`
	FirstName   string        `bun:"first_name"`
	LastName    string        `bun:"last_name"`
	Birthday    timezone.Date `bun:"birthday"`
	GroupName   string        `bun:"group_name"`
	SchoolClass string        `bun:"school_class"`
}

// birthdayDayFilter renders the recurring-day match as a single SQL condition.
//
// A birthday recurs annually, so the match is on (month, day) and never on the
// stored year. The comparison is a string one on `TO_CHAR(birthday, 'MM-DD')`
// rather than two EXTRACTs per day, because a query for a Monday carries three
// days (Saturday, Sunday, Monday) and an IN list over one expression stays
// readable and index-friendly against a functional index if one is ever added.
//
// 29 February is deliberately NOT folded onto 28 February here: the caller
// decides which days it wants and adds the leap-day fallback itself, so a
// list export ("everyone born in February") and the dashboard ("who has a
// birthday today") can differ without the repository guessing.
func birthdayDayFilter(days []users.MonthDay) (string, []any) {
	if len(days) == 0 {
		return "", nil
	}
	placeholders := make([]string, 0, len(days))
	args := make([]any, 0, len(days))
	for _, day := range days {
		placeholders = append(placeholders, "?")
		args = append(args, formatMonthDay(day))
	}
	return "TO_CHAR(\"person\".birthday, 'MM-DD') IN (" + strings.Join(placeholders, ", ") + ")", args
}

func formatMonthDay(day users.MonthDay) string {
	return fmt.Sprintf("%02d-%02d", int(day.Month), day.Day)
}

// FindBirthdaysOn returns every non-graduated child of the current tenant whose
// birthday falls on one of the given recurring days.
//
// Children without a stored birthday are absent by construction (the NOT NULL
// check), which is what makes the display degrade gracefully when a school has
// not maintained every date — the card simply shows fewer names instead of
// failing (#1542).
func (r *StudentRepository) FindBirthdaysOn(ctx context.Context, days []users.MonthDay) ([]users.BirthdayEntry, error) {
	if len(days) == 0 {
		return nil, nil
	}

	var rows []birthdayRow
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS id`).
		ColumnExpr(`"person".first_name AS first_name, "person".last_name AS last_name`).
		ColumnExpr(`"person".birthday AS birthday`).
		ColumnExpr(`"student".group_id AS group_id`).
		ColumnExpr(`"student".school_class AS school_class`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"person".birthday IS NOT NULL`).
		Where(`"person".deleted_at IS NULL`).
		// Graduation is the students table's soft delete: the row keeps
		// existing but must not appear on any staff-facing roster (same rule
		// as newStudentWithGroupQuery). users.students itself carries no
		// deleted_at column — status IS the marker.
		Where(`"student".status <> ?`, string(users.StudentStatusAlumnus)).
		// Same for a child whose care has ended (#2487): the birthday card is
		// a staff-facing list of the children the school currently cares for.
		Where(
			`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`,
			timezone.TodayDate(),
		)

	condition, args := birthdayDayFilter(days)
	query = query.Where(condition, args...)
	query = base.WithTenantFilter(ctx, query, "student")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find student birthdays", Err: base.TranslateNotFound(err)}
	}

	return mapBirthdayRows(rows, users.BirthdayKindStudent), nil
}

// The staff birthday projections moved to the School Membership composition
// (database/repositories/staff_membership_adapters.go): staff rows belong to
// that owner, so the staff half can no longer join users.staff here.

func mapBirthdayRows(rows []birthdayRow, kind users.BirthdayKind) []users.BirthdayEntry {
	entries := make([]users.BirthdayEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, users.BirthdayEntry{
			Kind:        kind,
			ID:          row.ID,
			FirstName:   row.FirstName,
			LastName:    row.LastName,
			Birthday:    row.Birthday,
			GroupID:     row.GroupID,
			GroupName:   row.GroupName,
			SchoolClass: row.SchoolClass,
		})
	}
	return entries
}
