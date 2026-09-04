package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun"
)

type groupRow struct {
	bun.BaseModel         `bun:"table:groups,alias:group"`
	ID                    int64      `bun:"id,pk,autoincrement"`
	TenantID              int64      `bun:"tenant_id,notnull"`
	CreatedAt             time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt             time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Name                  string     `bun:"name"`
	MaxParticipants       int        `bun:"max_participants,nullzero"`
	RequiredStaff         *int       `bun:"required_staff"`
	IsOpen                bool       `bun:"is_open"`
	CategoryID            int64      `bun:"category_id"`
	PlanningTrackID       *int64     `bun:"planning_track_id"`
	PlannedRoomID         *int64     `bun:"planned_room_id"`
	CreatedBy             *int64     `bun:"created_by"`
	Type                  string     `bun:"type"`
	EducationGroupID      *int64     `bun:"education_group_id"`
	ListKind              *string    `bun:"list_kind"`
	IsTemplate            bool       `bun:"is_template"`
	IsSystem              bool       `bun:"is_system"`
	ArchivedAt            *time.Time `bun:"archived_at"`
	SeriesRootID          *int64     `bun:"series_root_id"`
	CalendarPeriodID      *int64     `bun:"calendar_period_id"`
	TargetGroupType       string     `bun:"target_group_type"`
	TargetGradeLevel      *int16     `bun:"target_grade_level"`
	TargetSchoolClass     *string    `bun:"target_school_class"`
	SourceCareOfferingIDs []int64    `bun:"source_care_offering_ids,type:jsonb,nullzero"`
	SourceGradeLevels     []int      `bun:"source_grade_levels,type:jsonb,nullzero"`
	SourceSchoolClasses   []string   `bun:"source_school_classes,type:jsonb,nullzero"`
	Notes                 *string    `bun:"notes"`
	CategoryName          string     `bun:"category_name,scanonly"`
	CategoryCreatedAt     time.Time  `bun:"category_created_at,scanonly"`
	CategoryUpdatedAt     time.Time  `bun:"category_updated_at,scanonly"`
	CategoryDescription   string     `bun:"category_description,scanonly"`
	CategoryColor         string     `bun:"category_color,scanonly"`
	CategoryIsSystem      bool       `bun:"category_is_system,scanonly"`
	CategoryShiftTypeID   *int64     `bun:"category_shift_type_id,scanonly"`
	CategoryArchivedAt    *time.Time `bun:"category_archived_at,scanonly"`
}

