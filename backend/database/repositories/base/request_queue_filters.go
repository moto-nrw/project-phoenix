package base

import (
	"strings"

	"github.com/uptrace/bun"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// ApplyRequestQueueFilters narrows one change-request queue to the caller's
// page and fixes its ordering: newest first on (keysetColumn, id). Order and
// keyset belong together — a keyset position only means anything alongside the
// order it walks — so a repository gets both from this one call.
//
// alias and keysetColumn are the repository's own compile-time identifiers,
// never caller input. The queue's table must carry student_id, id and
// tenant_id, which all five request queues do.
func ApplyRequestQueueFilters(q *bun.SelectQuery, alias, keysetColumn string, f modelBase.RequestQueueFilters) *bun.SelectQuery {
	studentCol := bun.Ident(alias + ".student_id")
	instantCol := bun.Ident(alias + "." + keysetColumn)
	idCol := bun.Ident(alias + ".id")

	if f.StudentID > 0 {
		q = q.Where("? = ?", studentCol, f.StudentID)
	}
	if len(f.StudentIDs) > 0 {
		q = q.Where("? IN (?)", studentCol, bun.List(f.StudentIDs))
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		// The child's name lives two tables away. A subquery leaves the outer
		// row count untouched (a join would need a DISTINCT), and correlating
		// on tenant_id keeps the match inside the caller's tenant even on the
		// paths that run without RLS.
		q = q.Where("?", bun.SafeQuery(`? IN (
			SELECT s.id
			  FROM users.students AS s
			  JOIN users.persons AS p ON p.id = s.person_id AND p.tenant_id = s.tenant_id
			 WHERE s.tenant_id = ?
			   AND (p.first_name || ' ' || p.last_name) ILIKE ? ESCAPE '\'
		)`, studentCol, bun.Ident(alias+".tenant_id"), "%"+EscapeILike(search)+"%"))
	}
	if !f.BeforeInstant.IsZero() {
		q = q.Where("(?, ?) < (?, ?)", instantCol, idCol, f.BeforeInstant, f.BeforeID)
	}
	q = q.OrderExpr("? DESC", instantCol).OrderExpr("? DESC", idCol)
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	return q
}

// ApplyRequestUrgency narrows an open queue to one urgency phase. expression
// is repository-owned SQL and args are bound values; false selects its exact
// complement so no pending row can appear in both phases or in neither.
func ApplyRequestUrgency(
	q *bun.SelectQuery, filters modelBase.RequestQueueFilters, expression string, args ...any,
) *bun.SelectQuery {
	if filters.UrgentOnly == nil {
		return q
	}
	if *filters.UrgentOnly {
		return q.Where("?", bun.SafeQuery(expression, args...))
	}
	return q.Where("NOT (?)", bun.SafeQuery(expression, args...))
}

// EscapeILike neutralizes LIKE wildcards in free-text search input, so a "%"
// the user typed matches a literal percent instead of the whole table. Pair it
// with an explicit ESCAPE '\' clause. Backslash is escaped first so the escapes
// added for % and _ are not themselves re-escaped.
func EscapeILike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}
