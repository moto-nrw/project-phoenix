package postgres

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const pickupExtensionTaskColumns = `
	"task".id, "task".student_id, "task".pickup_exception_id,
	COALESCE("task".task_date::text, '') AS task_date,
	COALESCE("task".weekday, 0) AS weekday,
	COALESCE("task".effective_from::text, '') AS effective_from,
	to_char("task".previous_pickup_time, 'HH24:MI') AS previous_pickup,
	to_char("task".pickup_time, 'HH24:MI') AS pickup`

type pickupExtensionTaskRow struct {
	ID                int64       `bun:"id"`
	StudentID         int64       `bun:"student_id"`
	PickupExceptionID *int64      `bun:"pickup_exception_id"`
	TaskDate          domain.Date `bun:"task_date"`
	Weekday           int         `bun:"weekday"`
	EffectiveFrom     domain.Date `bun:"effective_from"`
	PreviousPickup    string      `bun:"previous_pickup"`
	Pickup            string      `bun:"pickup"`
}

func (row pickupExtensionTaskRow) toDomain() domain.PickupExtensionTask {
	return domain.PickupExtensionTask{
		ID: row.ID, StudentID: row.StudentID, PickupExceptionID: row.PickupExceptionID,
		Date: row.TaskDate, Weekday: row.Weekday, EffectiveFrom: row.EffectiveFrom,
		PreviousPickup: row.PreviousPickup, Pickup: row.Pickup,
	}
}

type pickupExtensionBlockRow struct {
	TaskID           int64       `bun:"task_id"`
	ID               int64       `bun:"id"`
	Title            string      `bun:"title"`
	StartTime        string      `bun:"start_time"`
	EndTime          string      `bun:"end_time"`
	Member           bool        `bun:"member"`
	CalendarPeriodID *int64      `bun:"calendar_period_id"`
	ValidFrom        domain.Date `bun:"valid_from"`
}

// UpsertPickupExtensionTask writes a day or weekday task. Each kind has its
// own unique index, so the conflict target follows the kind.
func (s *Store) UpsertPickupExtensionTask(ctx context.Context, task domain.PickupExtensionTask) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	if task.IsDay() {
		return execMeasuredWrite(ctx, db.NewRaw(`
			INSERT INTO schedule.pickup_extension_tasks
				(tenant_id, student_id, pickup_exception_id, task_date, previous_pickup_time, pickup_time)
			VALUES (?, ?, ?, ?::date, ?::time, ?::time)
			ON CONFLICT (tenant_id, student_id, task_date) WHERE task_date IS NOT NULL
			DO UPDATE SET pickup_exception_id = EXCLUDED.pickup_exception_id,
				previous_pickup_time = EXCLUDED.previous_pickup_time,
				pickup_time = EXCLUDED.pickup_time`,
			tenantID, task.StudentID, task.PickupExceptionID, task.Date, task.PreviousPickup, task.Pickup,
		), "upsert pickup day extension")
	}
	return execMeasuredWrite(ctx, db.NewRaw(`
		INSERT INTO schedule.pickup_extension_tasks
			(tenant_id, student_id, weekday, effective_from, previous_pickup_time, pickup_time)
		VALUES (?, ?, ?, ?::date, ?::time, ?::time)
		ON CONFLICT (tenant_id, student_id, weekday) WHERE weekday IS NOT NULL
		DO UPDATE SET effective_from = EXCLUDED.effective_from,
			previous_pickup_time = EXCLUDED.previous_pickup_time,
			pickup_time = EXCLUDED.pickup_time`,
		tenantID, task.StudentID, task.Weekday, task.EffectiveFrom, task.PreviousPickup, task.Pickup,
	), "upsert pickup weekday extension")
}

func (s *Store) DeletePickupDayExtensionTask(ctx context.Context, studentID int64, date domain.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().TableExpr(`schedule.pickup_extension_tasks`).
		Where("tenant_id = ?", tenantID).Where("student_id = ?", studentID).
		Where("task_date = ?::date", date), "delete pickup day extension")
}

