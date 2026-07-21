// backend/database/repositories/users/student_companion.go
package users

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StudentCompanionRepository implements users.StudentCompanionRepository.
//
// Every method has to look at BOTH endpoint columns, because an edge is stored
// once in normalized low/high order (see migration 1.15.209) — "the companions
// of child X" is therefore an OR over student_low_id and student_high_id, never
// a lookup on a single column.
type StudentCompanionRepository struct {
	*base.Repository[*users.StudentCompanion]
	db *bun.DB
}

// NewStudentCompanionRepository creates a new StudentCompanionRepository
func NewStudentCompanionRepository(db *bun.DB) users.StudentCompanionRepository {
	return newStudentCompanionRepository(db)
}

// newStudentCompanionRepository returns the concrete type so sibling
// repositories in this package (StudentRepository's shared departure-plan
// write path) can use methods that are not part of the model interface.
func newStudentCompanionRepository(db *bun.DB) *StudentCompanionRepository {
	repo := base.NewRepository[*users.StudentCompanion](db, "users.student_companions", "StudentCompanion")
	repo.TenantScoped = true
	return &StudentCompanionRepository{
		Repository: repo,
		db:         db,
	}
}

// ListForStudent returns every edge touching the given student, across all
// weekdays.
func (r *StudentCompanionRepository) ListForStudent(ctx context.Context, studentID int64) ([]*users.StudentCompanion, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student ID is required")
	}

	var edges []*users.StudentCompanion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID).
		Order("weekday ASC")

	query = base.WithTenantFilter(ctx, query, "student_companion")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student companions", Err: err}
	}
	return edges, nil
}

// ListLinksForStudent returns the child's companions folded per companion, with
// the companion's name joined in so the detail view can render them without a
// second round trip.
func (r *StudentCompanionRepository) ListLinksForStudent(ctx context.Context, studentID int64) ([]users.CompanionLink, error) {
	edges, err := r.ListForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}

	links := users.CompanionLinksFromEdges(studentID, edges)
	names, err := r.companionNames(ctx, companionIDsOf(links))
	if err != nil {
		return nil, err
	}
	applyCompanionNames(links, names)
	return links, nil
}

// ListLinksForStudents is the bulk form of ListLinksForStudent: the folded,
// name-carrying links of many children in two queries instead of two per child.
//
// It exists for the offline lists (student export, class roster), which render
// the "mit wem" detail for a whole school — per-child round trips there would be
// hundreds of queries for one document.
func (r *StudentCompanionRepository) ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]users.CompanionLink, error) {
	result := make(map[int64][]users.CompanionLink, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}

	var edges []*users.StudentCompanion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id IN (?) OR "student_companion".student_high_id IN (?))`,
			bun.List(studentIDs), bun.List(studentIDs)).
		Order("weekday ASC")

	query = base.WithTenantFilter(ctx, query, "student_companion")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student companions", Err: err}
	}

	// Bucket the edges by requested child in ONE pass before folding. Handing
	// the whole slice to CompanionLinksFromEdges per child would rescan it every
	// time — at the export cap (thousands of children, every edge of the school)
	// that is quadratic work for a document nobody waits minutes for.
	requested := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		requested[id] = true
	}
	byStudent := make(map[int64][]*users.StudentCompanion, len(studentIDs))
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		for _, near := range [2]int64{edge.StudentLowID, edge.StudentHighID} {
			if requested[near] {
				byStudent[near] = append(byStudent[near], edge)
			}
		}
	}

	var farEnds []int64
	for _, studentID := range studentIDs {
		links := users.CompanionLinksFromEdges(studentID, byStudent[studentID])
		if len(links) == 0 {
			continue
		}
		result[studentID] = links
		farEnds = append(farEnds, companionIDsOf(links)...)
	}

	// One name query for the far ends of every requested child.
	names, err := r.companionNames(ctx, farEnds)
	if err != nil {
		return nil, err
	}
	for _, links := range result {
		applyCompanionNames(links, names)
	}
	return result, nil
}

// companionName is the first/last name of one companion.
type companionName struct {
	FirstName string
	LastName  string
}

// companionIDsOf returns the companion ids of the given links, with duplicates.
func companionIDsOf(links []users.CompanionLink) []int64 {
	ids := make([]int64, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.CompanionStudentID)
	}
	return ids
}

// applyCompanionNames fills FirstName/LastName on the given links in place.
func applyCompanionNames(links []users.CompanionLink, names map[int64]companionName) {
	for i := range links {
		if name, ok := names[links[i].CompanionStudentID]; ok {
			links[i].FirstName = name.FirstName
			links[i].LastName = name.LastName
		}
	}
}

// companionNames resolves the names of the given children in one query.
func (r *StudentCompanionRepository) companionNames(ctx context.Context, studentIDs []int64) (map[int64]companionName, error) {
	names := make(map[int64]companionName, len(studentIDs))
	if len(studentIDs) == 0 {
		return names, nil
	}

	seen := make(map[int64]bool, len(studentIDs))
	ids := make([]int64, 0, len(studentIDs))
	for _, id := range studentIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	type nameRow struct {
		StudentID int64  `bun:"student_id"`
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	var rows []nameRow
	nameQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"person".deleted_at IS NULL`)

	nameQuery = base.WithTenantFilter(ctx, nameQuery, "student")

	if err := nameQuery.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "load companion names", Err: err}
	}

	for _, row := range rows {
		names[row.StudentID] = companionName{FirstName: row.FirstName, LastName: row.LastName}
	}
	return names, nil
}

