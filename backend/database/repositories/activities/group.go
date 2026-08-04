// backend/database/repositories/activities/group.go
package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableActivitiesGroups          = "activities.groups"
	tableExprActivitiesGroupsAsGrp = `activities.groups AS "group"`
	orderByNameAsc                 = "name ASC"
	whereIDEquals                  = "id = ?"
)

// GroupRepository implements activities.GroupRepository interface
type GroupRepository struct {
	*base.Repository[*activities.Group]
	db *bun.DB
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
			Err: err,
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
			Err: err,
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
		return nil, &modelBase.DatabaseError{Op: "find template series", Err: err}
	}
	return groups, nil
}

// FindTemplatesBySourceOffering returns every live template sourced from the
// given care offering (#2137).
func (r *GroupRepository) FindTemplatesBySourceOffering(ctx context.Context, offeringID int64) ([]*activities.Group, error) {
	tenantID := tenant.FromContext(ctx)
	groups := make([]*activities.Group, 0)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&groups).
		ModelTableExpr(tableExprActivitiesGroupsAsGrp).
		Where(`"group".tenant_id = ?`, tenantID).
		Where(`"group".is_template = TRUE`).
		Where(`"group".archived_at IS NULL`).
		Where(`"group".source_care_offering_id = ?`, offeringID).
		OrderExpr(`"group".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find templates by source offering", Err: err}
	}
	return groups, nil
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
		Where(`"group".source_care_offering_id IS NOT NULL`).
		OrderExpr(`"group".id ASC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find templates with offering source", Err: err}
	}
	return groups, nil
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
			Err: err,
		}
	}

	return groups, nil
}

// CountByCategory returns the number of activity groups per category id for
// the current tenant. Archived templates are counted too: they still reference
// their category and are restorable, so a category backing only archived rows
// is "in use" for the purpose of the archive warning (#2131).
func (r *GroupRepository) CountByCategory(ctx context.Context) (map[int64]int, error) {
	var rows []struct {
		CategoryID int64 `bun:"category_id"`
		Total      int   `bun:"total"`
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(tableExprActivitiesGroupsAsGrp).
		ColumnExpr(`"group".category_id AS category_id`).
		ColumnExpr(`COUNT(*) AS total`).
		GroupExpr(`"group".category_id`)

	query = base.WithTenantFilter(ctx, query, "group")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "count by category",
			Err: err,
		}
	}

	counts := make(map[int64]int, len(rows))
	for _, row := range rows {
		counts[row.CategoryID] = row.Total
	}

	return counts, nil
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
			Err: err,
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
			Err: err,
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
			Err: err,
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
			Err: err,
		}
	}

	// Convert to map
	countMap := make(map[int64]int)
	for _, count := range counts {
		countMap[count.GroupID] = count.Count
	}

	return groups, countMap, nil
}

// loadStaffWithPerson loads staff and person relations for a supervisor
func (r *GroupRepository) loadStaffWithPerson(ctx context.Context, sup *activities.SupervisorPlanned) {
	if sup.StaffID <= 0 {
		return
	}

	staff := new(users.Staff)
	staffErr := base.GetDB(ctx, r.db).NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(whereIDEquals, sup.StaffID).
		Scan(ctx)

	if staffErr != nil {
		return
	}

	sup.Staff = staff
	if staff.PersonID <= 0 {
		return
	}

	person := new(users.Person)
	personErr := base.GetDB(ctx, r.db).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(whereIDEquals, staff.PersonID).
		Scan(ctx)

	if personErr == nil {
		staff.Person = person
	}
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
			Err: err,
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
			Err: err,
		}
	}

	// Load Staff and Person relations for each supervisor
	for _, sup := range supervisors {
		r.loadStaffWithPerson(ctx, sup)
	}

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
			Err: err,
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
			Err: err,
		}
	}

	return base.AssertRowsAffected(result, 1, "update group")
}

