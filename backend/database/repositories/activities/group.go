// backend/database/repositories/activities/group.go
package activities

import (
	"context"
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
				g.planned_room_id AS room_id,
				COALESCE(r.name, '') AS room_name,
				g.education_group_id,
				COALESCE(eg.name, '') AS education_group_name,
				g.is_open,
			g.max_participants,
			g.calendar_period_id AS template_calendar_period_id,
			g.target_group_type,
			g.target_grade_level,
			g.target_school_class,
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
			TO_CHAR(s.valid_until, 'YYYY-MM-DD') AS schedule_valid_until
		FROM activities.groups AS g
		INNER JOIN activities.schedules AS s
			ON s.activity_group_id = g.id AND s.tenant_id = g.tenant_id
		LEFT JOIN schedule.timeframes AS tf
			ON tf.id = s.timeframe_id AND tf.tenant_id = g.tenant_id
		LEFT JOIN activities.categories AS c
			ON c.id = g.category_id AND c.tenant_id = g.tenant_id
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
	// Start with the established unfiltered fallback. Single-template GET/PUT
	// responses below replace it with period-aware counts when their group and
	// schedules resolve to one unambiguous planning-period pin.
	for i := range rows {
		rows[i].CapacityEnrollmentCount = rows[i].EnrollmentCount
		rows[i].CapacitySupervisorCount = rows[i].SupervisorCount
	}
	if err := r.loadPinnedTemplateCapacityCounts(ctx, rows); err != nil {
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
	if periodID == nil {
		for i := range rows {
			rows[i].CapacityEnrollmentCount = rows[i].EnrollmentCount
			rows[i].CapacitySupervisorCount = rows[i].SupervisorCount
		}
		return rows, nil
	}
	if err := r.loadTemplateCapacityCounts(ctx, rows, *periodID); err != nil {
		return nil, err
	}
	return rows, nil
}

// loadTemplateCapacityCounts adds the roster counts that can actually be
// materialized in periodID. The main template-list aggregates deliberately
// remain period-tolerant for display/editing; capacity must not let staff or
// children assigned only to another overlapping period leak into the
// selected period's staffing ratio.
//
// Both people tables are aggregated in one batched query for every returned
// template. Enrollments contribute when unscoped/scoped to periodID and their
// [valid_from, valid_until) window overlaps the selected planning period. This
// includes phase-bounded care-offer enrollments which materialize during the
// period even though the display roster intentionally lists open rows.
// Supervisors are deliberately more conservative: only matching open rows
// count. Template edits leave bounded historical staff versions behind, and
// unioning those versions across a whole period could falsely report coverage.
func (r *GroupRepository) loadTemplateCapacityCounts(
	ctx context.Context,
	rows []activities.TemplateListRow,
	periodID int64,
) error {
	if len(rows) == 0 {
		return nil
	}

	templateIDs := make([]int64, 0, len(rows))
	seenTemplateIDs := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if _, seen := seenTemplateIDs[row.TemplateID]; seen {
			continue
		}
		seenTemplateIDs[row.TemplateID] = struct{}{}
		templateIDs = append(templateIDs, row.TemplateID)
	}
	return r.loadTemplateCapacityCountsForIDs(ctx, rows, periodID, templateIDs)
}

// loadPinnedTemplateCapacityCounts upgrades unfiltered template detail rows
// to period-aware capacity when every schedule of a template resolves to one
// unambiguous pin. Group-level pins apply to schedules without their own pin;
// a conflicting schedule pin is treated as ambiguous rather than reporting a
// confidently wrong ratio. IDs are grouped by period, so this remains one
// aggregate query per distinct period rather than one query per template.
func (r *GroupRepository) loadPinnedTemplateCapacityCounts(
	ctx context.Context,
	rows []activities.TemplateListRow,
) error {
	type pinState struct {
		groupPeriodID    int64
		hasGroupPeriodID bool
		schedulePeriodID int64
		hasSchedulePin   bool
		ambiguous        bool
	}
	states := make(map[int64]*pinState)
	for _, row := range rows {
		state := states[row.TemplateID]
		if state == nil {
			state = &pinState{}
			states[row.TemplateID] = state
		}
		if row.TemplateCalendarPeriodID.Valid {
			groupPeriodID := row.TemplateCalendarPeriodID.Int64
			if state.hasGroupPeriodID && state.groupPeriodID != groupPeriodID {
				state.ambiguous = true
			}
			state.groupPeriodID = groupPeriodID
			state.hasGroupPeriodID = true
		}
		if row.CalendarPeriodID.Valid {
			schedulePeriodID := row.CalendarPeriodID.Int64
			if state.hasSchedulePin && state.schedulePeriodID != schedulePeriodID {
				state.ambiguous = true
			}
			state.schedulePeriodID = schedulePeriodID
			state.hasSchedulePin = true
		} else if !row.TemplateCalendarPeriodID.Valid {
			// With neither a schedule nor group pin, materialization chooses a
			// period by date; no single period can summarize this template.
			state.ambiguous = true
		}
	}

	idsByPeriod := make(map[int64][]int64)
	for templateID, state := range states {
		if state.ambiguous {
			continue
		}
		periodID := state.groupPeriodID
		if !state.hasGroupPeriodID {
			if !state.hasSchedulePin {
				continue
			}
			periodID = state.schedulePeriodID
		} else if state.hasSchedulePin && state.schedulePeriodID != periodID {
			// The schedule pin is materialization-time-authoritative. Mixed
			// group/schedule pins cannot be represented by one card ratio.
			continue
		}
		idsByPeriod[periodID] = append(idsByPeriod[periodID], templateID)
	}

	for periodID, templateIDs := range idsByPeriod {
		if err := r.loadTemplateCapacityCountsForIDs(ctx, rows, periodID, templateIDs); err != nil {
			return err
		}
	}
	return nil
}

