// Package postgres is the SQL side of the school search projection (ADR
// 0231). It reads in the caller's tenant transaction, filters on its school
// explicitly as well as through RLS, and returns bounded lists. It reads only
// the tables policy names for the read projection and never writes. The
// statement parts are constants, so the architecture check resolves every
// table they read.
//
// One search is one statement: a round trip and a plan per kind cost as much
// as the reads themselves in a school of ordinary size. Every kind has its
// part in the statement and a switch bound from the Scope; the planner folds
// a switched-off part away, so a kind outside the Scope reads no row.
//
// The query text is bound as an argument and folded into a contains pattern
// (patternJoin). bun writes bound values into the SQL it hands to the query
// log, so the statement opens with protocol.RedactLiteralsInLog: the
// slow-query log never repeats the text or the phone key.
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
// LIKE metacharacters escaped (#5791). Every part that compares the text
// binds it through its own patternJoin: the planner then folds the pattern to
// a constant, where a shared CTE would escape it again for every row.
const patternJoin = ` CROSS JOIN (SELECT '%' || ` + collation.UnaccentLikePatternSQL + ` || '%' AS p) AS q `

// phoneKeyOfStoredNumber is the SQL reading of phone.phone_number:
// schoolsearchview.PhoneNumberKey as public.f_phone_search_key. The database
// stores its result in the generated column phone.search_digits, which the phone
// search reads, so no search computes a key per stored number.
// TestPhoneSearchKeySQLMatchesGo holds the function and the column to the Go
// reading with the same cases.
const phoneKeyOfStoredNumber = `public.f_phone_search_key(phone.phone_number)`

// liveChildJoins narrow a child profile to a child still at the school: a
// live person and a live, non-graduated membership, with its OGS group.
const liveChildJoins = `
	JOIN users.persons AS person ON person.id = student.person_id AND person.tenant_id = student.tenant_id AND person.deleted_at IS NULL
	JOIN users.student_school_memberships AS membership ON membership.student_profile_id = student.id
		AND membership.tenant_id = student.tenant_id AND membership.deleted_at IS NULL AND membership.status <> 'alumnus'
	LEFT JOIN education.groups AS grp ON grp.id = membership.group_id AND grp.tenant_id = membership.tenant_id `

// liveCTE (notes, school) is every child still at the school. The child
// hits, the contact persons (a hit needs such a child) and the children of
// the contact hits all read it, so the statement joins the four tables once.
// The parent note is read only with notes.
const liveCTE = `live AS (
	SELECT student.id, person.first_name, person.last_name, membership.school_class, COALESCE(grp.name, '') AS group_name,
		CASE WHEN ? THEN COALESCE(student.extra_info, '') ELSE '' END AS extra_info
	FROM users.student_profiles AS student ` + liveChildJoins + `
	WHERE student.tenant_id = ?)`

// The match conditions of a child: its name in either order, its class, its
// group and its parent note.
const (
	childNameMatch = `(public.f_unaccent(live.first_name || ' ' || live.last_name) ILIKE q.p ESCAPE '\'
		OR public.f_unaccent(live.last_name || ' ' || live.first_name) ILIKE q.p ESCAPE '\')`
	childClassMatch = `public.f_unaccent(live.school_class) ILIKE q.p ESCAPE '\'`
	childGroupMatch = `public.f_unaccent(live.group_name) ILIKE q.p ESCAPE '\'`
	childNoteMatch  = `public.f_unaccent(live.extra_info) ILIKE q.p ESCAPE '\'`
)

