package users

import (
	"context"
	"strings"

	"github.com/uptrace/bun"

	directory "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	ports "github.com/moto-ogs/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// matchCandidateLimit bounds the candidate list: the office reviews them by
// hand, and a longer list means the name is too common to tell anyway.
const matchCandidateLimit = 20

type matchCandidateRow struct {
	ID          int64          `bun:"id"`
	FirstName   string         `bun:"first_name"`
	LastName    string         `bun:"last_name"`
	Birthday    *calendar.Date `bun:"birthday,type:date"`
	SchoolClass string         `bun:"school_class"`
	Status      string         `bun:"status"`
}

// FindEnrolledMatchCandidates returns the enrolled (active or pending)
// children of the current school with the first name, last name and
// birthday an enrollment names (#6053), compared like the existing_students
// match. The school is the one of the context and is required: a lookup
// without a school never widens to every school.
func (r *studentRepository) FindEnrolledMatchCandidates(
	ctx context.Context,
	firstName, lastName string,
	birthday calendar.Date,
) ([]directory.StudentMatchCandidate, error) {
	tenantID, err := r.runtime.RequireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.runtime.DB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []matchCandidateRow
	query := r.students.StudentModelRows(db, tenantID, &rows).
		ColumnExpr(`"student".id AS id`).
		ColumnExpr(`"person".first_name AS first_name, "person".last_name AS last_name`).
		ColumnExpr(`"person".birthday AS birthday`).
		ColumnExpr(`"student".school_class AS school_class, "student".status AS status`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".tenant_id = ?`, tenantID).
		Where(`"student".status IN (?)`, bun.List([]string{directory.StudentStatusActive, directory.StudentStatusPending})).
		Where(`"person".deleted_at IS NULL`).
		Where(`LOWER(NORMALIZE(TRIM("person".first_name), NFC) COLLATE public.german_name) = LOWER(NORMALIZE(TRIM(?), NFC) COLLATE public.german_name)`, strings.TrimSpace(firstName)).
		Where(`LOWER(NORMALIZE(TRIM("person".last_name), NFC) COLLATE public.german_name) = LOWER(NORMALIZE(TRIM(?), NFC) COLLATE public.german_name)`, strings.TrimSpace(lastName)).
		Where(`"person".birthday = ?`, birthday).
		OrderExpr(`"student".id ASC`).
		Limit(matchCandidateLimit)
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "find enrolled match candidates", Err: translateNotFound(err)}
	}
	result := make([]directory.StudentMatchCandidate, len(rows))
	for i, row := range rows {
		result[i] = directory.StudentMatchCandidate{
			StudentID: row.ID, FirstName: row.FirstName, LastName: row.LastName,
			Birthday: row.Birthday, SchoolClass: row.SchoolClass, Status: row.Status,
		}
	}
	return result, nil
}
