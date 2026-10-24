package postgres

import (
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

func filterRequestQueueStudentName(directory StudentDirectory, query *bun.SelectQuery, alias, search string) *bun.SelectQuery {
	return query.Where("?", bun.SafeQuery(`? IN (
  SELECT student.id FROM (?) AS student
  JOIN users.persons AS person ON person.id = student.person_id AND person.tenant_id = student.tenant_id
  WHERE student.tenant_id = ?
  AND (person.first_name || ' ' || person.last_name) ILIKE ? ESCAPE '\'
 )`, bun.Ident(alias+".student_id"), directory.AllSchoolsStudentRows(query.DB()), bun.Ident(alias+".tenant_id"), "%"+escapeILike(search)+"%"))
}