// The prefilters fold several fields of a row in one call. Folding and
// comparing is what a search costs per row, and most rows match nothing, so a
// row is first compared as one text and only a row that passes is compared
// field by field. A prefilter never hides a hit:
//
//   - A field that contains the pattern still contains it inside a longer
//     text. "first last first" holds the name in either order.
//   - unaccent and lower read the joined text like the single fields: the
//     separators (a space, a line break) are no part of an unaccent rule and
//     end a word for lower, as the end of the field does.
//   - ILIKE lowers by the collation of its text. Fields of public.german_name
//     and fields of the default collation are therefore folded apart; joined
//     under one collation, a letter could lower differently than in the
//     comparison of its own field.
//
// TestPrefiltersKeepEveryHit compares both readings on the same rows.
const (
	// childPrefilter: name and group are public.german_name, class and note
	// the default collation.
	childPrefilter = `(public.f_unaccent(live.first_name || ' ' || live.last_name || ' ' || live.first_name || E'\n' || live.group_name) ILIKE q.p ESCAPE '\'
		OR public.f_unaccent(live.school_class || E'\n' || live.extra_info) ILIKE q.p ESCAPE '\')`
	contactNamePrefilter = `public.f_unaccent(guardian.first_name || ' ' || guardian.last_name || ' ' || guardian.first_name) ILIKE q.p ESCAPE '\'`
	staffNamePrefilter   = `public.f_unaccent(person.first_name || ' ' || person.last_name || ' ' || person.first_name) ILIKE q.p ESCAPE '\'`
)

// childHitsCTE: notes, text, children, limit. child_matches computes each match condition
// once per row that passes the prefilter; child_hits names the reason. A name
// hit ranks before a class, group or note hit; within a rank the list is
// sorted by name.
const childHitsCTE = `child_matches AS MATERIALIZED (
	SELECT live.*, ` + childNameMatch + ` AS name_hit, ` + childClassMatch + ` AS class_hit,
		` + childGroupMatch + ` AS group_hit, (? AND ` + childNoteMatch + `) AS note_hit
	FROM live ` + patternJoin + `
	WHERE ? AND ` + childPrefilter + `),
child_hits AS (
	SELECT m.id, m.first_name, m.last_name, m.school_class, m.group_name,
		CASE WHEN m.name_hit THEN 'name' WHEN m.class_hit THEN 'class' WHEN m.group_hit THEN 'group' ELSE 'parent_note' END AS match_field,
		CASE WHEN m.name_hit OR m.class_hit OR m.group_hit THEN '' ELSE m.extra_info END AS match_text,
		CASE WHEN m.name_hit THEN 0 ELSE 1 END AS rank
	FROM child_matches AS m
	WHERE m.name_hit OR m.class_hit OR m.group_hit OR m.note_hit
	ORDER BY rank, m.last_name, m.first_name, m.id
	LIMIT ?)`

// childrenByIDSQL: ids, school, limit. The first children of the list that
// are still at the school, in the order of the list. The list can name every
// child of the school (an answer nearly all gave), so its place comes from
// the ordinality of the list and not from a search through it per child.
const childrenByIDSQL = `SELECT student.id, person.first_name, person.last_name, membership.school_class, COALESCE(grp.name, '') AS group_name
	FROM unnest(ARRAY[?]::bigint[]) WITH ORDINALITY AS wanted(id, place)
	JOIN users.student_profiles AS student ON student.id = wanted.id ` + liveChildJoins + `
	WHERE student.tenant_id = ?
	ORDER BY wanted.place
	LIMIT ?`

// contactHasLiveChild keeps contact persons of at least one child still at
// the school: the hit leads to that child's record.
const contactHasLiveChild = `EXISTS (SELECT 1 FROM users.student_guardian_relationships AS rel
	JOIN live ON live.id = rel.student_id
	WHERE rel.guardian_profile_id = guardian.id AND rel.tenant_id = guardian.tenant_id)`

// phoneDirectMatch: a stored key holds the query key or its alternative
// reading (schoolsearchview.PhoneSearchAlternativeKey) as a part: the whole
// number or a partial one such as the last digits.
const phoneDirectMatch = `(strpos(candidate.key, pq.key) > 0 OR (pq.alt <> '' AND strpos(candidate.key, pq.alt) > 0))`

