// Package postgres is the SQL side of the school search projection (ADR
// 0230). Every statement reads in the caller's tenant transaction, filters on
// its school explicitly as well as through RLS, and returns a bounded list.
// It reads only the tables policy names for the read projection and never
// writes. The statements are constants, so the architecture check resolves
// every table they read.
//
// The query text is bound as the first argument of each statement and folded
// into a contains pattern (patternJoin). bun writes bound values into the SQL
// it hands to the query log, so every statement that carries the text opens
// with protocol.RedactLiteralsInLog: the slow-query log never repeats it.
package postgres

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/moto-ogs/project-phoenix/modules/transactionruntime/protocol"
	"github.com/moto-ogs/project-phoenix/sharedkernel/collation"
	"github.com/uptrace/bun"
)

// Database resolves the tenant transaction and its school.
type Database func(context.Context) (bun.IDB, int64, error)

// Reader runs the search statements.
type Reader struct{ database Database }

// New builds the reader over the caller's tenant transaction.
func New(database Database) *Reader { return &Reader{database: database} }

// patternJoin folds the bound query text into a contains pattern q.p with its
// LIKE metacharacters escaped (#5791). Its `?` is the first argument.
const patternJoin = ` CROSS JOIN (SELECT '%' || ` + collation.UnaccentLikePatternSQL + ` || '%' AS p) AS q `

// phoneKeyOfStoredNumber is schoolsearchview.PhoneSearchKey for
// phone.phone_number: its digits without the German trunk or country prefix.
// TestPhoneSearchKeySQLMatchesGo holds both to the same cases.
const phoneKeyOfStoredNumber = `(SELECT CASE WHEN NOT k.intl OR k.d2 LIKE '0%' THEN ltrim(k.d2, '0') ELSE k.d2 END
	FROM (SELECT j.intl, CASE WHEN j.intl AND j.d1 LIKE '49%' THEN substr(j.d1, 3) ELSE j.d1 END AS d2
		FROM (SELECT i.intl, regexp_replace(i.digits, '^00', '') AS d1
			FROM (SELECT btrim(phone.phone_number) LIKE '+%' OR regexp_replace(phone.phone_number, '[^0-9]', '', 'g') LIKE '00%' AS intl,
				regexp_replace(phone.phone_number, '[^0-9]', '', 'g') AS digits) AS i) AS j) AS k)`

// liveChildJoins narrow a child profile to a child still at the school: a
// live person and a live, non-graduated membership, with its OGS group.
const liveChildJoins = `
	JOIN users.persons AS person ON person.id = student.person_id AND person.tenant_id = student.tenant_id AND person.deleted_at IS NULL
	JOIN users.student_school_memberships AS membership ON membership.student_profile_id = student.id
		AND membership.tenant_id = student.tenant_id AND membership.deleted_at IS NULL AND membership.status <> 'alumnus'
	LEFT JOIN education.groups AS grp ON grp.id = membership.group_id AND grp.tenant_id = membership.tenant_id `

// The match conditions of a child: its name in either order, its class, its
// group and its parent note.
const (
	childNameMatch = `(public.f_unaccent(person.first_name || ' ' || person.last_name) ILIKE q.p ESCAPE '\'
		OR public.f_unaccent(person.last_name || ' ' || person.first_name) ILIKE q.p ESCAPE '\')`
	childClassMatch = `public.f_unaccent(membership.school_class) ILIKE q.p ESCAPE '\'`
	childGroupMatch = `public.f_unaccent(COALESCE(grp.name, '')) ILIKE q.p ESCAPE '\'`
	childNoteMatch  = `public.f_unaccent(COALESCE(student.extra_info, '')) ILIKE q.p ESCAPE '\'`
)