func (s *Store) DeletePickupWeekdayExtensionTask(ctx context.Context, studentID int64, weekday int) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().TableExpr(`schedule.pickup_extension_tasks`).
		Where("tenant_id = ?", tenantID).Where("student_id = ?", studentID).
		Where("weekday = ?", weekday), "delete pickup weekday extension")
}

func (s *Store) DeletePastPickupExtensionTasks(ctx context.Context, today domain.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().TableExpr(`schedule.pickup_extension_tasks`).
		Where("tenant_id = ?", tenantID).Where("task_date < ?::date", today), "prune past pickup extensions")
}

func (s *Store) DeletePickupExtensionTask(ctx context.Context, id int64) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return execMeasuredWrite(ctx, db.NewDelete().TableExpr(`schedule.pickup_extension_tasks`).
		Where("tenant_id = ?", tenantID).Where("id = ?", id), "delete pickup extension")
}

// ListPickupExtensionTasks returns the school's tasks, or one child's when
// studentID is positive. Day tasks before today are left out.
func (s *Store) ListPickupExtensionTasks(ctx context.Context, studentID int64, today domain.Date) ([]domain.PickupExtensionTask, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]pickupExtensionTaskRow, 0)
	query := db.NewSelect().TableExpr(`schedule.pickup_extension_tasks AS "task"`).
		ColumnExpr(pickupExtensionTaskColumns).
		Where(`"task".tenant_id = ?`, tenantID).
		Where(`("task".task_date IS NULL OR "task".task_date >= ?::date)`, today).
		OrderExpr(`COALESCE("task".task_date, "task".effective_from) ASC, "task".id ASC`)
	if studentID > 0 {
		query = query.Where(`"task".student_id = ?`, studentID)
	}
	stats, err := scanAllInto(ctx, query, &rows, "list pickup extensions")
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.PickupExtensionTask, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.toDomain())
	}
	stats.Rows = int64(len(result))
	return result, stats, nil
}

func (s *Store) FindPickupExtensionTaskForUpdate(ctx context.Context, id int64) (domain.PickupExtensionTask, bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.PickupExtensionTask{}, false, domain.OperationStats{}, err
	}
	row := pickupExtensionTaskRow{}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`SELECT `+pickupExtensionTaskColumns+`
		FROM schedule.pickup_extension_tasks AS "task"
		WHERE "task".tenant_id = ? AND "task".id = ?
		FOR UPDATE`, tenantID, id).Scan(ctx, &row)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PickupExtensionTask{}, false, stats, nil
	}
	if err != nil {
		return domain.PickupExtensionTask{}, false, stats, classifyWriteError("find pickup extension", err, &stats)
	}
	stats.Rows = 1
	return row.toDomain(), true, stats, nil
}

