package postgres

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

var companionDayKeys = map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri"}

type companionEdgeRow struct {
	bun.BaseModel `bun:"table:student_companions,alias:student_companion"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentLowID  int64     `bun:"student_low_id,notnull"`
	StudentHighID int64     `bun:"student_high_id,notnull"`
	Weekday       int       `bun:"weekday,notnull"`
}

type companionPair struct {
	Low  int64 `bun:"low_id"`
	High int64 `bun:"high_id"`
}

func (s *Store) ListCompanionEdges(ctx context.Context, studentID int64) ([]domain.CompanionEdge, domain.OperationStats, error) {
	rows, stats, err := s.listCompanionEdgeRows(ctx, []int64{studentID})
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CompanionEdge, 0, len(rows))
	for _, row := range rows {
		result = append(result, companionEdgeToDomain(row))
	}
	return result, stats, nil
}

func (s *Store) ListCompanionLinks(ctx context.Context, studentIDs []int64) (map[int64][]domain.CompanionLink, domain.OperationStats, error) {
	result := make(map[int64][]domain.CompanionLink, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	rows, stats, err := s.listCompanionEdgeRows(ctx, studentIDs)
	if err != nil {
		return nil, stats, err
	}
	requested := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		requested[id] = true
	}
	buckets := make(map[int64][]domain.CompanionEdge, len(studentIDs))
	for _, row := range rows {
		edge := companionEdgeToDomain(row)
		if requested[edge.StudentLowID] {
			buckets[edge.StudentLowID] = append(buckets[edge.StudentLowID], edge)
		}
		if requested[edge.StudentHighID] {
			buckets[edge.StudentHighID] = append(buckets[edge.StudentHighID], edge)
		}
	}
	for id, edges := range buckets {
		result[id] = companionLinksFromEdges(id, edges)
	}
	return result, stats, nil
}

func (s *Store) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, domain.OperationStats, error) {
	counts := make(map[int64]int, len(studentIDs))
	if len(studentIDs) == 0 {
		return counts, domain.OperationStats{}, nil
	}
	rows, stats, err := s.listCompanionEdgeRows(ctx, studentIDs)
	if err != nil {
		return nil, stats, err
	}
	requested := idSet(studentIDs)
	distinct := make(map[int64]map[int64]bool, len(studentIDs))
	for _, edge := range rows {
		for _, pair := range [2][2]int64{{edge.StudentLowID, edge.StudentHighID}, {edge.StudentHighID, edge.StudentLowID}} {
			if !requested[pair[0]] || pair[1] == excludeID {
				continue
			}
			if distinct[pair[0]] == nil {
				distinct[pair[0]] = make(map[int64]bool)
			}
			distinct[pair[0]][pair[1]] = true
		}
	}
	for _, id := range studentIDs {
		counts[id] = len(distinct[id])
	}
	return counts, stats, nil
}

func (s *Store) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, domain.OperationStats, error) {
	covered := make(map[int64]map[string]bool, len(studentIDs))
	if len(studentIDs) == 0 {
		return covered, domain.OperationStats{}, nil
	}
	rows, stats, err := s.listCompanionEdgeRows(ctx, studentIDs)
	if err != nil {
		return nil, stats, err
	}
	requested := idSet(studentIDs)
	for _, edge := range rows {
		day := companionDayKeys[edge.Weekday]
		for _, pair := range [2][2]int64{{edge.StudentLowID, edge.StudentHighID}, {edge.StudentHighID, edge.StudentLowID}} {
			if !requested[pair[0]] || pair[1] == excludeID {
				continue
			}
			if covered[pair[0]] == nil {
				covered[pair[0]] = make(map[string]bool, 5)
			}
			covered[pair[0]][day] = true
		}
	}
	return covered, stats, nil
}

func (s *Store) ReplaceCompanionEdges(ctx context.Context, studentID int64, edges []domain.CompanionEdge) (domain.OperationStats, error) {
	db, tenantID, err := s.databaseForWrite(ctx, "replace companion edges")
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats, err := execAny(ctx, db.NewDelete().Model((*companionEdgeRow)(nil)).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID).
		Where(`"student_companion".tenant_id = ?`, tenantID), "clear companion edges")
	if err != nil || len(edges) == 0 {
		return stats, err
	}
	rows := make([]companionEdgeRow, 0, len(edges))
	for _, edge := range edges {
		row := companionEdgeFromDomain(edge)
		row.TenantID = tenantID
		rows = append(rows, row)
	}
	insertStats, err := execAny(ctx, db.NewInsert().Model(&rows).
		ModelTableExpr(`users.student_companions AS "student_companion"`), "insert companion edges")
	stats.Add(insertStats)
	return stats, err
}

func (s *Store) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) (domain.OperationStats, error) {
	if len(edgeIDs) == 0 {
		return domain.OperationStats{}, nil
	}
	db, tenantID, err := s.databaseForWrite(ctx, "delete companion edges")
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execAny(ctx, db.NewDelete().Model((*companionEdgeRow)(nil)).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`"student_companion".id IN (?)`, bun.List(edgeIDs)).
		Where(`"student_companion".tenant_id = ?`, tenantID), "delete companion edges")
}

func (s *Store) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	days := []int{}
	query := db.NewSelect().TableExpr(`users.student_companions AS "student_companion"`).
		ColumnExpr(`DISTINCT "student_companion".weekday`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID).
		OrderExpr(`"student_companion".weekday ASC`)
	query = withTenant(query, "student_companion", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &days)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		err = fmt.Errorf("care plan postgres: list companion weekdays: %w", err)
	}
	stats.Rows = int64(len(days))
	return days, stats, err
}

func (s *Store) CountCompanionLinks(ctx context.Context, studentID int64) (int, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := db.NewSelect().TableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID)
	query = withTenant(query, "student_companion", tenantID)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	count, err := query.Count(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("care plan postgres: count companion links: %w", err)
	}
	stats.Rows = int64(count)
	return count, stats, nil
}

func (s *Store) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, domain.OperationStats, error) {
	result := make(map[int64][]int64)
	if len(studentIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	rows, stats, err := s.listCompanionGroupEdges(ctx, studentIDs, weekday)
	if err != nil {
		return nil, stats, err
	}
	componentByStudent, membersByComponent := companionComponents(rows)
	for _, studentID := range studentIDs {
		members := membersByComponent[componentByStudent[studentID]]
		companions := slices.DeleteFunc(slices.Clone(members), func(id int64) bool { return id == studentID })
		if len(companions) > 0 {
			result[studentID] = companions
		}
	}
	return result, stats, nil
}

func (s *Store) listCompanionGroupEdges(
	ctx context.Context, studentIDs []int64, weekday int,
) ([]companionPair, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []companionPair{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		WITH RECURSIVE reach (member) AS (
			SELECT seed.id FROM unnest(ARRAY[?]::bigint[]) AS seed(id)
			UNION
			SELECT CASE WHEN companion.student_low_id = reach.member THEN companion.student_high_id ELSE companion.student_low_id END
			FROM reach JOIN users.student_companions AS companion
			  ON (companion.student_low_id = reach.member OR companion.student_high_id = reach.member)
			 AND companion.weekday = ? AND (? = 0 OR companion.tenant_id = ?)
		)
		SELECT companion.student_low_id AS low_id, companion.student_high_id AS high_id
		FROM users.student_companions AS companion JOIN reach ON reach.member = companion.student_low_id
		WHERE companion.weekday = ? AND (? = 0 OR companion.tenant_id = ?)
	`, bun.List(studentIDs), weekday, tenantID, tenantID, weekday, tenantID, tenantID).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("care plan postgres: list companion group: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func companionComponents(rows []companionPair) (map[int64]int64, map[int64][]int64) {
	parent := make(map[int64]int64, len(rows)*2)
	var find func(int64) int64
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
		parent[id] = root
		return root
	}
	for _, row := range rows {
		low, high := find(row.Low), find(row.High)
		if low != high {
			parent[high] = low
		}
	}
	members := make(map[int64][]int64)
	for id := range parent {
		root := find(id)
		parent[id] = root
		members[root] = append(members[root], id)
	}
	for _, group := range members {
		slices.Sort(group)
	}
	return parent, members
}