// phoneExtensionMatch: the query is a whole stored number of at least 7 key
// digits plus an extension of 1 to 5 digits ("0221 5550001-12" and "0221
// 5550001"; schoolsearchview.MinStoredPhoneDigits and MaxExtensionDigits).
const phoneExtensionMatch = `(length(candidate.key) >= 7 AND (
		(length(pq.key) - length(candidate.key) BETWEEN 1 AND 5 AND left(pq.key, length(candidate.key)) = candidate.key)
		OR (pq.alt <> '' AND length(pq.alt) - length(candidate.key) BETWEEN 1 AND 5 AND left(pq.alt, length(candidate.key)) = candidate.key)))`

// contactsByPhoneCTE: phone key, alternative key, contacts, school, limit. It finds the
// contact persons of a child at the school by a stored number that matches
// the query directly. Only when no number matches directly does it read the
// query as a stored number with an extension, and then only as the longest
// stored number that fits: an exact number never brings a shorter neighbour
// ("0221 555000" next to "0221 5550001") along. It reports the number that
// matched, the primary one first. The key of a stored number is its generated
// column search_digits.
const contactsByPhoneCTE = `contact_hits AS (
	WITH pq AS (SELECT CAST(? AS text) AS key, CAST(? AS text) AS alt),
	matched AS (
		SELECT candidate.*, ` + phoneDirectMatch + ` AS direct
		FROM (SELECT phone.id, phone.guardian_profile_id, phone.tenant_id, phone.phone_number, phone.is_primary, phone.priority,
				phone.search_digits AS key
			FROM users.guardian_phone_numbers AS phone
			WHERE ? AND phone.tenant_id = ?) AS candidate
		CROSS JOIN pq
		WHERE ` + phoneDirectMatch + ` OR ` + phoneExtensionMatch + `),
	reachable AS (
		SELECT matched.*, guardian.first_name, guardian.last_name
		FROM matched
		JOIN users.guardian_profiles AS guardian ON guardian.id = matched.guardian_profile_id AND guardian.tenant_id = matched.tenant_id
		WHERE ` + contactHasLiveChild + `)
	SELECT hit.id, hit.first_name, hit.last_name, hit.match_field, hit.match_text, 0 AS rank FROM (
		SELECT DISTINCT ON (reachable.guardian_profile_id) reachable.guardian_profile_id AS id, reachable.first_name, reachable.last_name,
			'phone' AS match_field, reachable.phone_number AS match_text
		FROM reachable
		WHERE reachable.direct OR (NOT EXISTS (SELECT 1 FROM reachable AS other WHERE other.direct)
			AND length(reachable.key) = (SELECT max(length(other.key)) FROM reachable AS other))
		ORDER BY reachable.guardian_profile_id, reachable.direct DESC, reachable.is_primary DESC, reachable.priority, reachable.id) AS hit
	ORDER BY hit.last_name, hit.first_name, hit.id
	LIMIT ?)`

const (
	contactNameMatch = `(public.f_unaccent(guardian.first_name || ' ' || guardian.last_name) ILIKE q.p ESCAPE '\'
		OR public.f_unaccent(guardian.last_name || ' ' || guardian.first_name) ILIKE q.p ESCAPE '\')`
	contactEmailMatch = `public.f_unaccent(COALESCE(guardian.email, '')) ILIKE q.p ESCAPE '\'`
)

// contactsByTextCTE: text, contacts, school, limit. It matches the name in either order or
// the e-mail address; name hits first. contact_matches computes both
// conditions once per contact person that passes the name prefilter or has a
// matching address.
const contactsByTextCTE = `contact_matches AS MATERIALIZED (
	SELECT guardian.id, guardian.tenant_id, guardian.first_name, guardian.last_name, guardian.email,
		` + contactNameMatch + ` AS name_hit, ` + contactEmailMatch + ` AS email_hit
	FROM users.guardian_profiles AS guardian ` + patternJoin + `
	WHERE ? AND guardian.tenant_id = ? AND (` + contactNamePrefilter + ` OR ` + contactEmailMatch + `)),
contact_hits AS (
	SELECT guardian.id, guardian.first_name, guardian.last_name,
		CASE WHEN guardian.name_hit THEN 'contact' ELSE 'email' END AS match_field,
		CASE WHEN guardian.name_hit THEN '' ELSE guardian.email END AS match_text,
		CASE WHEN guardian.name_hit THEN 0 ELSE 1 END AS rank
	FROM contact_matches AS guardian
	WHERE (guardian.name_hit OR guardian.email_hit) AND ` + contactHasLiveChild + `
	ORDER BY rank, guardian.last_name, guardian.first_name, guardian.id
	LIMIT ?)`