// ListPickupExtensionDayBlocks returns, per day task, the blocks of that date
// that have children and overlap the extra time. A row that keeps the child
// off the block for the day (not_scheduled) hides the block: it is neither a
// block the child attends nor one the child can be added to.
func (s *Store) ListPickupExtensionDayBlocks(ctx context.Context, tasks []domain.PickupExtensionTask) ([]domain.PickupExtensionBlock, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	taskIDs, studentIDs, dates, froms, tos := make([]int64, 0, len(tasks)), make([]int64, 0, len(tasks)), make([]domain.Date, 0, len(tasks)), make([]string, 0, len(tasks)), make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs, studentIDs, dates = append(taskIDs, task.ID), append(studentIDs, task.StudentID), append(dates, task.Date)
		froms, tos = append(froms, task.PreviousPickup), append(tos, task.Pickup)
	}
	rows := make([]pickupExtensionBlockRow, 0)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		WITH task AS (
			SELECT * FROM unnest(?::BIGINT[], ?::BIGINT[], ?::DATE[], ?::TIME[], ?::TIME[])
				AS task(task_id, student_id, task_date, from_time, to_time)
		)
		SELECT task.task_id, "instance".id, "instance".title,
			to_char("instance".start_time, 'HH24:MI') AS start_time,
			to_char("instance".end_time, 'HH24:MI') AS end_time,
			"own".id IS NOT NULL AS member,
			NULL::BIGINT AS calendar_period_id, '' AS valid_from
		FROM task
		JOIN schedule.activity_instances AS "instance"
			ON "instance".tenant_id = ? AND "instance".date = task.task_date
		LEFT JOIN schedule.instance_students AS "own"
			ON "own".tenant_id = "instance".tenant_id AND "own".instance_id = "instance".id
			AND "own".student_id = task.student_id
		WHERE "instance".status NOT IN ('cancelled', 'completed')
			AND "instance".start_time < task.to_time AND "instance".end_time > task.from_time
			AND ("own".id IS NULL OR NOT "own".not_scheduled)
			AND EXISTS (
				SELECT 1 FROM schedule.instance_students AS "attendee"
				WHERE "attendee".tenant_id = "instance".tenant_id AND "attendee".instance_id = "instance".id
					AND NOT "attendee".not_scheduled)
		ORDER BY task.task_id, "instance".start_time, "instance".id`,
		pgdialect.Array(taskIDs), pgdialect.Array(studentIDs), pgdialect.Array(dates),
		pgdialect.Array(froms), pgdialect.Array(tos), tenantID,
	).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, classifyWriteError("list pickup extension day blocks", err, &stats)
	}
	stats.Rows = int64(len(rows))
	return pickupExtensionBlocks(rows), stats, nil
}

// ListPickupExtensionWeekdayBlocks returns, per weekday task, the templates
// running on that weekday that overlap the extra time and have children:
// a target group or at least one child on the weekday roster. Member covers
// the stored roster only; target groups are matched by the caller.
func (s *Store) ListPickupExtensionWeekdayBlocks(ctx context.Context, tasks []domain.PickupExtensionTask) ([]domain.PickupExtensionBlock, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	taskIDs, studentIDs, weekdays, froms, tos := make([]int64, 0, len(tasks)), make([]int64, 0, len(tasks)), make([]int64, 0, len(tasks)), make([]string, 0, len(tasks)), make([]string, 0, len(tasks))
	effective := make([]domain.Date, 0, len(tasks))
	for _, task := range tasks {
		taskIDs, studentIDs, weekdays = append(taskIDs, task.ID), append(studentIDs, task.StudentID), append(weekdays, int64(task.Weekday))
		effective, froms, tos = append(effective, task.EffectiveFrom), append(froms, task.PreviousPickup), append(tos, task.Pickup)
	}
	rows := make([]pickupExtensionBlockRow, 0)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		WITH task AS (
			SELECT * FROM unnest(?::BIGINT[], ?::BIGINT[], ?::INT[], ?::DATE[], ?::TIME[], ?::TIME[])
				AS task(task_id, student_id, weekday, effective_from, from_time, to_time)
		)
		SELECT DISTINCT ON (task.task_id, "template".id)
			task.task_id, "template".id, "template".name AS title,
			to_char("timeframe".start_time, 'HH24:MI') AS start_time,
			to_char("timeframe".end_time, 'HH24:MI') AS end_time,
			COALESCE("schedule".calendar_period_id, "template".calendar_period_id) AS calendar_period_id,
			GREATEST(task.effective_from, COALESCE("schedule".valid_from, task.effective_from))::text AS valid_from,
			EXISTS (
				SELECT 1 FROM activities.student_enrollments AS "own"
				WHERE "own".tenant_id = "template".tenant_id AND "own".activity_group_id = "template".id
					AND "own".student_id = task.student_id
					AND "own".valid_from <= task.effective_from
					AND ("own".valid_until IS NULL OR "own".valid_until > task.effective_from)
					AND ("own".weekday IS NULL OR "own".weekday = task.weekday)
					AND (COALESCE(jsonb_array_length("own".selected_weekdays), 0) = 0
						OR "own".selected_weekdays @> to_jsonb(ARRAY[task.weekday]))
			) AS member
		FROM task
		JOIN activities.schedules AS "schedule"
			ON "schedule".tenant_id = ? AND "schedule".weekday = task.weekday
			AND ("schedule".valid_from IS NULL OR "schedule".valid_from <= task.effective_from)
			AND ("schedule".valid_until IS NULL OR "schedule".valid_until > task.effective_from)
		JOIN activities.groups AS "template"
			ON "template".id = "schedule".activity_group_id AND "template".tenant_id = "schedule".tenant_id
			AND "template".is_template AND "template".archived_at IS NULL
		JOIN schedule.timeframes AS "timeframe"
			ON "timeframe".id = "schedule".timeframe_id AND "timeframe".tenant_id = "schedule".tenant_id
		WHERE "timeframe".start_time < task.to_time AND "timeframe".end_time > task.from_time
			AND ("template".target_group_type <> 'none' OR EXISTS (
				SELECT 1 FROM activities.student_enrollments AS "attendee"
				WHERE "attendee".tenant_id = "template".tenant_id AND "attendee".activity_group_id = "template".id
					AND "attendee".valid_from <= task.effective_from
					AND ("attendee".valid_until IS NULL OR "attendee".valid_until > task.effective_from)
					AND ("attendee".weekday IS NULL OR "attendee".weekday = task.weekday)
					AND (COALESCE(jsonb_array_length("attendee".selected_weekdays), 0) = 0
						OR "attendee".selected_weekdays @> to_jsonb(ARRAY[task.weekday]))))
		ORDER BY task.task_id, "template".id, "schedule".valid_from DESC NULLS LAST`,
		pgdialect.Array(taskIDs), pgdialect.Array(studentIDs), pgdialect.Array(weekdays),
		pgdialect.Array(effective), pgdialect.Array(froms), pgdialect.Array(tos), tenantID,
	).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, classifyWriteError("list pickup extension weekday blocks", err, &stats)
	}
	stats.Rows = int64(len(rows))
	return pickupExtensionBlocks(rows), stats, nil
}

