// backend/database/repositories/activities/group.go
package activities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesGroups          = "activities.groups"
	tableExprActivitiesGroupsAsGrp = `activities.groups AS "group"`
	tableExprGroupTargets          = `activities.group_targets AS "group_target"`
	orderByNameAsc                 = "name ASC"
	whereIDEquals                  = "id = ?"
)

// GroupRepository implements activities.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*activities.Group]
	db       *bun.DB
	students StudentDirectory
	periods  CalendarPeriodSource
	rooms    TemplateRoomDirectory
	shifts   TemplateShiftTypeDirectory
}

type TemplateRoom struct {
	ID   int64
	Name string
}

type TemplateRoomDirectory interface {
	ListRoomsByID(context.Context, []int64) ([]TemplateRoom, error)
}

type TemplateShiftType struct {
	ID    int64
	Name  string
	Color string
}

type TemplateShiftTypeDirectory interface {
	ListShiftTypes(context.Context) ([]TemplateShiftType, error)
}

// BindStudentDirectory installs the People Directory the dynamic target
// cohorts are resolved through (#2662).
func (r *GroupRepository) BindStudentDirectory(students StudentDirectory) {
	r.students = students
}

// BindCalendarPeriods installs the School Calendar query the capacity
// occurrences read active periods through (#2666).
func (r *GroupRepository) BindCalendarPeriods(periods CalendarPeriodSource) {
	r.periods = periods
}

func (r *GroupRepository) BindTemplateRooms(rooms TemplateRoomDirectory) {
	r.rooms = rooms
}

func (r *GroupRepository) BindTemplateShiftTypes(shifts TemplateShiftTypeDirectory) {
	r.shifts = shifts
}

// NewGroupRepository creates a new GroupRepository
func NewGroupRepository(db *bun.DB) activities.GroupRepository {
	repo := base.NewRepository[*activities.Group](db, tableActivitiesGroups, "Group")
	repo.TenantScoped = true
	return &GroupRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByName finds a non-archived group by name, case-insensitively.
func (r *GroupRepository) FindByName(ctx context.Context, name string) (*activities.Group, error) {
	group := new(activities.Group)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`LOWER(TRIM("group".name)) = LOWER(TRIM(?))`, name).
		Where(`"group".archived_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find group by name",
			Err: base.TranslateNotFound(err),
		}
	}

	return group, nil
}

// FindByIDs returns the groups matching ids in one tenant-scoped IN query.
// Archived rows are included so display names for historical references
// still resolve (same behavior as the generic FindByID). Custom method
// (backend-conventions Rule 2): bulk IN lookup with the empty-slice
// short-circuit, mirroring facilities.RoomRepository.FindByIDs.
func (r *GroupRepository) FindByIDs(ctx context.Context, ids []int64) ([]*activities.Group, error) {
	groups := make([]*activities.Group, 0, len(ids))
	if len(ids) == 0 {
		return groups, nil
	}
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find groups by ids",
			Err: base.TranslateNotFound(err),
		}
	}
	return groups, nil
}

// FindTemplateSeries resolves groupID to its stable split-series root and
// returns every live segment in that lineage. Tenant predicates are explicit
// defense-in-depth alongside RLS.
func (r *GroupRepository) FindTemplateSeries(ctx context.Context, groupID int64) ([]*activities.Group, error) {
	tenantID := tenant.FromContext(ctx)
	groups := make([]*activities.Group, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".is_template = TRUE`).
		Where(`"group".archived_at IS NULL`).
		Where(`COALESCE("group".series_root_id, "group".id) = (
			SELECT COALESCE(selected.series_root_id, selected.id)
			FROM activities.groups AS selected
			WHERE selected.tenant_id = ? AND selected.id = ?
		)`, tenantID, groupID).
		OrderExpr(`"group".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find template series", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// FindTemplatesBySourceOffering returns every live template whose source id
// array contains the given care offering (#2137). The jsonb containment
// operator is served by the partial GIN index idx_activities_groups_source_offerings.
func (r *GroupRepository) FindTemplatesBySourceOffering(ctx context.Context, offeringID int64) ([]*activities.Group, error) {
	tenantID := tenant.FromContext(ctx)
	groups := make([]*activities.Group, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".is_template = TRUE`).
		Where(`"group".archived_at IS NULL`).
		Where(`"group".source_care_offering_ids @> to_jsonb(?::BIGINT)`, offeringID).
		OrderExpr(`"group".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find templates by source offering", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// FindTemplatesBySourceOfferings returns live templates fed by any supplied
// care offering in one tenant-scoped query. The generic filter cannot express
// membership in the JSONB source_care_offering_ids array.
func (r *GroupRepository) FindTemplatesBySourceOfferings(ctx context.Context, offeringIDs []int64) ([]*activities.Group, error) {
	if len(offeringIDs) == 0 {
		return []*activities.Group{}, nil
	}
	tenantID := tenant.FromContext(ctx)
	groups := make([]*activities.Group, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".is_template = TRUE`).
		Where(`"group".archived_at IS NULL`).
		Where(`EXISTS (
			SELECT 1 FROM jsonb_array_elements_text("group".source_care_offering_ids) AS source(id)
			WHERE source.id::BIGINT IN (?)
		)`, bun.List(offeringIDs)).
		OrderExpr(`"group".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find templates by source offerings", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// UpdateTemplateOfferingSource rewrites only the offering-source columns of
// one template — see the interface doc for why the detach flow needs this
// (jsonb array carries no FK; both columns must change atomically under
// chk_activities_groups_offering_source).
func (r *GroupRepository) UpdateTemplateOfferingSource(ctx context.Context, id int64, offeringIDs []int64, gradeLevels []int, schoolClasses []string) error {
	tenantID := tenant.FromContext(ctx)
	var offeringIDsValue, gradeLevelsValue, schoolClassesValue any
	if len(offeringIDs) > 0 {
		encoded, err := json.Marshal(offeringIDs)
		if err != nil {
			return &modelBase.DatabaseError{Op: "update template offering source", Err: fmt.Errorf("marshal source_care_offering_ids: %w", err)}
		}
		offeringIDsValue = string(encoded)
		if len(gradeLevels) > 0 {
			encodedLevels, err := json.Marshal(gradeLevels)
			if err != nil {
				return &modelBase.DatabaseError{Op: "update template offering source", Err: fmt.Errorf("marshal source_grade_levels: %w", err)}
			}
			gradeLevelsValue = string(encodedLevels)
		}
		if len(schoolClasses) > 0 {
			// Both filters are written through as given. Grade and class are
			// mutually exclusive, but enforcing that HERE by dropping one
			// would lose a filter silently; the DB CHECK
			// chk_activities_groups_offering_source rejects the pair loudly
			// instead, which is what a caller that skipped validation needs
			// to see (#2482).
			encodedClasses, err := json.Marshal(schoolClasses)
			if err != nil {
				return &modelBase.DatabaseError{Op: "update template offering source", Err: fmt.Errorf("marshal source_school_classes: %w", err)}
			}
			schoolClassesValue = string(encodedClasses)
		}
	}
	_, err := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.groups").
		Set("source_care_offering_ids = ?", offeringIDsValue).
		Set("source_grade_levels = ?", gradeLevelsValue).
		Set("source_school_classes = ?", schoolClassesValue).
		Set("updated_at = ?", time.Now()).
		Where("tenant_id = ?", tenantID).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update template offering source", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// FindTemplatesWithOfferingSource returns every live template of the tenant
// that declares an offering source (#2137). See the interface doc for why
// this cannot be a per-offering lookup.
func (r *GroupRepository) FindTemplatesWithOfferingSource(ctx context.Context) ([]*activities.Group, error) {
	tenantID := tenant.FromContext(ctx)
	groups := make([]*activities.Group, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".is_template = TRUE`).
		Where(`"group".archived_at IS NULL`).
		Where(`"group".source_care_offering_ids IS NOT NULL`).
		OrderExpr(`"group".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find templates with offering source", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

func (r *GroupRepository) ReplaceTargets(ctx context.Context, groupID int64, targets []*activities.GroupTarget) error {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("replace group targets requires a tenant")
	}
	if groupID <= 0 {
		return errors.New("replace group targets requires a positive activity group id")
	}
	normalized := make([]*activities.GroupTarget, 0, len(targets))
	var targetType string
	for _, target := range targets {
		if target == nil {
			return errors.New("invalid group target: target cannot be null")
		}
		copy := *target
		copy.ID = 0
		copy.ActivityGroupID = groupID
		copy.SetTenantID(tenantID)
		if err := copy.Validate(); err != nil {
			return fmt.Errorf("invalid group target: %w", err)
		}
		if targetType == "" {
			targetType = copy.TargetGroupType
		} else if copy.TargetGroupType != targetType {
			return errors.New("invalid group targets: all targets must have the same type")
		}
		normalized = append(normalized, &copy)
	}

	return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		db := base.GetDB(txCtx, r.db)
		if _, err := db.NewDelete().
			Model((*activities.GroupTarget)(nil)).
			ModelTableExpr(tableExprGroupTargets).
			Where(`"group_target".tenant_id = ?`, tenantID).
			Where(`"group_target".activity_group_id = ?`, groupID).
			Exec(txCtx); err != nil {
			return &modelBase.DatabaseError{Op: "delete group targets", Err: base.TranslateNotFound(err)}
		}
		if len(normalized) == 0 {
			return nil
		}
		if _, err := db.NewInsert().
			Model(&normalized).
			ModelTableExpr(tableExprGroupTargets).
			Exec(txCtx); err != nil {
			return &modelBase.DatabaseError{Op: "create group targets", Err: base.TranslateNotFound(err)}
		}
		return nil
	})
}

func (r *GroupRepository) FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activities.GroupTarget, error) {
	result := make(map[int64][]*activities.GroupTarget, len(groupIDs))
	if len(groupIDs) == 0 {
		return result, nil
	}
	tenantID := tenant.FromContext(ctx)
	var targets []*activities.GroupTarget
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&targets).
		ModelTableExpr(`activities.group_targets AS "target"`).
		ColumnExpr(`"target".*`).
		Where(`"target".tenant_id = ?`, tenantID).
		Where(`"target".activity_group_id IN (?)`, bun.List(groupIDs)).
		OrderExpr(`"target".activity_group_id ASC, "target".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find group targets", Err: base.TranslateNotFound(err)}
	}
	for _, target := range targets {
		result[target.ActivityGroupID] = append(result[target.ActivityGroupID], target)
	}
	return result, nil
}