// childrenSQL: text, school, notes, limit. A name hit ranks before a class,
// group or note hit; within a rank the list is sorted by name.
const childrenSQL = protocol.RedactLiteralsInLog + `SELECT hit.id, hit.first_name, hit.last_name, hit.school_class, hit.group_name, hit.match_field, hit.match_text FROM (
	SELECT student.id, person.first_name, person.last_name, membership.school_class, COALESCE(grp.name, '') AS group_name,
		CASE WHEN ` + childNameMatch + ` THEN 'name' WHEN ` + childClassMatch + ` THEN 'class'
			WHEN ` + childGroupMatch + ` THEN 'group' ELSE 'parent_note' END AS match_field,
		CASE WHEN ` + childNameMatch + ` OR ` + childClassMatch + ` OR ` + childGroupMatch + ` THEN ''
			ELSE COALESCE(student.extra_info, '') END AS match_text,
		CASE WHEN ` + childNameMatch + ` THEN 0 ELSE 1 END AS rank
	FROM users.student_profiles AS student ` + liveChildJoins + patternJoin + `
	WHERE student.tenant_id = ? AND (` + childNameMatch + ` OR ` + childClassMatch + ` OR ` + childGroupMatch + `
		OR (? AND ` + childNoteMatch + `))) AS hit
	ORDER BY hit.rank, hit.last_name, hit.first_name, hit.id
	LIMIT ?`

// childrenByIDSQL: school, ids.
const childrenByIDSQL = `SELECT student.id, person.first_name, person.last_name, membership.school_class, COALESCE(grp.name, '') AS group_name
	FROM users.student_profiles AS student ` + liveChildJoins + `
	WHERE student.tenant_id = ? AND student.id IN (?)`

type childRow struct {
	ID          int64  `bun:"id"`
	FirstName   string `bun:"first_name"`
	LastName    string `bun:"last_name"`
	SchoolClass string `bun:"school_class"`
	GroupName   string `bun:"group_name"`
	MatchField  string `bun:"match_field"`
	MatchText   string `bun:"match_text"`
}

func childValues(rows []childRow) []schoolsearchview.Child {
	out := make([]schoolsearchview.Child, 0, len(rows))
	for _, row := range rows {
		child := schoolsearchview.Child{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, GroupName: row.GroupName}
		if row.MatchField != "" {
			child.Match = schoolsearchview.Match{Field: schoolsearchview.MatchField(row.MatchField), Text: row.MatchText}
		}
		out = append(out, child)
	}
	return out
}

// Children finds children by name, class or group, and with notes also by the
// parent note.
func (r *Reader) Children(ctx context.Context, text string, notes bool, limit int) ([]schoolsearchview.Child, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []childRow
	if err := db.NewRaw(childrenSQL, text, schoolID, notes, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school search view: children: %w", err)
	}
	return childValues(rows), nil
}

// ChildrenByID reads the given children that are still at the school.
func (r *Reader) ChildrenByID(ctx context.Context, ids []int64) ([]schoolsearchview.Child, error) {
	if len(ids) == 0 {
		return []schoolsearchview.Child{}, nil
	}
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []childRow
	if err := db.NewRaw(childrenByIDSQL, schoolID, bun.List(ids)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school search view: children by id: %w", err)
	}
	return childValues(rows), nil
}

// contactHasLiveChild keeps contact persons of at least one child still at
// the school: the hit leads to that child's record.
const contactHasLiveChild = `EXISTS (SELECT 1 FROM users.student_guardian_relationships AS rel
	JOIN users.student_profiles AS student ON student.id = rel.student_id AND student.tenant_id = rel.tenant_id ` + liveChildJoins + `
	WHERE rel.guardian_profile_id = guardian.id AND rel.tenant_id = guardian.tenant_id)`

// contactsByPhoneSQL: phone key, school, limit. It matches the key as a part
// of each stored number's key and reports the number that matched, the
// primary one first.
const contactsByPhoneSQL = protocol.RedactLiteralsInLog + `SELECT * FROM (
		SELECT DISTINCT ON (guardian.id) guardian.id, guardian.first_name, guardian.last_name,
			'phone' AS match_field, phone.phone_number AS match_text
		FROM users.guardian_phone_numbers AS phone
		JOIN users.guardian_profiles AS guardian ON guardian.id = phone.guardian_profile_id AND guardian.tenant_id = phone.tenant_id
		CROSS JOIN (SELECT CAST(? AS text) AS key) AS q
		WHERE phone.tenant_id = ? AND strpos(` + phoneKeyOfStoredNumber + `, q.key) > 0 AND ` + contactHasLiveChild + `
		ORDER BY guardian.id, phone.is_primary DESC, phone.priority, phone.id) AS hit
	ORDER BY hit.last_name, hit.first_name, hit.id
	LIMIT ?`