// ListPickupExtensionTemplateInstances returns the planned blocks of one
// template on the weekday from the given date on that do not list the child.
func (s *Store) ListPickupExtensionTemplateInstances(ctx context.Context, templateID, studentID int64, weekday int, from domain.Date) ([]domain.PickupExtensionInstance, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := make([]domain.PickupExtensionInstance, 0)
	query := db.NewSelect().TableExpr(`schedule.activity_instances AS "instance"`).
		ColumnExpr(`"instance".id AS id, "instance".date::text AS date`).
		Where(`"instance".tenant_id = ?`, tenantID).
		Where(`"instance".activity_group_id = ?`, templateID).
		Where(`"instance".date >= ?::date`, from).
		Where(`date_part('isodow', "instance".date) = ?`, weekday).
		Where(`"instance".status IN ('planned', 'active')`).
		Where(`NOT "instance".is_spontaneous`).
		Where(`NOT EXISTS (
			SELECT 1 FROM schedule.instance_students AS "own"
			WHERE "own".tenant_id = "instance".tenant_id AND "own".instance_id = "instance".id
				AND "own".student_id = ?)`, studentID).
		OrderExpr(`"instance".date ASC, "instance".id ASC`)
	stats, err := scanAllInto(ctx, query, &rows, "list pickup extension template instances")
	if err != nil {
		return nil, stats, err
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func pickupExtensionBlocks(rows []pickupExtensionBlockRow) []domain.PickupExtensionBlock {
	result := make([]domain.PickupExtensionBlock, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.PickupExtensionBlock(row))
	}
	slices.SortStableFunc(result, func(a, b domain.PickupExtensionBlock) int {
		switch {
		case a.TaskID != b.TaskID:
			return compareOrdered(a.TaskID, b.TaskID)
		case a.StartTime != b.StartTime:
			return compareOrdered(a.StartTime, b.StartTime)
		default:
			return compareOrdered(a.ID, b.ID)
		}
	})
	return result
}

func compareOrdered[T int64 | string](a, b T) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
