package base

import (
	"fmt"
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
	studentCol := fmt.Sprintf(`"%s".student_id`, alias)
	instantCol := fmt.Sprintf(`"%s".%s`, alias, keysetColumn)
	idCol := fmt.Sprintf(`"%s".id`, alias)

	if f.StudentID > 0 {
		q = q.Where(studentCol+" = ?", f.StudentID)
	}
	if search := strings.TrimSpace(f.Search); search != "" {
		// The child's name lives two tables away. A subquery leaves the outer
		// row count untouched (a join would need a DISTINCT), and correlating
		// on tenant_id keeps the match inside the caller's tenant even on the
		// paths that run without RLS.
		q = q.Where(studentCol+` IN (
			SELECT s.id
			  FROM users.students AS s
			  JOIN users.persons AS p ON p.id = s.person_id AND p.tenant_id = s.tenant_id
			 WHERE s.tenant_id = "`+alias+`".tenant_id
			   AND (p.first_name || ' ' || p.last_name) ILIKE ? ESCAPE '\'
		)`, "%"+EscapeILike(search)+"%")
	}
	if !f.BeforeInstant.IsZero() {
		q = q.Where("("+instantCol+", "+idCol+") < (?, ?)", f.BeforeInstant, f.BeforeID)
	}
	q = q.OrderExpr(instantCol + " DESC").OrderExpr(idCol + " DESC")
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	return q
}

// EscapeILike neutralizes LIKE wildcards in free-text search input, so a "%"
// the user typed matches a literal percent instead of the whole table. Pair it
// with an explicit ESCAPE '\' clause. Backslash is escaped first so the escapes
// added for % and _ are not themselves re-escaped.
func EscapeILike(value string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(value)
}
