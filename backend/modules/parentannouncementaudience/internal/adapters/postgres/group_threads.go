package parentaudience

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
	"github.com/uptrace/bun"
)

// groupAudienceSQL resolves a parent group thread target (#4784) with the
// announcement rules: the same student match on the bound target row pt and
// the same reachable guardians (parent_portal.access, linked account, active
// membership). Every reached child is returned, a child without reachable
// guardians with an empty account list, so the OGS sees whom the thread
// misses.
//
// Bind order: target type, target id, target text, school, school, today,
// today, today, guardian links of the school, school.
const groupAudienceSQL = `
			WITH pt AS (SELECT ?::text AS target_type, ?::bigint AS target_ref_id, ?::text AS target_ref_text)
			SELECT s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '') AS last_name,
				COALESCE(sm.school_class, '') AS school_class,
				COALESCE(array_agg(DISTINCT gp.account_id ORDER BY gp.account_id)
					FILTER (WHERE gp.account_id IS NOT NULL), '{}') AS guardian_account_ids
			FROM pt
			JOIN users.student_profiles s ON s.tenant_id = ?` + studentMembershipJoins + ` AND (` + studentTargetMatchBound + `
			)
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
				AND sm.status <> 'alumnus'
			LEFT JOIN (
				(?) sg
				JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id AND gp.tenant_id = ?
					AND gp.account_id IS NOT NULL
				JOIN auth.account_tenants act ON act.account_id = gp.account_id
					AND act.tenant_id = gp.tenant_id AND act.status = 'active'
			) ON sg.student_id = s.id AND sg.permissions @> '{"parent_portal.access": true}'::jsonb
			GROUP BY s.id, p.first_name, p.last_name, sm.school_class
			ORDER BY LOWER(COALESCE(p.last_name, '')), LOWER(COALESCE(p.first_name, '')), s.id`

type groupAudienceRow struct {
	StudentID          int64   `bun:"student_id"`
	FirstName          string  `bun:"first_name"`
	LastName           string  `bun:"last_name"`
	SchoolClass        string  `bun:"school_class"`
	GuardianAccountIDs []int64 `bun:"guardian_account_ids,array"`
}

// GroupAudience returns every child a group thread target reaches today with
// the guardian accounts that may read the thread.
func (p *Projection) GroupAudience(ctx context.Context, schoolID int64, target contract.GroupAudienceTarget) ([]*contract.GroupAudienceChild, error) {
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	today := p.day()
	var rows []groupAudienceRow
	if err := db.NewRaw(groupAudienceSQL,
		target.Type, target.RefID, target.RefText,
		schoolID, schoolID, today, today, today,
		p.guardianLinks(db, schoolID), schoolID,
	).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent group thread audience: %w", err)
	}
	out := make([]*contract.GroupAudienceChild, 0, len(rows))
	for _, row := range rows {
		out = append(out, &contract.GroupAudienceChild{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, GuardianAccountIDs: row.GuardianAccountIDs,
		})
	}
	return out, nil
}

// groupStudentsSQL names the given children of a school, for the read
// list of a group thread: a child that has left the target since keeps its
// row there.
//
// Bind order: school, students.
const groupStudentsSQL = `
			SELECT s.id AS student_id,
				COALESCE(p.first_name, '') AS first_name,
				COALESCE(p.last_name, '') AS last_name,
				COALESCE(sm.school_class, '') AS school_class
			FROM users.student_profiles s
			LEFT JOIN users.student_school_memberships sm
				ON sm.student_profile_id = s.id AND sm.tenant_id = s.tenant_id AND sm.deleted_at IS NULL
			LEFT JOIN users.persons p ON p.id = s.person_id
			WHERE s.tenant_id = ? AND s.id IN (?)`

type groupStudentRow struct {
	StudentID   int64  `bun:"student_id"`
	FirstName   string `bun:"first_name"`
	LastName    string `bun:"last_name"`
	SchoolClass string `bun:"school_class"`
}

// GroupStudents names the given children of a school.
func (p *Projection) GroupStudents(ctx context.Context, schoolID int64, studentIDs []int64) ([]*contract.GroupStudent, error) {
	out := make([]*contract.GroupStudent, 0, len(studentIDs))
	if len(studentIDs) == 0 {
		return out, nil
	}
	db, _, err := p.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []groupStudentRow
	if err := db.NewRaw(groupStudentsSQL, schoolID, bun.List(studentIDs)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent group thread students: %w", err)
	}
	for _, row := range rows {
		out = append(out, &contract.GroupStudent{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass,
		})
	}
	return out, nil
}