func (r *GroupRepository) loadTemplateCapacityCountsForIDs(
	ctx context.Context,
	rows []activities.TemplateListRow,
	periodID int64,
	templateIDs []int64,
) error {
	if len(templateIDs) == 0 {
		return nil
	}

	type capacityCount struct {
		TemplateID      int64 `bun:"template_id"`
		EnrollmentCount int   `bun:"enrollment_count"`
		SupervisorCount int   `bun:"supervisor_count"`
	}
	counts := make([]capacityCount, 0, len(templateIDs))
	tenantID := tenant.FromContext(ctx)
	err := base.GetDB(ctx, r.db).NewRaw(`
		WITH selected_period AS (
			SELECT start_date, end_date
			FROM schedule.calendar_periods
			WHERE tenant_id = ?
			  AND id = ?
		)
		SELECT
			roster.activity_group_id AS template_id,
			COUNT(DISTINCT roster.person_id) FILTER (
				WHERE roster.member_type = 'student'
			) AS enrollment_count,
			COUNT(DISTINCT roster.person_id) FILTER (
				WHERE roster.member_type = 'staff'
			) AS supervisor_count
		FROM (
			SELECT
				activity_group_id,
				student_id AS person_id,
				'student'::TEXT AS member_type
			FROM activities.student_enrollments
			CROSS JOIN selected_period
			WHERE tenant_id = ?
			  AND (calendar_period_id IS NULL OR calendar_period_id = ?)
			  AND valid_from <= selected_period.end_date
			  AND (valid_until IS NULL OR valid_until > selected_period.start_date)
			  AND activity_group_id IN (?)
			UNION ALL
			SELECT
				group_id AS activity_group_id,
				staff_id AS person_id,
				'staff'::TEXT AS member_type
			FROM activities.supervisors
			CROSS JOIN selected_period
			WHERE tenant_id = ?
			  AND (calendar_period_id IS NULL OR calendar_period_id = ?)
			  AND valid_until IS NULL
			  AND valid_from <= selected_period.end_date
			  AND group_id IN (?)
		) AS roster
		GROUP BY roster.activity_group_id
	`, tenantID, periodID, tenantID, periodID, bun.List(templateIDs), tenantID, periodID, bun.List(templateIDs)).Scan(ctx, &counts)
	if err != nil {
		return err
	}

	byTemplateID := make(map[int64]capacityCount, len(counts))
	for _, count := range counts {
		byTemplateID[count.TemplateID] = count
	}
	includedTemplateIDs := make(map[int64]struct{}, len(templateIDs))
	for _, templateID := range templateIDs {
		includedTemplateIDs[templateID] = struct{}{}
	}
	for i := range rows {
		if _, included := includedTemplateIDs[rows[i].TemplateID]; !included {
			continue
		}
		count := byTemplateID[rows[i].TemplateID]
		rows[i].CapacityEnrollmentCount = count.EnrollmentCount
		rows[i].CapacitySupervisorCount = count.SupervisorCount
	}
	return nil
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

	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Table("activities.groups").
		Set("name = ?", fields.Name).
		Set("type = ?", fields.Type).
		Set("category_id = ?", fields.CategoryID).
		Set("planned_room_id = ?", fields.RoomID).
		Set("education_group_id = ?", fields.EducationGroupID).
		Set("max_participants = ?", fields.MaxParticipants).
		Set("calendar_period_id = ?", fields.CalendarPeriodID).
		Set("target_group_type = ?", targetGroupType).
		Set("target_grade_level = ?", fields.TargetGradeLevel).
		Set("target_school_class = ?", fields.TargetSchoolClass).
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