// List overrides the base List method to accept the new QueryOptions type
func (r *GroupRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activities.Group, error) {
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

	// Apply query options with table alias to avoid ambiguous column references
	// (both "group" and "category" have "id" columns)
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("group")
		}
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
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
				COALESCE(r.name, '') AS room_name,
				g.education_group_id,
				COALESCE(eg.name, '') AS education_group_name,
				g.is_open,
			g.max_participants,
			g.required_staff,
			g.calendar_period_id AS template_calendar_period_id,
			g.target_group_type,
			g.target_grade_level,
			g.target_school_class,
			g.source_care_offering_id,
			COALESCE(g.source_grade_levels::text, '') AS source_grade_levels_json,
			g.list_kind,
			g.notes,
			COALESCE(st.name, '') AS shift_type_name,
			COALESCE(st.color, '') AS shift_type_color,
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
			LEFT JOIN schedule.shift_types AS st
				ON st.id = c.shift_type_id AND st.tenant_id = g.tenant_id
			LEFT JOIN facilities.rooms AS r
				ON r.id = g.planned_room_id AND r.tenant_id = g.tenant_id
			LEFT JOIN education.groups AS eg
				ON eg.id = g.education_group_id AND eg.tenant_id = g.tenant_id`

// ListTemplateRows returns the template list read model, optionally filtered
// to one template (issue #584: moved verbatim from api/timetable).
func (r *GroupRepository) ListTemplateRows(ctx context.Context, templateID *int64) ([]activities.TemplateListRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateListRow, 0)
	query := templateListSelect + `
			LEFT JOIN (
				SELECT
					activity_group_id,
					COUNT(*) AS count,
					ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
				FROM (
					SELECT DISTINCT activity_group_id, student_id
					FROM activities.student_enrollments
					WHERE tenant_id = ?
					  AND valid_until IS NULL
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
	  AND s.valid_until IS NULL`
	args := []any{tenantID, tenantID, tenantID}
	if templateID != nil {
		query += ` AND g.id = ?`
		args = append(args, *templateID)
	}
	query += ` ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`
	if err := base.GetDB(ctx, r.db).NewRaw(query, args...).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListTemplateRowsForTemplatePeriod returns the editable detail read model for
// one template and calendar period. Unlike the list read, its top-level roster
// must not merge assignments from other periods because the editor writes this
// data back to the selected period.
func (r *GroupRepository) ListTemplateRowsForTemplatePeriod(
	ctx context.Context,
	templateID, periodID int64,
) ([]activities.TemplateListRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateListRow, 0)
	query := templateListSelect + `
		LEFT JOIN (
			SELECT
				activity_group_id,
				COUNT(*) AS count,
				ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
			FROM (
				SELECT DISTINCT activity_group_id, student_id
				FROM activities.student_enrollments
				WHERE tenant_id = ?
				  AND valid_until IS NULL
				  AND (calendar_period_id IS NULL OR calendar_period_id = ?)
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
	args := []any{
		tenantID, periodID,
		periodID, tenantID, periodID,
		tenantID, periodID, periodID, templateID,
	}
	if err := base.GetDB(ctx, r.db).NewRaw(query, args...).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListTemplateWeekdayRoster returns the weekday-scoped roster memberships of
// the open roster of every non-archived template (issue #2129). Staff, students,
// and empty-day markers come back as one flat, kind-tagged stream so the caller
// can group them into per-weekday assignments in Go instead of building a
// second jsonb-aggregating monster query.
//
// The `valid_until IS NULL` filter mirrors the flat aggregates in
// templateListSelect: the editor reads the currently open roster, not the
// historical retired rows. A nil calendarPeriodID selects only unscoped rows;
// it must never mean "all periods", because the response has no period field
// with which the caller could separate those rosters again.
func (r *GroupRepository) ListTemplateWeekdayRoster(
	ctx context.Context,
	templateID, calendarPeriodID *int64,
) ([]activities.TemplateWeekdayRosterRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateWeekdayRosterRow, 0)
	query := `
		WITH per_weekday_templates AS (
			SELECT supervisor.group_id AS template_id
			FROM activities.supervisors AS supervisor
			WHERE supervisor.tenant_id = ?
			  AND supervisor.valid_until IS NULL
			  AND supervisor.weekday IS NOT NULL
	`
	args := []any{tenantID}
	if calendarPeriodID != nil {
		query += ` AND (supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = ?)`
		args = append(args, *calendarPeriodID)
	} else {
		query += ` AND supervisor.calendar_period_id IS NULL`
	}
	query += `
			GROUP BY supervisor.group_id
			UNION
			SELECT enrollment.activity_group_id AS template_id
			FROM activities.student_enrollments AS enrollment
			WHERE enrollment.tenant_id = ?
			  AND enrollment.valid_until IS NULL
			  AND enrollment.weekday IS NOT NULL
	`
	args = append(args, tenantID)
	if calendarPeriodID != nil {
		query += ` AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = ?)`
		args = append(args, *calendarPeriodID)
	} else {
		query += ` AND enrollment.calendar_period_id IS NULL`
	}
	query += `
			GROUP BY enrollment.activity_group_id
		), scheduled_template_weekdays AS (
			SELECT g.id AS template_id, schedule.weekday
			FROM activities.groups AS g
			INNER JOIN activities.schedules AS schedule
				ON schedule.activity_group_id = g.id
				AND schedule.tenant_id = g.tenant_id
			WHERE g.tenant_id = ?
			  AND g.is_template = TRUE
			  AND g.archived_at IS NULL
			  AND schedule.valid_until IS NULL`
	args = append(args, tenantID)
	if calendarPeriodID != nil {
		query += ` AND (
				schedule.calendar_period_id = ?
				OR (
					schedule.calendar_period_id IS NULL
					AND (g.calendar_period_id = ? OR g.calendar_period_id IS NULL)
				)
			)`
		args = append(args, *calendarPeriodID, *calendarPeriodID)
	}
	if templateID != nil {
		query += ` AND g.id = ?`
		args = append(args, *templateID)
	}
	query += `
			GROUP BY g.id, schedule.weekday
		), template_weekdays AS (
			SELECT scheduled.template_id, scheduled.weekday
			FROM scheduled_template_weekdays AS scheduled
			INNER JOIN per_weekday_templates AS scoped
				ON scoped.template_id = scheduled.template_id
		), effective_primary_staff AS (
			SELECT
				template_day.template_id,
				template_day.weekday,
				primary_staff.staff_id
			FROM template_weekdays AS template_day
			LEFT JOIN LATERAL (
				SELECT candidate.staff_id
				FROM activities.supervisors AS candidate
				WHERE candidate.tenant_id = ?
				  AND candidate.group_id = template_day.template_id
				  AND candidate.valid_until IS NULL`
	args = append(args, tenantID)
	if calendarPeriodID != nil {
		query += `
				  AND (candidate.calendar_period_id IS NULL OR candidate.calendar_period_id = ?)`
		args = append(args, *calendarPeriodID)
	} else {
		query += `
				  AND candidate.calendar_period_id IS NULL`
	}
	query += `
				  AND (candidate.weekday IS NULL OR candidate.weekday = template_day.weekday)
				  AND candidate.is_primary
				ORDER BY
					(candidate.calendar_period_id IS NOT NULL) DESC,
					(candidate.weekday IS NOT NULL) DESC,
					candidate.id DESC
				LIMIT 1
			) AS primary_staff ON TRUE
		)
		SELECT
			template_day.template_id,
			template_day.weekday,
			'empty' AS kind,
			0 AS person_id,
			FALSE AS is_primary
		FROM template_weekdays AS template_day
		UNION ALL
		SELECT
			supervisor.group_id AS template_id,
			template_day.weekday,
			'staff' AS kind,
			supervisor.staff_id AS person_id,
			COALESCE(BOOL_OR(supervisor.staff_id = effective_primary.staff_id), FALSE) AS is_primary
		FROM activities.supervisors AS supervisor
		INNER JOIN template_weekdays AS template_day
			ON template_day.template_id = supervisor.group_id
			AND (supervisor.weekday IS NULL OR template_day.weekday = supervisor.weekday)
		INNER JOIN effective_primary_staff AS effective_primary
			ON effective_primary.template_id = template_day.template_id
			AND effective_primary.weekday = template_day.weekday
		WHERE supervisor.tenant_id = ?
		  AND supervisor.valid_until IS NULL`
	args = append(args, tenantID)
	if calendarPeriodID != nil {
		query += ` AND (supervisor.calendar_period_id IS NULL OR supervisor.calendar_period_id = ?)`
		args = append(args, *calendarPeriodID)
	} else {
		query += ` AND supervisor.calendar_period_id IS NULL`
	}
	query += `
		GROUP BY supervisor.group_id, template_day.weekday, supervisor.staff_id
		UNION ALL
		SELECT
			enrollment.activity_group_id AS template_id,
			template_day.weekday,
			'student' AS kind,
			enrollment.student_id AS person_id,
			FALSE AS is_primary
		FROM activities.student_enrollments AS enrollment
		INNER JOIN template_weekdays AS template_day
			ON template_day.template_id = enrollment.activity_group_id
			AND (enrollment.weekday IS NULL OR template_day.weekday = enrollment.weekday)
		WHERE enrollment.tenant_id = ?
		  AND enrollment.valid_until IS NULL`
	args = append(args, tenantID)
	if calendarPeriodID != nil {
		query += ` AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = ?)`
		args = append(args, *calendarPeriodID)
	} else {
		query += ` AND enrollment.calendar_period_id IS NULL`
	}
	query += `
		  AND (
			enrollment.selected_weekdays IS NULL
			OR jsonb_array_length(enrollment.selected_weekdays) = 0
			OR enrollment.selected_weekdays @> jsonb_build_array(template_day.weekday)
		  )
		GROUP BY enrollment.activity_group_id, template_day.weekday, enrollment.student_id
		UNION ALL
		SELECT
			enrollment.activity_group_id AS template_id,
			template_day.weekday,
			'protected_student' AS kind,
			enrollment.student_id AS person_id,
			FALSE AS is_primary
		FROM activities.student_enrollments AS enrollment
		INNER JOIN scheduled_template_weekdays AS template_day
			ON template_day.template_id = enrollment.activity_group_id
		WHERE enrollment.tenant_id = ?
		  AND enrollment.valid_until IS NULL`
	args = append(args, tenantID)
	if calendarPeriodID != nil {
		query += ` AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = ?)`
		args = append(args, *calendarPeriodID)
	} else {
		query += ` AND enrollment.calendar_period_id IS NULL`
	}
	query += `
		  AND (
			enrollment.enrollment_request_child_id IS NOT NULL
			OR COALESCE(jsonb_array_length(enrollment.selected_weekdays), 0) > 0
		  )
		  AND (enrollment.weekday IS NULL OR enrollment.weekday = template_day.weekday)
		  AND (
			enrollment.selected_weekdays IS NULL
			OR jsonb_array_length(enrollment.selected_weekdays) = 0
			OR enrollment.selected_weekdays @> jsonb_build_array(template_day.weekday)
		  )
		GROUP BY enrollment.activity_group_id, template_day.weekday, enrollment.student_id
		ORDER BY template_id ASC, weekday ASC, kind ASC, is_primary DESC, person_id ASC`
	if err := base.GetDB(ctx, r.db).NewRaw(query, args...).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListTemplateRowsForPeriod is the calendar-period-filtered template list
// (issue #584: moved verbatim from api/timetable).
func (r *GroupRepository) ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activities.TemplateListRow, error) {
	tenantID := tenant.FromContext(ctx)
	rows := make([]activities.TemplateListRow, 0)
	query := templateListSelect + `
			LEFT JOIN (
				SELECT
					activity_group_id,
					COUNT(*) AS count,
					ARRAY_AGG(student_id ORDER BY student_id) AS student_ids
				FROM (
					SELECT DISTINCT activity_group_id, student_id
					FROM activities.student_enrollments
					WHERE tenant_id = ?
					  AND valid_until IS NULL
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
	  AND s.valid_until IS NULL`

	args := []any{tenantID}
	args = append(args, tenantID)
	args = append(args, tenantID)
	if periodID != nil {
		// Match materialization's period precedence exactly: a schedule pin is
		// authoritative; an unpinned schedule inherits the group pin; only a
		// schedule and group that are both unpinned are period-flexible.
		query += ` AND (
			s.calendar_period_id = ?
			OR (
				s.calendar_period_id IS NULL
				AND (g.calendar_period_id = ? OR g.calendar_period_id IS NULL)
			)
		)`
		args = append(args, *periodID, *periodID)
	}
	query += ` ORDER BY g.name ASC, s.weekday ASC, tf.start_time ASC`

	if err := base.GetDB(ctx, r.db).NewRaw(query, args...).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ListTemplateCapacityOccurrences returns one row for every date on which at
// least one schedule of a template actually recurs. Keeping the repository
// result date-granular is essential: the tenant-specific children/staff ratio
// is business logic, and combining non-concurrent rosters here would erase the
// evidence the service needs to choose the real worst occurrence.
func (r *GroupRepository) ListTemplateCapacityOccurrences(
	ctx context.Context,
	periodID *int64,
	templateIDs []int64,
) ([]activities.TemplateCapacityOccurrence, error) {
	if len(templateIDs) == 0 {
		return []activities.TemplateCapacityOccurrence{}, nil
	}

	occurrences := make([]activities.TemplateCapacityOccurrence, 0)
	tenantID := tenant.FromContext(ctx)
	err := base.GetDB(ctx, r.db).NewRaw(`
		WITH selected_period AS (
			SELECT id AS calendar_period_id, start_date, end_date, week_cycle_length, week_cycle_anchor
			FROM schedule.calendar_periods
			WHERE tenant_id = ?
			  AND is_active = TRUE
			  AND (?::BIGINT IS NULL OR id = ?)
		), candidate_occurrences AS MATERIALIZED (
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
			  AND exception.exception_type IS DISTINCT FROM 'cancelled'
			  AND COALESCE(exception.room_id, g.planned_room_id, 0) > 0
			  AND EXTRACT(ISODOW FROM days.day)::INT = s.weekday
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
						SELECT MIN(active_period.id)
						FROM schedule.calendar_periods AS active_period
						WHERE active_period.tenant_id = g.tenant_id
						  AND active_period.is_active = TRUE
						  AND active_period.start_date <= days.day::DATE
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
		), capacity_parts AS (
			SELECT
				occurrence.template_id,
				occurrence.calendar_period_id,
				occurrence.occurrence_date,
				COUNT(DISTINCT enrollment.student_id)::INT AS enrollment_count,
				0::INT AS supervisor_count
			FROM candidate_occurrences AS occurrence
			INNER JOIN activities.student_enrollments AS enrollment
				ON enrollment.tenant_id = ?
				AND enrollment.activity_group_id = occurrence.template_id
				AND enrollment.valid_from <= occurrence.occurrence_date
				AND (enrollment.valid_until IS NULL OR enrollment.valid_until > occurrence.occurrence_date)
				AND (enrollment.calendar_period_id IS NULL OR enrollment.calendar_period_id = occurrence.calendar_period_id)
				AND (
					enrollment.selected_weekdays IS NULL
					OR jsonb_array_length(enrollment.selected_weekdays) = 0
					OR enrollment.selected_weekdays @> jsonb_build_array(
						EXTRACT(ISODOW FROM occurrence.occurrence_date)::INT
					)
				)
				-- Weekday-scoped roster row (#2129): NULL applies on every
				-- weekday of the series, a value only on that weekday.
				AND (
					enrollment.weekday IS NULL
					OR enrollment.weekday = EXTRACT(ISODOW FROM occurrence.occurrence_date)::INT
				)
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
					OR supervisor.weekday = EXTRACT(ISODOW FROM occurrence.occurrence_date)::INT
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
		ORDER BY template_id ASC, occurrence_date ASC, calendar_period_id ASC
	`, tenantID, periodID, periodID,
		tenantID, bun.List(templateIDs),
		tenantID,
		tenantID,
	).Scan(ctx, &occurrences)
	if err != nil {
		return nil, err
	}
	return occurrences, nil
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

	// jsonb column via raw Set(): bind the JSON text (PostgreSQL casts the
	// parameter to the column type); nil keeps the column NULL. A grade
	// filter without a source is normalized away to satisfy
	// chk_activities_groups_offering_source.
	var sourceGradeLevels any
	if fields.SourceCareOfferingID != nil && len(fields.SourceGradeLevels) > 0 {
		encoded, err := json.Marshal(fields.SourceGradeLevels)
		if err != nil {
			return 0, fmt.Errorf("marshal source_grade_levels: %w", err)
		}
		sourceGradeLevels = string(encoded)
	}

	query := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.groups").
		Set("name = ?", fields.Name).
		Set("type = ?", fields.Type).
		Set("category_id = ?", fields.CategoryID).
		Set("planned_room_id = ?", fields.RoomID).
		Set("education_group_id = ?", fields.EducationGroupID).
		Set("max_participants = ?", fields.MaxParticipants).
		Set("required_staff = ?", fields.RequiredStaff).
		Set("calendar_period_id = ?", fields.CalendarPeriodID).
		Set("target_group_type = ?", targetGroupType).
		Set("target_grade_level = ?", fields.TargetGradeLevel).
		Set("target_school_class = ?", fields.TargetSchoolClass).
		Set("source_care_offering_id = ?", fields.SourceCareOfferingID).
		Set("source_grade_levels = ?", sourceGradeLevels).
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