// contactChildrenCTE: contacts, school. Every child still at the school of every
// contact hit.
const contactChildrenCTE = `contact_children AS (
	SELECT rel.guardian_profile_id, live.id, live.first_name, live.last_name, live.school_class, rel.relationship_type
	FROM users.student_guardian_relationships AS rel
	JOIN live ON live.id = rel.student_id
	WHERE ? AND rel.tenant_id = ? AND rel.guardian_profile_id IN (SELECT hit.id FROM contact_hits AS hit))`

// staffHitsCTE: text, staff, school, limit. Current staff by name in either order.
const staffHitsCTE = `staff_hits AS (
	SELECT staff.id, person.first_name, person.last_name
	FROM users.staff_school_memberships AS staff
	JOIN users.persons AS person ON person.id = staff.person_id AND person.tenant_id = staff.tenant_id AND person.deleted_at IS NULL
	` + patternJoin + `
	WHERE ? AND staff.tenant_id = ? AND staff.deleted_at IS NULL AND staff.personal_data_erased_at IS NULL
		AND ` + staffNamePrefilter + `
		AND (public.f_unaccent(person.first_name || ' ' || person.last_name) ILIKE q.p ESCAPE '\'
			OR public.f_unaccent(person.last_name || ' ' || person.first_name) ILIKE q.p ESCAPE '\')
	ORDER BY person.last_name, person.first_name, staff.id
	LIMIT ?)`

// roomHitsCTE: text, rooms, school, toilet names, Schulhof, limit. Like the room list it
// leaves out archived rooms (#6845), the kiosk toilets and every other system
// room but the Schulhof, by Facilities' own names (ADR 0019).
const roomHitsCTE = `room_hits AS (
	SELECT room.id, room.name, COALESCE(room.building, '') AS building
	FROM facilities.rooms AS room ` + patternJoin + `
	WHERE ? AND room.tenant_id = ? AND room.archived_at IS NULL AND room.name NOT IN (?, ?) AND (NOT room.is_system OR room.name = ?)
		AND (public.f_unaccent(room.name) ILIKE q.p ESCAPE '\' OR public.f_unaccent(COALESCE(room.building, '')) ILIKE q.p ESCAPE '\')
	ORDER BY room.name, room.id
	LIMIT ?)`

// groupHitsCTE: text, groups, school, limit.
const groupHitsCTE = `group_hits AS (
	SELECT grp.id, grp.name FROM education.groups AS grp ` + patternJoin + `
	WHERE ? AND grp.tenant_id = ? AND public.f_unaccent(grp.name) ILIKE q.p ESCAPE '\'
	ORDER BY grp.name, grp.id
	LIMIT ?)`

// The kinds of a result row, as the row parts below spell them out. The rows
// of one kind come in the order of its list (ord).
const (
	kindChild = iota + 1
	kindContact
	kindContactChild
	kindStaff
	kindRoom
	kindGroup
)