func (s *Store) listCompanionEdgeRows(ctx context.Context, studentIDs []int64) ([]companionEdgeRow, domain.OperationStats, error) {
	if len(studentIDs) == 0 {
		return []companionEdgeRow{}, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []companionEdgeRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id IN (?) OR "student_companion".student_high_id IN (?))`, bun.List(studentIDs), bun.List(studentIDs)).
		OrderExpr(`"student_companion".weekday ASC, "student_companion".id ASC`)
	query = withTenant(query, "student_companion", tenantID)
	stats, err := scanAll(ctx, query, "list companion edges")
	stats.Rows = int64(len(rows))
	return rows, stats, err
}

func companionLinksFromEdges(studentID int64, edges []domain.CompanionEdge) []domain.CompanionLink {
	byCompanion := make(map[int64]map[int]bool)
	for _, edge := range edges {
		other := edge.StudentLowID
		if other == studentID {
			other = edge.StudentHighID
		} else if edge.StudentHighID != studentID {
			continue
		}
		if byCompanion[other] == nil {
			byCompanion[other] = make(map[int]bool)
		}
		byCompanion[other][edge.Weekday] = true
	}
	result := make([]domain.CompanionLink, 0, len(byCompanion))
	for id, days := range byCompanion {
		numbers := make([]int, 0, len(days))
		for day := range days {
			numbers = append(numbers, day)
		}
		sort.Ints(numbers)
		keys := make([]string, 0, len(numbers))
		for _, day := range numbers {
			keys = append(keys, companionDayKeys[day])
		}
		result = append(result, domain.CompanionLink{CompanionStudentID: id, Weekdays: keys})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CompanionStudentID < result[j].CompanionStudentID })
	return result
}

func companionEdgeFromDomain(value domain.CompanionEdge) companionEdgeRow {
	return companionEdgeRow{ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, StudentLowID: value.StudentLowID, StudentHighID: value.StudentHighID, Weekday: value.Weekday}
}

func companionEdgeToDomain(row companionEdgeRow) domain.CompanionEdge {
	return domain.CompanionEdge{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentLowID: row.StudentLowID, StudentHighID: row.StudentHighID, Weekday: row.Weekday}
}

func idSet(ids []int64) map[int64]bool {
	result := make(map[int64]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result
}
