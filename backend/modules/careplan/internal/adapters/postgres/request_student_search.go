package postgres

import (
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
	"github.com/uptrace/bun"
)

func filterRequestQueueStudentName(query *bun.SelectQuery, alias, search string) *bun.SelectQuery {
	return query.Where("?", bun.SafeQuery(`? IN (
  SELECT student.id FROM (?) AS student
  JOIN users.persons AS person ON person.id = student.person_id AND person.tenant_id = student.tenant_id
  WHERE student.tenant_id = ?
  AND (person.first_name || ' ' || person.last_name) ILIKE ? ESCAPE '\'
 )`, bun.Ident(alias+".student_id"), studentdirectoryview.Query(query.DB(), 0), bun.Ident(alias+".tenant_id"), "%"+escapeILike(search)+"%"))
}