// CompanionCountsExcluding returns, per requested student, how many DISTINCT
// other children they are linked to across all weekdays, ignoring links to
// excludeID.
//
// Two callers need exactly this number and both are about the FAR end of an
// edge, which the per-child list endpoints never look at: the cap has to hold
// for the companion too (an edge adds one to both degrees), and a removal may
// only orphan a companion that has no other link left.
func (r *StudentCompanionRepository) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, nil
	}

	var edges []*users.StudentCompanion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id IN (?) OR "student_companion".student_high_id IN (?))`,
			bun.List(studentIDs), bun.List(studentIDs))

	query = base.WithTenantFilter(ctx, query, "student_companion")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "count student companions", Err: err}
	}

	requested := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		requested[id] = true
	}

	// Fold to DISTINCT far ends first: one pair linked on five weekdays is five
	// rows but one companion, and counting rows would reject a perfectly legal
	// list.
	distinct := make(map[int64]map[int64]bool, len(studentIDs))
	for _, edge := range edges {
		for _, pair := range [2][2]int64{
			{edge.StudentLowID, edge.StudentHighID},
			{edge.StudentHighID, edge.StudentLowID},
		} {
			near, far := pair[0], pair[1]
			if !requested[near] || far == excludeID {
				continue
			}
			if distinct[near] == nil {
				distinct[near] = make(map[int64]bool)
			}
			distinct[near][far] = true
		}
	}

	for _, id := range studentIDs {
		counts[id] = len(distinct[id])
	}
	return counts, nil
}

// CompanionDaysCoveredExcluding returns, per requested student, the weekday
// keys on which they keep at least one edge to a child other than excludeID.
//
// This is the per-day counterpart of CompanionCountsExcluding, and it exists
// because the accompanied-requires-a-note invariant is checked PER WEEKDAY
// (Student.Validate): a Monday link is no answer for an accompanied Tuesday.
// Removal checks therefore need to know which days survive, not merely whether
// any companion survives at all.
func (r *StudentCompanionRepository) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	covered := make(map[int64]map[string]bool, len(studentIDs))
	if len(studentIDs) == 0 {
		return covered, nil
	}

	var edges []*users.StudentCompanion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id IN (?) OR "student_companion".student_high_id IN (?))`,
			bun.List(studentIDs), bun.List(studentIDs))

	query = base.WithTenantFilter(ctx, query, "student_companion")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student companion days", Err: err}
	}

	requested := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		requested[id] = true
	}

	for _, edge := range edges {
		day, ok := users.CompanionWeekdayKeys[edge.Weekday]
		if !ok {
			continue
		}
		for _, pair := range [2][2]int64{
			{edge.StudentLowID, edge.StudentHighID},
			{edge.StudentHighID, edge.StudentLowID},
		} {
			near, far := pair[0], pair[1]
			if !requested[near] || far == excludeID {
				continue
			}
			if covered[near] == nil {
				covered[near] = make(map[string]bool, len(users.PickupDayOrder))
			}
			covered[near][day] = true
		}
	}
	return covered, nil
}