type groupTargetRow struct {
	ID                 int64     `bun:"id,pk,autoincrement"`
	TenantID           int64     `bun:"tenant_id"`
	CreatedAt          time.Time `bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt          time.Time `bun:"updated_at,nullzero,default:current_timestamp"`
	ActivityGroupID    int64     `bun:"activity_group_id"`
	TargetGroupType    string    `bun:"target_group_type"`
	TargetGradeLevel   *int16    `bun:"target_grade_level"`
	TargetSchoolClass  *string   `bun:"target_school_class"`
	EducationGroupID   *int64    `bun:"education_group_id"`
	EducationGroupName string    `bun:"education_group_name,scanonly"`
}

func (s *Store) FindGroup(ctx context.Context, id int64, lock string) (domain.Group, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Group{}, false, domain.OperationStats{}, err
	}
	row := groupRow{}
	query := db.NewSelect().Model(&row).ModelTableExpr(`activities.groups AS "group"`).ColumnExpr(`"group".*`).
		Where(`"group".tenant_id = ?`, tenantID).Where(`"group".id = ?`, id)
	if lock != "" {
		query = query.For(lock)
	}
	found, stats, err := scanOne(ctx, query, "find group")
	return groupToDomain(row, false), found, stats, err
}

func (s *Store) FindGroupByName(ctx context.Context, name string) (domain.Group, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Group{}, false, domain.OperationStats{}, err
	}
	row := groupRow{}
	query := db.NewSelect().Model(&row).ModelTableExpr(`activities.groups AS "group"`).ColumnExpr(`"group".*`).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`LOWER(TRIM("group".name)) = LOWER(TRIM(?))`, name).
		Where(`"group".archived_at IS NULL`)
	found, stats, err := scanOne(ctx, query, "find group by name")
	return groupToDomain(row, false), found, stats, err
}

func (s *Store) ListGroups(ctx context.Context, filter domain.GroupFilter) ([]domain.Group, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []groupRow{}
	query := groupListQuery(db, &rows, tenantID, filter)
	stats, err := scanAll(ctx, query, "list groups")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.Group, 0, len(rows))
	for _, row := range rows {
		result = append(result, groupToDomain(row, true))
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) CreateGroup(ctx context.Context, fields domain.GroupFields) (domain.Group, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Group{}, domain.OperationStats{}, err
	}
	row := groupRow{TenantID: tenantID}
	applyGroupFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`activities.groups`).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.Group{}, stats, classifyWriteError("create group", err, &stats)
	}
	stats.Rows = 1
	return groupToDomain(row, false), stats, nil
}

func (s *Store) UpdateGroup(ctx context.Context, id int64, fields domain.GroupFields) (domain.Group, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.Group{}, false, domain.OperationStats{}, err
	}
	row := groupRow{}
	row.ID, row.TenantID = id, tenantID
	applyGroupFields(&row, fields)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = groupUpdateQuery(db, &row, tenantID, id).Returning("*").Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Group{}, false, stats, nil
	}
	if err != nil {
		return domain.Group{}, false, stats, classifyWriteError("update group", err, &stats)
	}
	stats.Rows = 1
	return groupToDomain(row, false), true, stats, nil
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewDelete().Table("activities.groups").Where("tenant_id = ?", tenantID).Where("id = ?", id).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError("delete group", err, &stats)
	}
	_, err = rowsAffected(result, "deleted groups", &stats)
	return stats, err
}

func (s *Store) UpdateTemplate(ctx context.Context, id int64, fields domain.TemplateFields) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	offeringIDs, grades, classes, err := templateSourceValues(fields)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := templateUpdateQuery(db, tenantID, id, fields, offeringIDs, grades, classes)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, classifyWriteError("update template", err, &stats)
	}
	rows, err := rowsAffected(result, "updated templates", &stats)
	return rows, stats, err
}

func (s *Store) ArchiveTemplate(ctx context.Context, id int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	now := time.Now()
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Table("activities.groups").Set("archived_at = ?", now).
		Set("updated_at = ?", now).Where("tenant_id = ?", tenantID).Where("id = ?", id).
		Where("is_template = TRUE").Where("archived_at IS NULL").Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, classifyWriteError("archive template", err, &stats)
	}
	rows, err := rowsAffected(result, "archived templates", &stats)
	return rows, stats, err
}

func (s *Store) UpdateGroupOfferingSource(ctx context.Context, id int64, fields domain.OfferingSourceFields) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	offeringIDs, err := jsonText(fields.CareOfferingIDs)
	if err != nil {
		return domain.OperationStats{}, fmt.Errorf("timetable postgres: encode source care offering IDs: %w", err)
	}
	grades, err := jsonText(fields.GradeLevels)
	if err != nil {
		return domain.OperationStats{}, fmt.Errorf("timetable postgres: encode source grade levels: %w", err)
	}
	classes, err := jsonText(fields.SchoolClasses)
	if err != nil {
		return domain.OperationStats{}, fmt.Errorf("timetable postgres: encode source school classes: %w", err)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().Table("activities.groups").Set("source_care_offering_ids = ?", offeringIDs).
		Set("source_grade_levels = ?", grades).Set("source_school_classes = ?", classes).
		Set("updated_at = ?", time.Now()).Where("tenant_id = ?", tenantID).Where("id = ?", id).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, classifyWriteError("update group offering source", err, &stats)
	}
	_, err = rowsAffected(result, "updated group offering sources", &stats)
	return stats, err
}

func templateUpdateQuery(db bun.IDB, tenantID, id int64, fields domain.TemplateFields, offeringIDs, grades, classes any) *bun.UpdateQuery {
	query := db.NewUpdate().Table("activities.groups").Set("name = ?", fields.Name).Set("type = ?", fields.Type).
		Set("category_id = ?", fields.CategoryID).Set("planned_room_id = ?", fields.RoomID).
		Set("education_group_id = ?", fields.EducationGroupID).Set("required_staff = ?", fields.RequiredStaff).
		Set("calendar_period_id = ?", fields.CalendarPeriodID).Set("target_group_type = ?", fields.TargetGroupType).
		Set("target_grade_level = ?", fields.TargetGradeLevel).Set("target_school_class = ?", fields.TargetSchoolClass).
		Set("source_care_offering_ids = ?", offeringIDs).Set("source_grade_levels = ?", grades).
		Set("source_school_classes = ?", classes).Set("list_kind = ?", fields.ListKind).
		Set("notes = ?", fields.Notes).Set("updated_at = ?", time.Now()).
		Where("tenant_id = ?", tenantID).Where("id = ?", id).Where("is_template = TRUE").Where("archived_at IS NULL")
	if fields.PlanningTrackIDProvided {
		query = query.Set("planning_track_id = ?", fields.PlanningTrackID)
	}
	if fields.MaxParticipantsProvided || fields.MaxParticipants > 0 {
		var limit any
		if fields.MaxParticipants > 0 {
			limit = fields.MaxParticipants
		}
		query = query.Set("max_participants = ?", limit)
	}
	return query
}

func templateSourceValues(fields domain.TemplateFields) (any, any, any, error) {
	offeringIDs, err := jsonText(fields.SourceCareOfferingIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("timetable postgres: encode source care offering IDs: %w", err)
	}
	grades, err := jsonText(fields.SourceGradeLevels)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("timetable postgres: encode source grade levels: %w", err)
	}
	classes, err := jsonText(fields.SourceSchoolClasses)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("timetable postgres: encode source school classes: %w", err)
	}
	return offeringIDs, grades, classes, nil
}

func jsonText[T any](values []T) (any, error) {
	if len(values) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

func groupListQuery(db bun.IDB, rows *[]groupRow, tenantID int64, filter domain.GroupFilter) *bun.SelectQuery {
	query := db.NewSelect().Model(rows).ModelTableExpr(`activities.groups AS "group"`).
		ColumnExpr(`"group".*`).
		ColumnExpr(`"category".created_at AS category_created_at, "category".updated_at AS category_updated_at`).
		ColumnExpr(`"category".name AS category_name, "category".description AS category_description`).
		ColumnExpr(`"category".color AS category_color, "category".is_system AS category_is_system`).
		ColumnExpr(`"category".shift_type_id AS category_shift_type_id, "category".archived_at AS category_archived_at`).
		Join(`LEFT JOIN activities.categories AS "category" ON "category".tenant_id = "group".tenant_id AND "category".id = "group".category_id`).
		Where(`"group".tenant_id = ?`, tenantID)
	if filter.Name != "" {
		query = query.Where(`"group".name = ?`, filter.Name)
	}
	if filter.CategoryID != nil {
		query = query.Where(`"group".category_id = ?`, *filter.CategoryID)
	}
	if filter.IsOpen != nil {
		query = query.Where(`"group".is_open = ?`, *filter.IsOpen)
	}
	if filter.IsSystem != nil {
		query = query.Where(`"group".is_system = ?`, *filter.IsSystem)
	}
	if filter.IsTemplate != nil {
		query = query.Where(`"group".is_template = ?`, *filter.IsTemplate)
	}
	if len(filter.IDs) > 0 {
		query = query.Where(`"group".id IN (?)`, bun.List(filter.IDs))
	}
	query = applyGroupSourceFilter(query, tenantID, filter)
	if filter.ActiveOnly {
		query = query.Where(`"group".archived_at IS NULL`)
	}
	if filter.OrderByName {
		query = query.OrderExpr(`"group".name ASC`)
	}
	if filter.OrderByID {
		query = query.OrderExpr(`"group".id ASC`)
	}
	return query
}

func applyGroupSourceFilter(query *bun.SelectQuery, tenantID int64, filter domain.GroupFilter) *bun.SelectQuery {
	if filter.SeriesForGroupID != nil {
		query = query.Where(`COALESCE("group".series_root_id, "group".id) = (
			SELECT COALESCE(selected.series_root_id, selected.id) FROM activities.groups AS selected
			WHERE selected.tenant_id = ? AND selected.id = ?)`, tenantID, *filter.SeriesForGroupID)
	}
	if len(filter.SourceOfferingIDs) == 1 {
		query = query.Where(`"group".source_care_offering_ids @> to_jsonb(?::BIGINT)`, filter.SourceOfferingIDs[0])
	} else if len(filter.SourceOfferingIDs) > 1 {
		query = query.Where(`EXISTS (
			SELECT 1 FROM jsonb_array_elements_text("group".source_care_offering_ids) AS source(id)
			WHERE source.id::BIGINT IN (?))`, bun.List(filter.SourceOfferingIDs))
	}
	if filter.HasOfferingSource {
		query = query.Where(`"group".source_care_offering_ids IS NOT NULL`)
	}
	return query
}

func (s *Store) ListGroupTargets(ctx context.Context, ids []int64) (map[int64][]domain.GroupTarget, domain.OperationStats, error) {
	result := make(map[int64][]domain.GroupTarget, len(ids))
	if len(ids) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []groupTargetRow{}
	query := db.NewSelect().Model(&rows).ModelTableExpr(`activities.group_targets AS "target"`).ColumnExpr(`"target".*`).
		Where(`"target".tenant_id = ?`, tenantID).Where(`"target".activity_group_id IN (?)`, bun.List(ids)).
		OrderExpr(`"target".activity_group_id ASC, "target".id ASC`)
	stats, err := scanAll(ctx, query, "list group targets")
	if err != nil {
		return nil, stats, err
	}
	for _, row := range rows {
		result[row.ActivityGroupID] = append(result[row.ActivityGroupID], groupTargetToDomain(row))
	}
	stats.Rows = int64(len(rows))
	return result, stats, nil
}

func (s *Store) ReplaceGroupTargets(ctx context.Context, groupID int64, targets []domain.GroupTargetFields) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	if err := deleteGroupTargets(ctx, db, tenantID, groupID, &stats); err != nil {
		return stats, err
	}
	if len(targets) == 0 {
		return stats, nil
	}
	rows := make([]groupTargetRow, 0, len(targets))
	for _, target := range targets {
		rows = append(rows, groupTargetRow{TenantID: tenantID, ActivityGroupID: groupID,
			TargetGroupType: target.TargetGroupType, TargetGradeLevel: target.TargetGradeLevel,
			TargetSchoolClass: target.TargetSchoolClass, EducationGroupID: target.EducationGroupID})
	}
	started := time.Now()
	result, err := db.NewInsert().Model(&rows).ModelTableExpr(`activities.group_targets`).Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("timetable postgres: create group targets: %w", err)
	}
	_, err = rowsAffected(result, "created group targets", &stats)
	return stats, err
}

func deleteGroupTargets(ctx context.Context, db bun.IDB, tenantID, groupID int64, stats *domain.OperationStats) error {
	started := time.Now()
	result, err := db.NewDelete().Table("activities.group_targets").
		Where("tenant_id = ?", tenantID).Where("activity_group_id = ?", groupID).Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return fmt.Errorf("timetable postgres: delete group targets: %w", err)
	}
	_, err = rowsAffected(result, "deleted group targets", stats)
	return err
}

func applyGroupFields(row *groupRow, fields domain.GroupFields) {
	row.Name, row.MaxParticipants, row.RequiredStaff = fields.Name, fields.MaxParticipants, fields.RequiredStaff
	row.IsOpen, row.CategoryID, row.PlanningTrackID = fields.IsOpen, fields.CategoryID, fields.PlanningTrackID
	row.PlannedRoomID, row.CreatedBy, row.Type = fields.PlannedRoomID, fields.CreatedBy, fields.Type
	row.EducationGroupID, row.ListKind, row.IsTemplate = fields.EducationGroupID, fields.ListKind, fields.IsTemplate
	row.IsSystem, row.ArchivedAt, row.SeriesRootID = fields.IsSystem, fields.ArchivedAt, fields.SeriesRootID
	row.CalendarPeriodID, row.TargetGroupType = fields.CalendarPeriodID, fields.TargetGroupType
	row.TargetGradeLevel, row.TargetSchoolClass = fields.TargetGradeLevel, fields.TargetSchoolClass
	row.SourceCareOfferingIDs, row.SourceGradeLevels = fields.SourceCareOfferingIDs, fields.SourceGradeLevels
	row.SourceSchoolClasses, row.Notes = fields.SourceSchoolClasses, fields.Notes
}

func groupUpdateQuery(db bun.IDB, row *groupRow, tenantID, id int64) *bun.UpdateQuery {
	return db.NewUpdate().Model(row).ModelTableExpr(`activities.groups`).
		Column("name", "max_participants", "required_staff", "is_open", "category_id", "planning_track_id").
		Column("planned_room_id", "created_by", "type", "education_group_id", "list_kind", "is_template").
		Column("is_system", "archived_at", "series_root_id", "calendar_period_id", "target_group_type").
		Column("target_grade_level", "target_school_class", "source_care_offering_ids", "source_grade_levels").
		Column("source_school_classes", "notes").
		Where("id = ?", id).Where("tenant_id = ?", tenantID)
}

func groupToDomain(row groupRow, withCategory bool) domain.Group {
	group := domain.Group{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Name: row.Name, MaxParticipants: row.MaxParticipants, RequiredStaff: row.RequiredStaff, IsOpen: row.IsOpen,
		CategoryID: row.CategoryID, PlanningTrackID: row.PlanningTrackID, PlannedRoomID: row.PlannedRoomID,
		CreatedBy: row.CreatedBy, Type: row.Type, EducationGroupID: row.EducationGroupID, ListKind: row.ListKind,
		IsTemplate: row.IsTemplate, IsSystem: row.IsSystem, ArchivedAt: row.ArchivedAt, SeriesRootID: row.SeriesRootID,
		CalendarPeriodID: row.CalendarPeriodID, TargetGroupType: row.TargetGroupType,
		TargetGradeLevel: row.TargetGradeLevel, TargetSchoolClass: row.TargetSchoolClass,
		SourceCareOfferingIDs: row.SourceCareOfferingIDs, SourceGradeLevels: row.SourceGradeLevels,
		SourceSchoolClasses: row.SourceSchoolClasses, Notes: row.Notes,
	}
	if withCategory {
		group.Category = &domain.Category{ID: row.CategoryID, TenantID: row.TenantID,
			CreatedAt: row.CategoryCreatedAt, UpdatedAt: row.CategoryUpdatedAt, Name: row.CategoryName,
			Description: row.CategoryDescription, Color: row.CategoryColor, IsSystem: row.CategoryIsSystem,
			ShiftTypeID: row.CategoryShiftTypeID, ArchivedAt: row.CategoryArchivedAt}
	}
	return group
}

func groupTargetToDomain(row groupTargetRow) domain.GroupTarget {
	return domain.GroupTarget{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ActivityGroupID: row.ActivityGroupID,
		TargetGroupType: row.TargetGroupType, TargetGradeLevel: row.TargetGradeLevel,
		TargetSchoolClass: row.TargetSchoolClass, EducationGroupID: row.EducationGroupID,
		EducationGroupName: row.EducationGroupName,
	}
}