const (
	contactNameMatch = `(public.f_unaccent(guardian.first_name || ' ' || guardian.last_name) ILIKE q.p ESCAPE '\'
		OR public.f_unaccent(guardian.last_name || ' ' || guardian.first_name) ILIKE q.p ESCAPE '\')`
	contactEmailMatch = `public.f_unaccent(COALESCE(guardian.email, '')) ILIKE q.p ESCAPE '\'`
)

// contactsByTextSQL: text, school, limit. It matches the name in either order
// or the e-mail address; name hits first.
const contactsByTextSQL = protocol.RedactLiteralsInLog + `SELECT guardian.id, guardian.first_name, guardian.last_name,
		CASE WHEN ` + contactNameMatch + ` THEN 'contact' ELSE 'email' END AS match_field,
		CASE WHEN ` + contactNameMatch + ` THEN '' ELSE guardian.email END AS match_text
	FROM users.guardian_profiles AS guardian ` + patternJoin + `
	WHERE guardian.tenant_id = ? AND (` + contactNameMatch + ` OR ` + contactEmailMatch + `) AND ` + contactHasLiveChild + `
	ORDER BY CASE WHEN ` + contactNameMatch + ` THEN 0 ELSE 1 END, guardian.last_name, guardian.first_name, guardian.id
	LIMIT ?`

// contactChildrenSQL: school, contact ids.
const contactChildrenSQL = `SELECT rel.guardian_profile_id, student.id, person.first_name, person.last_name, membership.school_class, rel.relationship_type
	FROM users.student_guardian_relationships AS rel
	JOIN users.student_profiles AS student ON student.id = rel.student_id AND student.tenant_id = rel.tenant_id ` + liveChildJoins + `
	WHERE rel.tenant_id = ? AND rel.guardian_profile_id IN (?)
	ORDER BY person.last_name, person.first_name, student.id`

type contactRow struct {
	ID         int64  `bun:"id"`
	FirstName  string `bun:"first_name"`
	LastName   string `bun:"last_name"`
	MatchField string `bun:"match_field"`
	MatchText  string `bun:"match_text"`
}

type contactChildRow struct {
	GuardianID       int64  `bun:"guardian_profile_id"`
	ID               int64  `bun:"id"`
	FirstName        string `bun:"first_name"`
	LastName         string `bun:"last_name"`
	SchoolClass      string `bun:"school_class"`
	RelationshipType string `bun:"relationship_type"`
}

// Contacts finds contact persons by phone key, or by name and e-mail, each
// with every child still at the school: two statements for the whole list.
func (r *Reader) Contacts(ctx context.Context, text, phoneKey string, limit int) ([]schoolsearchview.Contact, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []contactRow
	if phoneKey != "" {
		err = db.NewRaw(contactsByPhoneSQL, phoneKey, schoolID, limit).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(contactsByTextSQL, text, schoolID, limit).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, fmt.Errorf("school search view: contacts: %w", err)
	}
	contacts := make([]schoolsearchview.Contact, 0, len(rows))
	index := make(map[int64]int, len(rows))
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		index[row.ID] = len(contacts)
		ids = append(ids, row.ID)
		contacts = append(contacts, schoolsearchview.Contact{
			ID: row.ID, FirstName: row.FirstName, LastName: row.LastName,
			Match:    schoolsearchview.Match{Field: schoolsearchview.MatchField(row.MatchField), Text: row.MatchText},
			Children: []schoolsearchview.ContactChild{},
		})
	}
	if len(ids) == 0 {
		return contacts, nil
	}
	var children []contactChildRow
	if err := db.NewRaw(contactChildrenSQL, schoolID, bun.List(ids)).Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("school search view: contact children: %w", err)
	}
	for _, child := range children {
		i := index[child.GuardianID]
		contacts[i].Children = append(contacts[i].Children, schoolsearchview.ContactChild{
			ID: child.ID, FirstName: child.FirstName, LastName: child.LastName, SchoolClass: child.SchoolClass, RelationshipType: child.RelationshipType,
		})
	}
	return contacts, nil
}