// ReplaceForStudent makes the given edges the child's complete companion set:
// every existing edge touching the student is removed first, then the new set is
// inserted.
//
// Replace rather than diff on purpose — the child detail view submits the whole
// "läuft mit" list, and a delete+insert of at most a handful of rows keeps the
// symmetry invariant trivially correct (removing Tom from Lina's card must
// remove Lina from Tom's card, which is the same row).
//
// The same symmetry means a delete here reaches into ANOTHER child's record: an
// edge that was the far child's only "mit wem" detail leaves them with an
// accompanied departure plan and nothing to back it up. Reconciling that is the
// caller's job (services/users checkCompanionRemovals) — this method writes what
// it is told.
//
// Callers run inside the request's tenant transaction (withTx), so the delete
// and the insert commit or roll back together.
//
// This method writes what it is told, and the delete is unconditional — whether
// the caller's list is still a valid replacement for what is stored is decided
// one layer up, by the ExpectedFingerprint comparison in
// services/users.validateCompanionUpdate, which runs under the subject's row
// lock. Do NOT treat the lock alone as protection here: it orders two competing
// replacements, it does not notice that the second one is stale.
func (r *StudentCompanionRepository) ReplaceForStudent(ctx context.Context, studentID int64, edges []*users.StudentCompanion) error {
	if studentID <= 0 {
		return fmt.Errorf("student ID is required")
	}

	db := base.GetDB(ctx, r.db)
	tenantID := tenant.FromContext(ctx)

	deleteQuery := db.NewDelete().
		Model((*users.StudentCompanion)(nil)).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID)
	if tenantID > 0 {
		deleteQuery = deleteQuery.Where(`"student_companion".tenant_id = ?`, tenantID)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear student companions", Err: err}
	}

	if len(edges) == 0 {
		return nil
	}

	for _, edge := range edges {
		if edge == nil {
			return fmt.Errorf("companion edge cannot be nil")
		}
		if edge.TenantID == 0 && tenantID > 0 {
			edge.TenantID = tenantID
		}
	}

	if _, err := db.NewInsert().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "insert student companions", Err: err}
	}
	return nil
}

// DeleteEdges removes the given edge rows by id. Used by the student
// repository's departure-plan reconciliation, which drops only the edges whose
// weekday the new plan no longer allows — ReplaceForStudent would rewrite the
// surviving rows (new ids, reset created_at) for no reason.
func (r *StudentCompanionRepository) DeleteEdges(ctx context.Context, edgeIDs []int64) error {
	if len(edgeIDs) == 0 {
		return nil
	}

	deleteQuery := base.GetDB(ctx, r.db).NewDelete().
		Model((*users.StudentCompanion)(nil)).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`"student_companion".id IN (?)`, bun.List(edgeIDs))
	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		deleteQuery = deleteQuery.Where(`"student_companion".tenant_id = ?`, tenantID)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete student companion edges", Err: err}
	}
	return nil
}