// The result rows of the kinds, one shape for all: the hit, the contact
// person a contact child belongs to (ref) and up to six texts. ord repeats
// the order of the kind's list, so the rows arrive sorted whatever plan joins
// the kinds.
const (
	childRowsSQL = `SELECT 1 AS kind, row_number() OVER (ORDER BY hit.rank, hit.last_name, hit.first_name, hit.id) AS ord, hit.id, 0 AS ref,
		hit.first_name AS a, hit.last_name AS b, hit.school_class AS c, hit.group_name AS d, hit.match_field AS e, hit.match_text AS f
	FROM child_hits AS hit`
	contactRowsSQL = `SELECT 2, row_number() OVER (ORDER BY hit.rank, hit.last_name, hit.first_name, hit.id), hit.id, 0,
		hit.first_name, hit.last_name, '', '', hit.match_field, hit.match_text
	FROM contact_hits AS hit`
	contactChildRowsSQL = `SELECT 3, row_number() OVER (ORDER BY child.last_name, child.first_name, child.id), child.id, child.guardian_profile_id,
		child.first_name, child.last_name, child.school_class, child.relationship_type, '', ''
	FROM contact_children AS child`
	staffRowsSQL = `SELECT 4, row_number() OVER (ORDER BY hit.last_name, hit.first_name, hit.id), hit.id, 0,
		hit.first_name, hit.last_name, '', '', '', ''
	FROM staff_hits AS hit`
	roomRowsSQL = `SELECT 5, row_number() OVER (ORDER BY hit.name, hit.id), hit.id, 0, hit.name, hit.building, '', '', '', ''
	FROM room_hits AS hit`
	groupRowsSQL = `SELECT 6, row_number() OVER (ORDER BY hit.name, hit.id), hit.id, 0, hit.name, '', '', '', '', ''
	FROM group_hits AS hit`
)

// searchRowsSQL returns the rows of every kind, sorted by kind and by the
// order of its list.
const searchRowsSQL = `
SELECT r.kind, r.ord, r.id, r.ref, r.a, r.b, r.c, r.d, r.e, r.f FROM (
` + childRowsSQL + `
UNION ALL
` + contactRowsSQL + `
UNION ALL
` + contactChildRowsSQL + `
UNION ALL
` + staffRowsSQL + `
UNION ALL
` + roomRowsSQL + `
UNION ALL
` + groupRowsSQL + `
) AS r ORDER BY r.kind, r.ord`

// The two search statements. Contact persons match a phone query by its key
// (searchByPhoneSQL) and every other query by name and e-mail address
// (searchByTextSQL); the other kinds read the text in both.
const (
	searchByTextSQL = protocol.RedactLiteralsInLog + `WITH ` + liveCTE + `,
` + childHitsCTE + `,
` + contactsByTextCTE + `,
` + contactChildrenCTE + `,
` + staffHitsCTE + `,
` + roomHitsCTE + `,
` + groupHitsCTE + searchRowsSQL
	searchByPhoneSQL = protocol.RedactLiteralsInLog + `WITH ` + liveCTE + `,
` + childHitsCTE + `,
` + contactsByPhoneCTE + `,
` + contactChildrenCTE + `,
` + staffHitsCTE + `,
` + roomHitsCTE + `,
` + groupHitsCTE + searchRowsSQL
)

// searchArguments are the arguments of a search statement in the order of
// its parts; contacts are those of its contact part.
func searchArguments(schoolID int64, text string, scope schoolsearchview.Scope, contacts []any) []any {
	args := []any{
		scope.ChildDetails, schoolID, // liveCTE
		scope.ChildDetails, text, scope.Children, schoolsearchview.ChildLimit, // childHitsCTE
	}
	args = append(args, contacts...)
	return append(args,
		scope.Contacts, schoolID, // contactChildrenCTE
		text, scope.Staff, schoolID, schoolsearchview.OtherLimit, // staffHitsCTE
		text, scope.Rooms, schoolID, facilities.WCRoomName, facilities.WCRoomAliasName, facilities.SchulhofRoomName, schoolsearchview.OtherLimit, // roomHitsCTE
		text, scope.Groups, schoolID, schoolsearchview.OtherLimit, // groupHitsCTE
	)
}

// hitRow is one result row of the search statement (childRowsSQL and its
// siblings name the columns per kind).
type hitRow struct {
	Kind int    `bun:"kind"`
	Ord  int    `bun:"ord"`
	ID   int64  `bun:"id"`
	Ref  int64  `bun:"ref"`
	A    string `bun:"a"`
	B    string `bun:"b"`
	C    string `bun:"c"`
	D    string `bun:"d"`
	E    string `bun:"e"`
	F    string `bun:"f"`
}