// staffSQL: text, school, limit. Current staff by name in either order.
const staffSQL = protocol.RedactLiteralsInLog + `SELECT staff.id, person.first_name, person.last_name
	FROM users.staff_school_memberships AS staff
	JOIN users.persons AS person ON person.id = staff.person_id AND person.tenant_id = staff.tenant_id AND person.deleted_at IS NULL
	` + patternJoin + `
	WHERE staff.tenant_id = ? AND staff.deleted_at IS NULL AND staff.personal_data_erased_at IS NULL
		AND (public.f_unaccent(person.first_name || ' ' || person.last_name) ILIKE q.p ESCAPE '\'
			OR public.f_unaccent(person.last_name || ' ' || person.first_name) ILIKE q.p ESCAPE '\')
	ORDER BY person.last_name, person.first_name, staff.id
	LIMIT ?`

type staffRow struct {
	ID        int64  `bun:"id"`
	FirstName string `bun:"first_name"`
	LastName  string `bun:"last_name"`
}

// Staff finds current staff members by name.
func (r *Reader) Staff(ctx context.Context, text string, limit int) ([]schoolsearchview.Staff, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []staffRow
	if err := db.NewRaw(staffSQL, text, schoolID, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school search view: staff: %w", err)
	}
	out := make([]schoolsearchview.Staff, 0, len(rows))
	for _, row := range rows {
		out = append(out, schoolsearchview.Staff(row))
	}
	return out, nil
}

// roomsSQL: text, school, toilet names, Schulhof, limit. Like the room list
// it leaves out the kiosk toilets and every other system room but the
// Schulhof, by Facilities' own names (ADR 0019).
const roomsSQL = protocol.RedactLiteralsInLog + `SELECT room.id, room.name, COALESCE(room.building, '') AS building
	FROM facilities.rooms AS room ` + patternJoin + `
	WHERE room.tenant_id = ? AND room.name NOT IN (?, ?) AND (NOT room.is_system OR room.name = ?)
		AND (public.f_unaccent(room.name) ILIKE q.p ESCAPE '\' OR public.f_unaccent(COALESCE(room.building, '')) ILIKE q.p ESCAPE '\')
	ORDER BY room.name, room.id
	LIMIT ?`

type roomRow struct {
	ID       int64  `bun:"id"`
	Name     string `bun:"name"`
	Building string `bun:"building"`
}

// Rooms finds rooms by name or building.
func (r *Reader) Rooms(ctx context.Context, text string, limit int) ([]schoolsearchview.Room, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []roomRow
	err = db.NewRaw(roomsSQL, text, schoolID, facilities.WCRoomName, facilities.WCRoomAliasName, facilities.SchulhofRoomName, limit).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("school search view: rooms: %w", err)
	}
	out := make([]schoolsearchview.Room, 0, len(rows))
	for _, row := range rows {
		out = append(out, schoolsearchview.Room(row))
	}
	return out, nil
}

// groupsSQL: text, school, limit.
const groupsSQL = protocol.RedactLiteralsInLog + `SELECT grp.id, grp.name FROM education.groups AS grp ` + patternJoin + `
	WHERE grp.tenant_id = ? AND public.f_unaccent(grp.name) ILIKE q.p ESCAPE '\'
	ORDER BY grp.name, grp.id
	LIMIT ?`

type groupRow struct {
	ID   int64  `bun:"id"`
	Name string `bun:"name"`
}

// Groups finds the OGS groups by name.
func (r *Reader) Groups(ctx context.Context, text string, limit int) ([]schoolsearchview.Group, error) {
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []groupRow
	if err := db.NewRaw(groupsSQL, text, schoolID, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school search view: groups: %w", err)
	}
	out := make([]schoolsearchview.Group, 0, len(rows))
	for _, row := range rows {
		out = append(out, schoolsearchview.Group(row))
	}
	return out, nil
}