// CompanionIDsForWeekday returns, per requested student, the ids of every OTHER
// child in their Laufgemeinschaft on the given weekday — the whole connected
// component, not just the direct neighbours. Students without a link are absent
// from the map.
//
// The full component is what the Kindersuche grouping needs, and it can only be
// resolved here. Edges incident on the requested students alone are not enough:
// in a chain A-B-C-D where the page shows only A and D, that would yield A-B and
// C-D and no way to see that the hidden B-C edge joins them — the page would
// render one Laufgemeinschaft as two. The recursive walk closes exactly that
// gap; the caller still only names the members it received and counts the rest.
//
// This is the bulk read behind the grouping: one query for the whole page
// instead of one per child.
//
// The walk deliberately does NOT carry the seed it started from. Doing so makes
// the recursion emit one row per (requested child, reachable member): a page
// showing R children of the SAME Laufgemeinschaft of size N materializes R*N
// rows, and the degree cap of 10 bounds only how many links one child may have,
// not how large a legal chain can grow. Instead the query walks the reachable
// nodes ONCE and returns the edges between them — at most one row per link —
// and the components are assembled here with a union-find, which is linear in
// the number of edges. The result map is the same either way.
func (r *StudentCompanionRepository) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	result := make(map[int64][]int64)
	if len(studentIDs) == 0 {
		return result, nil
	}
	if _, ok := users.CompanionWeekdayKeys[weekday]; !ok {
		return nil, fmt.Errorf("%w: got %d", users.ErrCompanionInvalidWeekday, weekday)
	}

	// One row per edge inside the touched components. UNION — not UNION ALL —
	// terminates the walk: a cycle re-derives members that are already in the
	// working set, and those are discarded.
	type edgeRow struct {
		Low  int64 `bun:"low_id"`
		High int64 `bun:"high_id"`
	}
	var rows []edgeRow

	// Defense in depth on top of RLS, mirroring base.WithTenantFilter: filter by
	// tenant when the context carries one, and leave the query alone when it does
	// not (CLI/superuser paths).
	tenantID := tenant.FromContext(ctx)

	// The reachable set is closed under the weekday's edges, so joining on the
	// low endpoint alone already collects every edge of every touched component;
	// the high endpoint of such an edge is reachable by definition.
	if err := base.GetDB(ctx, r.db).NewRaw(`
		WITH RECURSIVE reach (member) AS (
			SELECT seed.id
			FROM unnest(ARRAY[?]::bigint[]) AS seed(id)
			UNION
			SELECT CASE WHEN companion.student_low_id = reach.member
			            THEN companion.student_high_id
			            ELSE companion.student_low_id
			       END
			FROM reach
			JOIN users.student_companions AS companion
			  ON (companion.student_low_id = reach.member OR companion.student_high_id = reach.member)
			 AND companion.weekday = ?
			 AND (? = 0 OR companion.tenant_id = ?)
		)
		SELECT companion.student_low_id AS low_id, companion.student_high_id AS high_id
		FROM users.student_companions AS companion
		JOIN reach ON reach.member = companion.student_low_id
		WHERE companion.weekday = ?
		  AND (? = 0 OR companion.tenant_id = ?)
	`, bun.List(studentIDs), weekday, tenantID, tenantID, weekday, tenantID, tenantID).Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list companions for weekday", Err: err}
	}

	// Union-find over the returned edges: every node ends up under the
	// representative of its connected component, which is exactly the
	// Laufgemeinschaft the recursion used to re-derive per seed.
	parent := make(map[int64]int64, len(rows)*2)
	var find func(id int64) int64
	find = func(id int64) int64 {
		root, ok := parent[id]
		if !ok {
			parent[id] = id
			return id
		}
		if root == id {
			return id
		}
		root = find(root)
		parent[id] = root // path compression
		return root
	}
	for _, row := range rows {
		low, high := find(row.Low), find(row.High)
		if low != high {
			parent[high] = low
		}
	}

	members := make(map[int64][]int64, len(parent))
	for id := range parent {
		root := find(id)
		members[root] = append(members[root], id)
	}
	for _, group := range members {
		slices.Sort(group)
	}

	// A member outside the requested set is kept on purpose — the caller decides
	// what to do with a child that is not on the current page (it counts them
	// instead of naming them). A child without a link on this weekday never made
	// it into an edge, so it stays absent from the map.
	for _, studentID := range studentIDs {
		if _, linked := parent[studentID]; !linked {
			continue
		}
		group, ok := members[find(studentID)]
		if !ok {
			continue
		}
		companions := make([]int64, 0, len(group)-1)
		for _, member := range group {
			if member != studentID {
				companions = append(companions, member)
			}
		}
		if len(companions) > 0 {
			result[studentID] = companions
		}
	}
	return result, nil
}