func (r *GroupRepository) FindTargetStudentIDs(ctx context.Context, groupID int64) ([]int64, error) {
	byGroup, err := r.FindTargetStudentIDsByGroupIDs(ctx, []int64{groupID})
	if err != nil {
		return nil, err
	}
	return byGroup[groupID], nil
}

// A child whose care has already ended drops out of every dynamic source
// (#2487) — the source answers "who belongs to this Jahrgang / class / group
// today", and they no longer do. A child whose exit is still ahead stays in:
// the per-date filter in the materializer decides which of the upcoming days
// they are still planned for.
//
// The target rules are read here; the students they match belong to the
// People Directory (#2662) and are matched in Go (see matchTargetStudents).
func (r *GroupRepository) FindTargetStudentIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error) {
	result := make(map[int64][]int64, len(groupIDs))
	if len(groupIDs) == 0 {
		return result, nil
	}
	if r.students == nil {
		return nil, errStudentDirectoryRequired
	}
	targets, err := r.FindTargetsByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return result, nil
	}
	students, err := r.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	today := timezone.TodayDate()
	for groupID, members := range matchTargetStudents(targets, students, &today) {
		if len(members) > 0 {
			result[groupID] = members
		}
	}
	return result, nil
}

// FindByCategory finds all groups in a specific category
func (r *GroupRepository) FindByCategory(ctx context.Context, categoryID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where("category_id = ?", categoryID)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by category",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindOpenGroups finds all groups that are open for enrollment
func (r *GroupRepository) FindOpenGroups(ctx context.Context) ([]*activities.Group, error) {
	var groups []*activities.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where("is_open = ?", true).
		// System activities (Schulhof Freispiel, WC) are created with
		// is_open = true for the IoT flows but are never openly enrollable
		// through the staff UI (issue #923).
		Where("is_system = ?", false)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find open groups",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindAllTemplates returns all activity groups flagged as templates
// (is_template = true). Tenant-scoped via the standard base.TenantWhere helper.
func (r *GroupRepository) FindAllTemplates(ctx context.Context) ([]*activities.Group, error) {
	var groups []*activities.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".is_template = ?`, true).
		Where(`"group".archived_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find all templates",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// FindWithEnrollmentCounts returns groups with their current enrollment counts
func (r *GroupRepository) FindWithEnrollmentCounts(ctx context.Context) ([]*activities.Group, map[int64]int, error) {
	groups := make([]*activities.Group, 0)
	groupQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp)

	groupQuery = base.WithTenantFilter(ctx, groupQuery, "group")

	err := groupQuery.
		Order(orderByNameAsc).
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find groups",
			Err: base.TranslateNotFound(err),
		}
	}

	// If no groups, return early
	if len(groups) == 0 {
		return groups, make(map[int64]int), nil
	}

	// Get all group IDs
	var groupIDs []interface{}
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}

	// Get enrollment counts for each group
	type countResult struct {
		GroupID int64 `bun:"activity_group_id"`
		Count   int   `bun:"count"`
	}
	var counts []countResult
	countQuery := base.GetDB(ctx, r.db).NewSelect().
		ModelTableExpr("activities.student_enrollments AS se").
		ColumnExpr("se.activity_group_id").
		ColumnExpr("COUNT(*) AS count").
		Where("se.activity_group_id IN (?)", bun.List(groupIDs)).
		Group("se.activity_group_id")

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		countQuery = countQuery.Where("se.tenant_id = ?", tenantID)
	}

	err = countQuery.Scan(ctx, &counts)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "count enrollments",
			Err: base.TranslateNotFound(err),
		}
	}

	// Convert to map
	countMap := make(map[int64]int)
	for _, count := range counts {
		countMap[count.GroupID] = count.Count
	}

	return groups, countMap, nil
}

// FindWithSupervisors returns a group with its supervisors
func (r *GroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activities.Group, []*activities.SupervisorPlanned, error) {
	// First get the group
	group := new(activities.Group)
	groupQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(group).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(whereIDEquals, groupID)

	groupQuery = base.WithTenantFilter(ctx, groupQuery, "group")

	err := groupQuery.Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find group",
			Err: base.TranslateNotFound(err),
		}
	}

	// Then get the supervisors
	var supervisors []*activities.SupervisorPlanned
	supQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&supervisors).
		ModelTableExpr(`activities.supervisors AS "supervisor_planned"`).
		Where("group_id = ?", groupID)

	supQuery = base.WithTenantFilter(ctx, supQuery, "supervisor_planned")

	err = supQuery.
		Order("is_primary DESC").
		Scan(ctx)

	if err != nil {
		return nil, nil, &modelBase.DatabaseError{
			Op:  "find supervisors",
			Err: base.TranslateNotFound(err),
		}
	}

	// The Staff relation of every supervisor is attached by the composition
	// layer through School Membership (#2667).
	return group, supervisors, nil
}

// FindByStaffSupervisor finds all activity groups where a staff member is a supervisor
func (r *GroupRepository) FindByStaffSupervisor(ctx context.Context, staffID int64) ([]*activities.Group, error) {
	var groups []*activities.Group
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Join("JOIN activities.supervisors AS s ON s.group_id = \"group\".id").
		Where("s.staff_id = ?", staffID)

	query = base.WithTenantFilter(ctx, query, "group")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff supervisor",
			Err: base.TranslateNotFound(err),
		}
	}

	return groups, nil
}

// Update overrides the base Update method to handle validation
func (r *GroupRepository) Update(ctx context.Context, group *activities.Group) error {
	if group == nil {
		return fmt.Errorf("group cannot be nil")
	}

	// Validate group
	if err := group.Validate(); err != nil {
		return err
	}

	// Get the query builder - GetDB handles transaction extraction from context
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(group).
		Where(whereIDEquals, group.ID).
		ModelTableExpr(tableActivitiesGroups)

	if tenantID := tenant.FromContext(ctx); tenantID > 0 {
		query = query.Where("tenant_id = ?", tenantID)
	}

	// Execute the query
	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: base.TranslateNotFound(err),
		}
	}

	return base.AssertRowsAffected(result, 1, "update group")
}

// List implements the model repository's QueryOptions list contract.
func (r *GroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Group, error) {
	return r.ListWithOptions(ctx, options)
}

// ListWithCategory lists groups and their category in one joined snapshot.
func (r *GroupRepository) ListWithCategory(ctx context.Context, params *activities.GroupListQuery) ([]*activities.Group, error) {
	groups := make([]*activities.Group, 0)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		ColumnExpr(`"group".*`).
		ColumnExpr(`"category"."id" AS "category__id"`).
		ColumnExpr(`"category"."created_at" AS "category__created_at"`).
		ColumnExpr(`"category"."updated_at" AS "category__updated_at"`).
		ColumnExpr(`"category"."name" AS "category__name"`).
		ColumnExpr(`"category"."description" AS "category__description"`).
		ColumnExpr(`"category"."color" AS "category__color"`).
		Join(`LEFT JOIN activities.categories AS "category" ON "category"."id" = "group"."category_id"`)
	query = base.WithTenantFilter(ctx, query, "group")
	if params != nil {
		if params.Name != "" {
			query = query.Where(`"group".name = ?`, params.Name)
		}
		if params.CategoryID != nil {
			query = query.Where(`"group".category_id = ?`, *params.CategoryID)
		}
		if params.IsSystem != nil {
			query = query.Where(`"group".is_system = ?`, *params.IsSystem)
		}
		if len(params.IDs) > 0 {
			query = query.Where(`"group".id IN (?)`, bun.List(params.IDs))
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: base.TranslateNotFound(err)}
	}
	return groups, nil
}

// templateListSelect is the shared SELECT head of the template list read
// model (issue #584: moved verbatim from api/timetable/templates_list.go).
const templateListSelect = `
		SELECT
			g.id AS template_id,
			g.name,
			g.type,
			g.category_id,
			COALESCE(c.name, '') AS category_name,
			g.planning_track_id,
			COALESCE(pt.name, '') AS planning_track_name,
			COALESCE(pt.color, '') AS planning_track_color,
			pt.sort_order AS planning_track_sort_order,
				g.planned_room_id AS room_id,
				'' AS room_name,
				g.education_group_id,
				g.is_open,
			COALESCE(g.max_participants, 0) AS max_participants,
			g.required_staff,
			g.calendar_period_id AS template_calendar_period_id,
			g.target_group_type,
			g.target_grade_level,
			g.target_school_class,
			COALESCE(g.source_care_offering_ids::text, '') AS source_care_offering_ids_json,
			COALESCE(g.source_grade_levels::text, '') AS source_grade_levels_json,
			COALESCE(g.source_school_classes::text, '') AS source_school_classes_json,
			g.list_kind,
			g.notes,
			c.shift_type_id,
			'' AS shift_type_name,
			'' AS shift_type_color,
			COALESCE(enrollments.count, 0) AS enrollment_count,
			COALESCE(supervisors.count, 0) AS supervisor_count,
			COALESCE(enrollments.student_ids, ARRAY[]::BIGINT[]) AS student_ids,
			COALESCE(supervisors.staff_ids, ARRAY[]::BIGINT[]) AS staff_ids,
			supervisors.primary_staff_id,
			s.id AS schedule_id,
			s.weekday,
			COALESCE(TO_CHAR(tf.start_time, 'HH24:MI'), '') AS start_time,
			COALESCE(TO_CHAR(tf.end_time, 'HH24:MI'), '') AS end_time,
			s.week_pattern,
			s.calendar_period_id,
			TO_CHAR(s.valid_from, 'YYYY-MM-DD') AS schedule_valid_from,
			TO_CHAR(s.valid_until, 'YYYY-MM-DD') AS schedule_valid_until
		FROM activities.groups AS g
		INNER JOIN activities.schedules AS s
			ON s.activity_group_id = g.id AND s.tenant_id = g.tenant_id
		LEFT JOIN schedule.timeframes AS tf
			ON tf.id = s.timeframe_id AND tf.tenant_id = g.tenant_id
		LEFT JOIN activities.categories AS c
			ON c.id = g.category_id AND c.tenant_id = g.tenant_id
		LEFT JOIN schedule.planning_tracks AS pt
			ON pt.id = g.planning_track_id AND pt.tenant_id = g.tenant_id
			`

// enrollmentDisplayValidityFilter is the student-row admission test shared by
// the template display-roster reads. Open rows (valid_until IS NULL) always
// count. A phase-bounded row counts only while its (exclusive) valid_until
// lies in the future AND its template is fed by a care-offering source —
// sourced rows carry a non-null valid_until by design and must still surface
// on planner cards and in editor responses (#2147 review). Every other
// bounded row — retired editor rows, split-capped predecessors, decision rows
// of the legacy 1:1 offering link — stays out of the display roster (pinned
// by TestListTemplatesEnrollmentCountIsPeriodTolerant). Requires the
// student_enrollments table to be aliased "enrollment" and consumes one
// placeholder: the exclusive valid_until boundary (today).
const enrollmentDisplayValidityFilter = `
  AND (enrollment.valid_until IS NULL OR (enrollment.valid_until > ? AND EXISTS (
    SELECT 1
    FROM activities.groups AS sourced_template
    WHERE sourced_template.id = enrollment.activity_group_id
      AND sourced_template.tenant_id = enrollment.tenant_id
      AND sourced_template.source_care_offering_ids IS NOT NULL
  )))`

// ListTemplateRows returns the template list read model, optionally filtered
// to one template (issue #584: moved verbatim from api/timetable).
//
// Student aggregates admit rows via enrollmentDisplayValidityFilter (open
// rows plus still-running offering-sourced rows). Supervisor aggregates keep
// the open-rows-only filter on purpose.
func (r *GroupRepository) ListTemplateRows(ctx context.Context, templateID *int64) ([]activities.TemplateListRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateListRow, 0)
	const query = templateListSelect + `
			LEFT JOIN (
				SELECT
					activity_group_id,
					COUNT(*) AS count,
					ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
				FROM (
					SELECT DISTINCT enrollment.activity_group_id, enrollment.student_id
					FROM activities.student_enrollments AS enrollment
					WHERE enrollment.tenant_id = ?` + enrollmentDisplayValidityFilter + `
				) AS active_enrollments
				GROUP BY activity_group_id
			) AS enrollments ON enrollments.activity_group_id = g.id
			LEFT JOIN (
			SELECT
				group_id,
					COUNT(*) AS count,
					ARRAY_AGG(staff_id ORDER BY is_primary DESC, staff_id) AS staff_ids,
					MAX(staff_id) FILTER (WHERE is_primary) AS primary_staff_id
				FROM (
					SELECT group_id, staff_id, BOOL_OR(is_primary) AS is_primary
					FROM activities.supervisors
					WHERE tenant_id = ?
					  AND valid_until IS NULL
					GROUP BY group_id, staff_id
				) AS active_supervisors
				GROUP BY group_id
			) AS supervisors ON supervisors.group_id = g.id
	WHERE g.tenant_id = ?
	  AND g.is_template = true
	  AND g.archived_at IS NULL
	  AND s.valid_until IS NULL
	  AND (?::BIGINT IS NULL OR g.id = ?)
	ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`
	if err := base.GetDB(ctx, r.db).NewRaw(
		query, tenantID, timezone.TodayDate(), tenantID, tenantID, templateID, templateID,
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, r.attachTemplateOwnerNames(ctx, rows)
}

// ListTemplateRowsForTemplatePeriod returns the editable detail read model for
// one template and calendar period. Unlike the list read, its top-level roster
// must not merge assignments from other periods because the editor writes this
// data back to the selected period. Like ListTemplateRows, the student
// aggregate admits rows via enrollmentDisplayValidityFilter.
func (r *GroupRepository) ListTemplateRowsForTemplatePeriod(
	ctx context.Context,
	templateID, periodID int64,
) ([]activities.TemplateListRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateListRow, 0)
	const query = templateListSelect + `
		LEFT JOIN (
			SELECT
				activity_group_id,
				COUNT(*) AS count,
				ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
			FROM (
				SELECT DISTINCT enrollment.activity_group_id, enrollment.student_id
				FROM activities.student_enrollments AS enrollment
				WHERE enrollment.tenant_id = ?` + enrollmentDisplayValidityFilter + `
				  AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = ?)
			) AS active_enrollments
			GROUP BY activity_group_id
		) AS enrollments ON enrollments.activity_group_id = g.id
		LEFT JOIN (
			SELECT
				group_id,
				COUNT(*) AS count,
				ARRAY_AGG(staff_id ORDER BY is_primary DESC, primary_rank DESC, staff_id) AS staff_ids,
				(ARRAY_AGG(staff_id ORDER BY primary_rank DESC, staff_id)
					FILTER (WHERE is_primary))[1] AS primary_staff_id
			FROM (
				SELECT
					group_id,
					staff_id,
					BOOL_OR(is_primary) AS is_primary,
					MAX(CASE
						WHEN is_primary THEN
							CASE WHEN calendar_period_id = ? THEN 2 ELSE 0 END
							+ CASE WHEN weekday IS NOT NULL THEN 1 ELSE 0 END
						ELSE -1
					END) AS primary_rank
				FROM activities.supervisors
				WHERE tenant_id = ?
				  AND valid_until IS NULL
				  AND (calendar_period_id IS NULL OR calendar_period_id = ?)
				GROUP BY group_id, staff_id
			) AS active_supervisors
			GROUP BY group_id
		) AS supervisors ON supervisors.group_id = g.id
	WHERE g.tenant_id = ?
	  AND g.is_template = TRUE
	  AND g.archived_at IS NULL
	  AND s.valid_until IS NULL
	  AND (
		s.calendar_period_id = ?
		OR (
			s.calendar_period_id IS NULL
			AND (g.calendar_period_id = ? OR g.calendar_period_id IS NULL)
		)
	  )
	  AND g.id = ?
	ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`
	if err := base.GetDB(ctx, r.db).NewRaw(
		query, tenantID, timezone.TodayDate(), periodID,
		periodID, tenantID, periodID,
		tenantID, periodID, periodID, templateID,
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, r.attachTemplateOwnerNames(ctx, rows)
}

// ListTemplateWeekdayRoster returns the weekday-scoped roster memberships of
// the open roster of every non-archived template (issue #2129). Staff, students,
// and empty-day markers come back as one flat, kind-tagged stream so the caller
// can group them into per-weekday assignments in Go instead of building a
// second jsonb-aggregating monster query.
//
// The validity filters mirror the flat aggregates in templateListSelect: the
// editor reads the current roster, not the historical retired rows. Student
// rows are admitted via enrollmentDisplayValidityFilter (open rows, plus
// still-running phase-bounded rows on offering-sourced templates, which must
// surface as protected assignments — #2147 review); supervisor rows keep the
// open-rows-only filter. A nil calendarPeriodID
// selects only unscoped rows; it must never mean "all periods", because the
// response has no period field with which the caller could separate those
// rosters again.
const templateWeekdayRosterQuery = `
		WITH parameters AS (
			SELECT ?::BIGINT AS tenant_id, ?::BIGINT AS period_id, ?::BIGINT AS template_id
		), per_weekday_templates AS (
			SELECT supervisor.group_id AS template_id
			FROM activities.supervisors AS supervisor, parameters
			WHERE supervisor.tenant_id = parameters.tenant_id
			  AND supervisor.valid_until IS NULL AND supervisor.weekday IS NOT NULL
			  AND ((parameters.period_id IS NULL AND supervisor.calendar_period_id IS NULL)
			       OR (parameters.period_id IS NOT NULL AND
			           (supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = parameters.period_id)))
			GROUP BY supervisor.group_id
			UNION
			SELECT enrollment.activity_group_id AS template_id
			FROM activities.student_enrollments AS enrollment, parameters
			WHERE enrollment.tenant_id = parameters.tenant_id` + enrollmentDisplayValidityFilter + `
			  AND enrollment.weekday IS NOT NULL
			  AND ((parameters.period_id IS NULL AND enrollment.calendar_period_id IS NULL)
			       OR (parameters.period_id IS NOT NULL AND
			           (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = parameters.period_id)))
			GROUP BY enrollment.activity_group_id
		), scheduled_template_weekdays AS (
			SELECT activity_group.id AS template_id, schedule.weekday
			FROM activities.groups AS activity_group
			JOIN activities.schedules AS schedule
			  ON schedule.activity_group_id = activity_group.id AND schedule.tenant_id = activity_group.tenant_id
			CROSS JOIN parameters
			WHERE activity_group.tenant_id = parameters.tenant_id
			  AND activity_group.is_template = TRUE AND activity_group.archived_at IS NULL
			  AND schedule.valid_until IS NULL
			  AND (parameters.period_id IS NULL OR schedule.calendar_period_id = parameters.period_id
			       OR (schedule.calendar_period_id IS NULL AND
			           (activity_group.calendar_period_id = parameters.period_id OR activity_group.calendar_period_id IS NULL)))
			  AND (parameters.template_id IS NULL OR activity_group.id = parameters.template_id)
			GROUP BY activity_group.id, schedule.weekday
		), template_weekdays AS (
			SELECT scheduled.template_id, scheduled.weekday
			FROM scheduled_template_weekdays AS scheduled
			JOIN per_weekday_templates AS scoped ON scoped.template_id = scheduled.template_id
		), effective_primary_staff AS (
			SELECT template_day.template_id, template_day.weekday, primary_staff.staff_id
			FROM template_weekdays AS template_day
			CROSS JOIN parameters
			LEFT JOIN LATERAL (
				SELECT candidate.staff_id
				FROM activities.supervisors AS candidate
				WHERE candidate.tenant_id = parameters.tenant_id
				  AND candidate.group_id = template_day.template_id AND candidate.valid_until IS NULL
				  AND ((parameters.period_id IS NULL AND candidate.calendar_period_id IS NULL)
				       OR (parameters.period_id IS NOT NULL AND
				           (candidate.calendar_period_id IS NULL OR candidate.calendar_period_id = parameters.period_id)))
				  AND (candidate.weekday IS NULL OR candidate.weekday = template_day.weekday)
				  AND candidate.is_primary
				ORDER BY (candidate.calendar_period_id IS NOT NULL) DESC,
				         (candidate.weekday IS NOT NULL) DESC, candidate.id DESC
				LIMIT 1
			) AS primary_staff ON TRUE
		)
		SELECT template_day.template_id, template_day.weekday, 'empty' AS kind,
		       0 AS person_id, FALSE AS is_primary
		FROM template_weekdays AS template_day
		UNION ALL
		SELECT supervisor.group_id, template_day.weekday, 'staff', supervisor.staff_id,
		       COALESCE(BOOL_OR(supervisor.staff_id = primary_staff.staff_id), FALSE)
		FROM activities.supervisors AS supervisor
		JOIN template_weekdays AS template_day
		  ON template_day.template_id = supervisor.group_id
		 AND (supervisor.weekday IS NULL OR template_day.weekday = supervisor.weekday)
		JOIN effective_primary_staff AS primary_staff
		  ON primary_staff.template_id = template_day.template_id AND primary_staff.weekday = template_day.weekday
		CROSS JOIN parameters
		WHERE supervisor.tenant_id = parameters.tenant_id AND supervisor.valid_until IS NULL
		  AND ((parameters.period_id IS NULL AND supervisor.calendar_period_id IS NULL)
		       OR (parameters.period_id IS NOT NULL AND
		           (supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = parameters.period_id)))
		GROUP BY supervisor.group_id, template_day.weekday, supervisor.staff_id
		UNION ALL
		SELECT enrollment.activity_group_id, template_day.weekday, 'student', enrollment.student_id, FALSE
		FROM activities.student_enrollments AS enrollment
		JOIN template_weekdays AS template_day
		  ON template_day.template_id = enrollment.activity_group_id
		 AND (enrollment.weekday IS NULL OR template_day.weekday = enrollment.weekday)
		CROSS JOIN parameters
		WHERE enrollment.tenant_id = parameters.tenant_id` + enrollmentDisplayValidityFilter + `
		  AND ((parameters.period_id IS NULL AND enrollment.calendar_period_id IS NULL)
		       OR (parameters.period_id IS NOT NULL AND
		           (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = parameters.period_id)))
		  AND (enrollment.selected_weekdays IS NULL OR jsonb_array_length(enrollment.selected_weekdays) = 0
		       OR enrollment.selected_weekdays @> jsonb_build_array(template_day.weekday))
		GROUP BY enrollment.activity_group_id, template_day.weekday, enrollment.student_id
		UNION ALL
		SELECT enrollment.activity_group_id, template_day.weekday, 'protected_student', enrollment.student_id, FALSE
		FROM activities.student_enrollments AS enrollment
		JOIN scheduled_template_weekdays AS template_day ON template_day.template_id = enrollment.activity_group_id
		CROSS JOIN parameters
		WHERE enrollment.tenant_id = parameters.tenant_id` + enrollmentDisplayValidityFilter + `
		  AND ((parameters.period_id IS NULL AND enrollment.calendar_period_id IS NULL)
		       OR (parameters.period_id IS NOT NULL AND
		           (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = parameters.period_id)))
		  AND (enrollment.enrollment_request_child_id IS NOT NULL
		       OR COALESCE(jsonb_array_length(enrollment.selected_weekdays), 0) > 0)
		  AND (enrollment.weekday IS NULL OR enrollment.weekday = template_day.weekday)
		  AND (enrollment.selected_weekdays IS NULL OR jsonb_array_length(enrollment.selected_weekdays) = 0
		       OR enrollment.selected_weekdays @> jsonb_build_array(template_day.weekday))
		GROUP BY enrollment.activity_group_id, template_day.weekday, enrollment.student_id
		ORDER BY template_id ASC, weekday ASC, kind ASC, is_primary DESC, person_id ASC`

func (r *GroupRepository) ListTemplateWeekdayRoster(
	ctx context.Context,
	templateID, calendarPeriodID *int64,
) ([]activities.TemplateWeekdayRosterRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateWeekdayRosterRow, 0)
	if err := base.GetDB(ctx, r.db).NewRaw(
		templateWeekdayRosterQuery, tenantID, calendarPeriodID, templateID,
		timezone.TodayDate(), timezone.TodayDate(), timezone.TodayDate(),
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListTemplateRowsForPeriod is the calendar-period-filtered template list
// (issue #584: moved verbatim from api/timetable).
func (r *GroupRepository) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activities.TemplateListRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateListRow, 0)
	const query = templateListSelect + `
			LEFT JOIN (
				SELECT
					activity_group_id,
					COUNT(*) AS count,
					ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
				FROM (
					SELECT DISTINCT enrollment.activity_group_id, enrollment.student_id
					FROM activities.student_enrollments AS enrollment
					WHERE enrollment.tenant_id = ?` + enrollmentDisplayValidityFilter + `
				) AS active_enrollments
				GROUP BY activity_group_id
			) AS enrollments ON enrollments.activity_group_id = g.id
		LEFT JOIN (
			SELECT
				group_id,
					COUNT(*) AS count,
					ARRAY_AGG(staff_id ORDER BY is_primary DESC, staff_id) AS staff_ids,
					MAX(staff_id) FILTER (WHERE is_primary) AS primary_staff_id
				FROM (
					SELECT group_id, staff_id, BOOL_OR(is_primary) AS is_primary
					FROM activities.supervisors
					WHERE tenant_id = ?
					  AND valid_until IS NULL
					GROUP BY group_id, staff_id
				) AS active_supervisors
				GROUP BY group_id
			) AS supervisors ON supervisors.group_id = g.id
	WHERE g.tenant_id = ?
	  AND g.is_template = true
	  AND g.archived_at IS NULL
	  AND s.valid_until IS NULL
	  AND (?::BIGINT IS NULL OR (
		s.calendar_period_id = ?
		OR (s.calendar_period_id IS NULL
			AND (g.calendar_period_id = ? OR g.calendar_period_id IS NULL))
	  ))
	ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`

	if err := base.GetDB(ctx, r.db).NewRaw(
		query, tenantID, timezone.TodayDate(), tenantID, tenantID, periodID, periodID, periodID,
	).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, r.attachTemplateOwnerNames(ctx, rows)
}

func (r *GroupRepository) attachTemplateOwnerNames(ctx context.Context, rows []activities.TemplateListRow) error {
	roomNames, err := r.templateRoomNames(ctx, rows)
	if err != nil {
		return err
	}
	shiftTypes, err := r.templateShiftTypes(ctx)
	if err != nil {
		return err
	}
	for index := range rows {
		if rows[index].RoomID.Valid {
			rows[index].RoomName.String, rows[index].RoomName.Valid = roomNames[rows[index].RoomID.Int64], true
		}
		if rows[index].ShiftTypeID.Valid {
			shift := shiftTypes[rows[index].ShiftTypeID.Int64]
			rows[index].ShiftTypeName, rows[index].ShiftTypeColor = shift.Name, shift.Color
		}
	}
	return nil
}

func (r *GroupRepository) templateRoomNames(ctx context.Context, rows []activities.TemplateListRow) (map[int64]string, error) {
	if r.rooms == nil {
		return nil, errors.New("activities repositories: template room directory is not bound")
	}
	ids := make([]int64, 0)
	for _, row := range rows {
		if row.RoomID.Valid {
			ids = append(ids, row.RoomID.Int64)
		}
	}
	rooms, err := r.rooms.ListRoomsByID(ctx, dedupeInt64(ids))
	result := make(map[int64]string, len(rooms))
	for _, room := range rooms {
		result[room.ID] = room.Name
	}
	return result, err
}

func dedupeInt64(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func (r *GroupRepository) templateShiftTypes(ctx context.Context) (map[int64]TemplateShiftType, error) {
	if r.shifts == nil {
		return nil, errors.New("activities repositories: template shift type directory is not bound")
	}
	rows, err := r.shifts.ListShiftTypes(ctx)
	result := make(map[int64]TemplateShiftType, len(rows))
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, err
}

// ListTemplateCapacityOccurrences returns one row for every date on which at
// least one schedule of a template actually recurs. Keeping the repository
// result date-granular is essential: the tenant-specific children/staff ratio
// is business logic, and combining non-concurrent rosters here would erase the
// evidence the service needs to choose the real worst occurrence.
const templateCapacityOccurrencesQuery = `
		WITH active_periods AS (
			SELECT
				period.calendar_period_id,
				period.start_date,
				period.end_date,
				period.week_cycle_length,
				NULLIF(period.week_cycle_anchor, '')::DATE AS week_cycle_anchor
			FROM unnest(?::BIGINT[], ?::DATE[], ?::DATE[], ?::INT[], ?::TEXT[])
				AS period(calendar_period_id, start_date, end_date, week_cycle_length, week_cycle_anchor)
		), selected_period AS (
			SELECT calendar_period_id, start_date, end_date, week_cycle_length, week_cycle_anchor
			FROM active_periods
			WHERE (?::BIGINT IS NULL OR calendar_period_id = ?)
		), candidate_occurrences AS (
			SELECT DISTINCT
				g.id AS template_id,
				period.calendar_period_id,
				days.day::DATE AS occurrence_date
			FROM activities.groups AS g
			INNER JOIN activities.schedules AS s
				ON s.activity_group_id = g.id
				AND s.tenant_id = g.tenant_id
			INNER JOIN schedule.timeframes AS timeframe
				ON timeframe.id = s.timeframe_id
				AND timeframe.tenant_id = g.tenant_id
				AND timeframe.start_time IS NOT NULL
				AND timeframe.end_time IS NOT NULL
			CROSS JOIN selected_period AS period
			CROSS JOIN LATERAL generate_series(
				period.start_date,
				period.end_date,
				INTERVAL '1 day'
			) AS days(day)
			LEFT JOIN schedule.activity_exceptions AS exception
				ON exception.tenant_id = g.tenant_id
				AND exception.activity_group_id = g.id
				AND exception.exception_date = days.day::DATE
			WHERE g.tenant_id = ?
			  AND g.id IN (?)
			  AND g.is_template = TRUE
			  AND g.archived_at IS NULL
			  AND (exception.exception_type IS NULL OR exception.exception_type <> 'cancelled')
			  AND COALESCE(exception.room_id, g.planned_room_id, 0) > 0
			  AND DATE_PART('isodow', days.day)::INT = s.weekday
			  AND (s.valid_from IS NULL OR s.valid_from <= days.day::DATE)
			  AND (s.valid_until IS NULL OR s.valid_until > days.day::DATE)
			  AND (
				s.calendar_period_id = period.calendar_period_id
				OR (
					s.calendar_period_id IS NULL
					AND g.calendar_period_id = period.calendar_period_id
				)
				OR (
					s.calendar_period_id IS NULL
					AND g.calendar_period_id IS NULL
					AND period.calendar_period_id = (
						SELECT MIN(active_period.calendar_period_id)
						FROM active_periods AS active_period
						WHERE active_period.start_date <= days.day::DATE
						  AND active_period.end_date >= days.day::DATE
					)
				)
			  )
			  AND (
				s.week_pattern = 0
				OR period.week_cycle_length <= 1
				OR period.week_cycle_anchor IS NULL
				OR s.week_pattern = (
					MOD(
						MOD(
							FLOOR((days.day::DATE - period.week_cycle_anchor) / 7.0)::INT,
							period.week_cycle_length
						) + period.week_cycle_length,
						period.week_cycle_length
					) + 1
				)
			  )
		), dynamic_target_students AS (
			SELECT dynamic.template_id, dynamic.student_id
			FROM unnest(?::BIGINT[], ?::BIGINT[]) AS dynamic(template_id, student_id)
		), capacity_parts AS (
			SELECT
				occurrence.template_id,
				occurrence.calendar_period_id,
				occurrence.occurrence_date,
				COUNT(DISTINCT roster.student_id)::INT AS enrollment_count,
				0::INT AS supervisor_count
			FROM candidate_occurrences AS occurrence
			CROSS JOIN LATERAL (
				SELECT enrollment.student_id
				FROM activities.student_enrollments AS enrollment
				WHERE enrollment.tenant_id = ?
					AND enrollment.activity_group_id = occurrence.template_id
					AND enrollment.valid_from <= occurrence.occurrence_date
					AND (enrollment.valid_until IS NULL OR enrollment.valid_until > occurrence.occurrence_date)
					AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = occurrence.calendar_period_id)
					AND (
						enrollment.selected_weekdays IS NULL
						OR jsonb_array_length(enrollment.selected_weekdays) = 0
						OR enrollment.selected_weekdays @> jsonb_build_array(DATE_PART('isodow', occurrence.occurrence_date)::INT)
					)
					AND (
						enrollment.weekday IS NULL
						OR enrollment.weekday = DATE_PART('isodow', occurrence.occurrence_date)::INT
					)
				UNION
				SELECT dynamic.student_id
				FROM dynamic_target_students AS dynamic
				WHERE dynamic.template_id = occurrence.template_id
			) AS roster
			GROUP BY occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date

			UNION ALL

			SELECT
				occurrence.template_id,
				occurrence.calendar_period_id,
				occurrence.occurrence_date,
				0::INT AS enrollment_count,
				COUNT(DISTINCT supervisor.staff_id)::INT AS supervisor_count
			FROM candidate_occurrences AS occurrence
			INNER JOIN activities.supervisors AS supervisor
				ON supervisor.tenant_id = ?
				AND supervisor.group_id = occurrence.template_id
				AND supervisor.valid_from <= occurrence.occurrence_date
				AND (supervisor.valid_until IS NULL OR supervisor.valid_until > occurrence.occurrence_date)
				AND (supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = occurrence.calendar_period_id)
				AND (
					supervisor.weekday IS NULL
					OR supervisor.weekday = DATE_PART('isodow', occurrence.occurrence_date)::INT
				)
			GROUP BY occurrence.template_id, occurrence.calendar_period_id, occurrence.occurrence_date

			UNION ALL

			-- Preserve occurrences with no matching roster rows. Keeping the
			-- student and staff aggregates in UNION branches avoids joining two
			-- severely underestimated aggregate CTEs: PostgreSQL otherwise
			-- rescans them through nested loops for every generated date.
			SELECT
				occurrence.template_id,
				occurrence.calendar_period_id,
				occurrence.occurrence_date,
				0::INT AS enrollment_count,
				0::INT AS supervisor_count
			FROM candidate_occurrences AS occurrence
		)
		-- Each aggregate branch populates one metric and zeroes the other;
		-- MAX recombines them without multiplying student and staff rows.
		SELECT
			template_id,
			calendar_period_id,
			occurrence_date,
			MAX(enrollment_count) AS enrollment_count,
			MAX(supervisor_count) AS supervisor_count
		FROM capacity_parts
		GROUP BY template_id, calendar_period_id, occurrence_date
		ORDER BY template_id ASC, occurrence_date ASC, calendar_period_id ASC`

func (r *GroupRepository) ListTemplateCapacityOccurrences(
	ctx context.Context,
	periodID *int64,
	templateIDs []int64,
) ([]activities.TemplateCapacityOccurrence, error) {
	if len(templateIDs) == 0 {
		return []activities.TemplateCapacityOccurrence{}, nil
	}
	dynamicTemplateIDs, dynamicStudentIDs, err := r.dynamicTargetPairs(ctx, templateIDs)
	if err != nil {
		return nil, err
	}
	if r.periods == nil {
		return nil, errCalendarPeriodSourceRequired
	}
	activePeriods, err := r.periods.ListActiveCalendarPeriods(ctx)
	if err != nil {
		return nil, err
	}
	tenantID := tenant.FromContext(ctx)
	tenantPeriods := activePeriods[:0:0]
	for _, period := range activePeriods {
		if period.TenantID == tenantID {
			tenantPeriods = append(tenantPeriods, period)
		}
	}
	periodIDs, periodStarts, periodEnds, periodCycleLengths, periodAnchors := capacityPeriodColumns(tenantPeriods)
	occurrences := make([]activities.TemplateCapacityOccurrence, 0)
	err = base.GetDB(ctx, r.db).NewRaw(
		templateCapacityOccurrencesQuery,
		pgdialect.Array(periodIDs), pgdialect.Array(periodStarts), pgdialect.Array(periodEnds),
		pgdialect.Array(periodCycleLengths), pgdialect.Array(periodAnchors),
		periodID, periodID,
		tenantID, bun.List(templateIDs),
		pgdialect.Array(dynamicTemplateIDs), pgdialect.Array(dynamicStudentIDs), tenantID,
		tenantID,
	).Scan(ctx, &occurrences)
	if err != nil {
		return nil, err
	}
	return occurrences, nil
}

// dynamicTargetPairs resolves the dynamic target cohorts of the templates
// through the People Directory (#2662) as parallel (template, student)
// arrays for the capacity query. Unlike the timetable roster, capacity
// counts every non-alumni child a rule matches regardless of the care end
// date, exactly as the former SQL join did.
func (r *GroupRepository) dynamicTargetPairs(ctx context.Context, templateIDs []int64) ([]int64, []int64, error) {
	if r.students == nil {
		return nil, nil, errStudentDirectoryRequired
	}
	targets, err := r.FindTargetsByGroupIDs(ctx, templateIDs)
	if err != nil {
		return nil, nil, err
	}
	templates, students := []int64{}, []int64{}
	if len(targets) == 0 {
		return templates, students, nil
	}
	enrolled, err := r.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, nil, err
	}
	matches := matchTargetStudents(targets, enrolled, nil)
	for _, templateID := range templateIDs {
		for _, studentID := range matches[templateID] {
			templates = append(templates, templateID)
			students = append(students, studentID)
		}
	}
	return templates, students, nil
}

// UpdateTemplateFields patches the editable fields of a non-archived template
// (issue #584: moved verbatim from api/timetable; extended for
// calendar_period_id/Zielgruppe in issue #1838).
func (r *GroupRepository) UpdateTemplateFields(ctx context.Context, id int64, fields activities.TemplateFieldsUpdate) (int64, error) {
	tenantID := tenant.FromContext(ctx)

	// This is a raw column Set(), which bypasses bun's Model()-based
	// zero-value-to-DEFAULT omission (the mechanism that lets
	// activities.Group{} literals elsewhere in the codebase skip
	// TargetGroupType entirely and still satisfy the DB CHECK constraint via
	// its 'none' default). An empty string here must be normalized
	// explicitly, or it is sent as literal '' and violates
	// chk_activities_groups_target_group_type.
	targetGroupType := fields.TargetGroupType
	if targetGroupType == "" {
		targetGroupType = activities.TargetGroupTypeNone
	}

	// jsonb columns via raw Set(): bind the JSON text (PostgreSQL casts the
	// parameter to the column type); nil keeps the column NULL. A grade
	// filter without a source is normalized away to satisfy
	// chk_activities_groups_offering_source.
	var sourceCareOfferingIDs any
	if len(fields.SourceCareOfferingIDs) > 0 {
		encoded, err := json.Marshal(fields.SourceCareOfferingIDs)
		if err != nil {
			return 0, fmt.Errorf("marshal source_care_offering_ids: %w", err)
		}
		sourceCareOfferingIDs = string(encoded)
	}
	var sourceGradeLevels any
	if len(fields.SourceCareOfferingIDs) > 0 && len(fields.SourceGradeLevels) > 0 {
		encoded, err := json.Marshal(fields.SourceGradeLevels)
		if err != nil {
			return 0, fmt.Errorf("marshal source_grade_levels: %w", err)
		}
		sourceGradeLevels = string(encoded)
	}
	// Written through as given, like the grade filter above: the two are
	// mutually exclusive, but silently dropping one here would hide a caller
	// that skipped validation. chk_activities_groups_offering_source rejects
	// the pair (#2482).
	var sourceSchoolClasses any
	if len(fields.SourceCareOfferingIDs) > 0 && len(fields.SourceSchoolClasses) > 0 {
		encoded, err := json.Marshal(fields.SourceSchoolClasses)
		if err != nil {
			return 0, fmt.Errorf("marshal source_school_classes: %w", err)
		}
		sourceSchoolClasses = string(encoded)
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.groups").
		Set("name = ?", fields.Name).
		Set("type = ?", fields.Type).
		Set("category_id = ?", fields.CategoryID).
		Set("planned_room_id = ?", fields.RoomID).
		Set("education_group_id = ?", fields.EducationGroupID).
		Set("required_staff = ?", fields.RequiredStaff).
		Set("calendar_period_id = ?", fields.CalendarPeriodID).
		Set("target_group_type = ?", targetGroupType).
		Set("target_grade_level = ?", fields.TargetGradeLevel).
		Set("target_school_class = ?", fields.TargetSchoolClass).
		Set("source_care_offering_ids = ?", sourceCareOfferingIDs).
		Set("source_grade_levels = ?", sourceGradeLevels).
		Set("source_school_classes = ?", sourceSchoolClasses).
		Set("list_kind = ?", fields.ListKind).
		Set("notes = ?", fields.Notes).
		Set("updated_at = ?", time.Now()).
		Where("tenant_id = ?", tenantID).
		Where("id = ?", id).
		Where("is_template = true").
		Where("archived_at IS NULL")
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
	res, err := query.Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ArchiveTemplate soft-deletes a non-archived template (issue #584: moved
// verbatim from api/timetable).
func (r *GroupRepository) ArchiveTemplate(ctx context.Context, id int64) (int64, error) {
	tenantID := tenant.FromContext(ctx)
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.groups").
		Set("archived_at = ?", time.Now()).
		Set("updated_at = ?", time.Now()).
		Where("tenant_id = ?", tenantID).
		Where("id = ?", id).
		Where("is_template = true").
		Where("archived_at IS NULL").
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
