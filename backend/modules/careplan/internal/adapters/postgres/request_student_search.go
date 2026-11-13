package postgres

import (
	"github.com/moto-ogs/project-phoenix/sharedkernel/collation"
	"github.com/uptrace/bun"
)

type requestStore struct {
	*Store
	directory StudentDirectory
}

// NewRequestStore binds the request tables; the queue's name search reads
// the student directory.
func NewRequestStore(store *Store, directory StudentDirectory) *requestStore {
	if store == nil || directory == nil {
		panic("care plan request store: store and student directory are required")
	}
	return &requestStore{Store: store, directory: directory}
}

// filterRequestQueueStudentName matches the name without accents; the
// pattern escapes LIKE metacharacters after folding (#5791).
func filterRequestQueueStudentName(directory StudentDirectory, query *bun.SelectQuery, alias, search string) *bun.SelectQuery {
	return query.Where("?", bun.SafeQuery(`? IN (
  SELECT student.id FROM (?) AS student
  JOIN users.persons AS person ON person.id = student.person_id AND person.tenant_id = student.tenant_id
  WHERE student.tenant_id = ?
  AND public.f_unaccent(person.first_name || ' ' || person.last_name) ILIKE '%' || `+collation.UnaccentLikePatternSQL+` || '%' ESCAPE '\'
 )`, bun.Ident(alias+".student_id"), directory.AllSchoolsStudentRows(query.DB()), bun.Ident(alias+".tenant_id"), search))
}