// Search reads every kind the scope opens in one statement: children by
// name, class or group and with ChildDetails also by the parent note; contact
// persons by the phone key (and its alternative reading) when phoneKey is not
// empty, else by name and e-mail address, each with every child still at the
// school; staff, rooms and groups by name. Every list is present and bounded
// by its limit.
func (r *Reader) Search(ctx context.Context, text, phoneKey, alternativeKey string, scope schoolsearchview.Scope) (schoolsearchview.Result, error) {
	result := schoolsearchview.Result{
		Children: []schoolsearchview.Child{}, Contacts: []schoolsearchview.Contact{},
		Staff: []schoolsearchview.Staff{}, Rooms: []schoolsearchview.Room{}, Groups: []schoolsearchview.Group{},
	}
	if !scope.Children && !scope.Contacts && !scope.Staff && !scope.Rooms && !scope.Groups {
		return result, nil
	}
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return result, err
	}
	var rows []hitRow
	if phoneKey != "" {
		contacts := []any{phoneKey, alternativeKey, scope.Contacts, schoolID, schoolsearchview.ContactLimit}
		err = db.NewRaw(searchByPhoneSQL, searchArguments(schoolID, text, scope, contacts)...).Scan(ctx, &rows)
	} else {
		contacts := []any{text, scope.Contacts, schoolID, schoolsearchview.ContactLimit}
		err = db.NewRaw(searchByTextSQL, searchArguments(schoolID, text, scope, contacts)...).Scan(ctx, &rows)
	}
	if err != nil {
		return result, fmt.Errorf("school search view: search: %w", err)
	}
	contactIndex := map[int64]int{}
	for _, row := range rows {
		switch row.Kind {
		case kindChild:
			result.Children = append(result.Children, schoolsearchview.Child{
				ID: row.ID, FirstName: row.A, LastName: row.B, SchoolClass: row.C, GroupName: row.D,
				Match: schoolsearchview.Match{Field: schoolsearchview.MatchField(row.E), Text: row.F},
			})
		case kindContact:
			contactIndex[row.ID] = len(result.Contacts)
			result.Contacts = append(result.Contacts, schoolsearchview.Contact{
				ID: row.ID, FirstName: row.A, LastName: row.B,
				Match:    schoolsearchview.Match{Field: schoolsearchview.MatchField(row.E), Text: row.F},
				Children: []schoolsearchview.ContactChild{},
			})
		case kindContactChild:
			i := contactIndex[row.Ref]
			result.Contacts[i].Children = append(result.Contacts[i].Children, schoolsearchview.ContactChild{
				ID: row.ID, FirstName: row.A, LastName: row.B, SchoolClass: row.C, RelationshipType: row.D,
			})
		case kindStaff:
			result.Staff = append(result.Staff, schoolsearchview.Staff{ID: row.ID, FirstName: row.A, LastName: row.B})
		case kindRoom:
			result.Rooms = append(result.Rooms, schoolsearchview.Room{ID: row.ID, Name: row.A, Building: row.B})
		case kindGroup:
			result.Groups = append(result.Groups, schoolsearchview.Group{ID: row.ID, Name: row.A})
		}
	}
	return result, nil
}

type childRow struct {
	ID          int64  `bun:"id"`
	FirstName   string `bun:"first_name"`
	LastName    string `bun:"last_name"`
	SchoolClass string `bun:"school_class"`
	GroupName   string `bun:"group_name"`
}

// ChildrenByID reads the first limit children of ids that are still at the
// school, in the order of ids.
func (r *Reader) ChildrenByID(ctx context.Context, ids []int64, limit int) ([]schoolsearchview.Child, error) {
	if len(ids) == 0 || limit <= 0 {
		return []schoolsearchview.Child{}, nil
	}
	db, schoolID, err := r.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []childRow
	if err := db.NewRaw(childrenByIDSQL, bun.List(ids), schoolID, limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school search view: children by id: %w", err)
	}
	out := make([]schoolsearchview.Child, 0, len(rows))
	for _, row := range rows {
		out = append(out, schoolsearchview.Child{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, GroupName: row.GroupName})
	}
	return out, nil
}
