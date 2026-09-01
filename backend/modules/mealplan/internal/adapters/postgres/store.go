package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/uptrace/bun"
)

type Database func(context.Context) (bun.IDB, int64, error)

type Store struct{ database Database }

type row struct {
	bun.BaseModel `bun:"table:meal_plan_entries,alias:meal_plan_entry"`
	TenantID      int64         `bun:"tenant_id"`
	Date          timezone.Date `bun:"date,type:date"`
	Position      int           `bun:"position"`
	Dish          string        `bun:"dish"`
	Note          *string       `bun:"note"`
}

type participationScheduleRow struct {
	bun.BaseModel `bun:"table:meal_participation_schedules,alias:meal_participation_schedule"`
	TenantID      int64         `bun:"tenant_id"`
	StudentID     int64         `bun:"student_id"`
	EffectiveFrom timezone.Date `bun:"effective_from,type:date"`
	Monday        bool          `bun:"monday"`
	Tuesday       bool          `bun:"tuesday"`
	Wednesday     bool          `bun:"wednesday"`
	Thursday      bool          `bun:"thursday"`
	Friday        bool          `bun:"friday"`
	ChangedBy     int64         `bun:"changed_by_account_id"`
}

type participationOverrideRow struct {
	bun.BaseModel `bun:"table:meal_participation_overrides,alias:meal_participation_override"`
	TenantID      int64         `bun:"tenant_id"`
	StudentID     int64         `bun:"student_id"`
	Date          timezone.Date `bun:"date,type:date"`
	Participating bool          `bun:"participating"`
	ChangedBy     int64         `bun:"changed_by_account_id"`
}

type sickDayRow struct {
	ID         int64         `bun:"id"`
	Date       timezone.Date `bun:"date,type:date"`
	ChangedAt  time.Time     `bun:"changed_at"`
	ReportedAt time.Time     `bun:"reported_at"`
	ClearedAt  *time.Time    `bun:"cleared_at"`
}

type dailyCandidateRow struct {
	StudentID      int64      `bun:"student_id"`
	FirstName      string     `bun:"first_name"`
	LastName       string     `bun:"last_name"`
	SchoolClass    string     `bun:"school_class"`
	Regular        bool       `bun:"regular"`
	Override       *bool      `bun:"override"`
	SickReportedAt *time.Time `bun:"sick_reported_at"`
	SickClearedAt  *time.Time `bun:"sick_cleared_at"`
}

func New(database Database) *Store {
	if database == nil {
		panic("meal plan postgres: database runtime is required")
	}
	return &Store{database: database}
}

func (s *Store) FindWeek(ctx context.Context, start, end domain.Date) ([]domain.Entry, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	var rows []row
	err = db.NewSelect().Model(&rows).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".tenant_id = ?`, tenantID).
		Where(`"meal_plan_entry".date >= ?`, start).
		Where(`"meal_plan_entry".date <= ?`, end).
		OrderExpr(`"meal_plan_entry".date ASC`).
		OrderExpr(`"meal_plan_entry".position ASC`).
		Scan(ctx)
	if err != nil {
		return nil, stats, fmt.Errorf("meal plan postgres: find week: %w", err)
	}
	entries := make([]domain.Entry, 0, len(rows))
	for _, value := range rows {
		entries = append(entries, domain.Entry{Date: value.Date, Position: value.Position, Dish: value.Dish, Note: value.Note})
	}
	return entries, stats, nil
}

func (s *Store) ReplaceDay(ctx context.Context, date domain.Date, dishes []domain.Dish) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats, err := deleteDay(ctx, db, tenantID, date)
	if err != nil || len(dishes) == 0 {
		return stats, err
	}
	rows := make([]row, 0, len(dishes))
	for position, dish := range dishes {
		rows = append(rows, row{TenantID: tenantID, Date: date, Position: position, Dish: dish.Dish, Note: dish.Note})
	}
	query := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_plan_entries`)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: insert day: %w", err)
	}
	inserted, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: count inserted rows: %w", err)
	}
	stats.Rows += inserted
	return stats, nil
}

func (s *Store) ClearDay(ctx context.Context, date domain.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	return deleteDay(ctx, db, tenantID, date)
}

func deleteDay(ctx context.Context, db bun.IDB, tenantID int64, date domain.Date) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	query := db.NewDelete().Model((*row)(nil)).
		ModelTableExpr(`schedule.meal_plan_entries AS "meal_plan_entry"`).
		Where(`"meal_plan_entry".tenant_id = ?`, tenantID).
		Where(`"meal_plan_entry".date = ?`, date)
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: delete day: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: count deleted rows: %w", err)
	}
	stats.Rows = rows
	return stats, nil
}

func (s *Store) FindParticipation(ctx context.Context, studentID int64, start, end domain.Date) (domain.ParticipationData, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.ParticipationData{}, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	var schedules []participationScheduleRow
	if err := timedQuery(&stats, func() error {
		return db.NewSelect().Model(&schedules).
			ModelTableExpr(`schedule.meal_participation_schedules AS "meal_participation_schedule"`).
			Where(`"meal_participation_schedule".tenant_id = ?`, tenantID).
			Where(`"meal_participation_schedule".student_id = ?`, studentID).
			Where(`"meal_participation_schedule".effective_from <= ?`, end).
			OrderExpr(`"meal_participation_schedule".effective_from ASC`).
			Scan(ctx)
	}); err != nil {
		return domain.ParticipationData{}, stats, fmt.Errorf("meal plan postgres: find participation schedules: %w", err)
	}

	var overrides []participationOverrideRow
	if err := timedQuery(&stats, func() error {
		return db.NewSelect().Model(&overrides).
			ModelTableExpr(`schedule.meal_participation_overrides AS "meal_participation_override"`).
			Where(`"meal_participation_override".tenant_id = ?`, tenantID).
			Where(`"meal_participation_override".student_id = ?`, studentID).
			Where(`"meal_participation_override".date BETWEEN ? AND ?`, start, end).
			OrderExpr(`"meal_participation_override".date ASC`).
			Scan(ctx)
	}); err != nil {
		return domain.ParticipationData{}, stats, fmt.Errorf("meal plan postgres: find participation overrides: %w", err)
	}

	var sickDays []sickDayRow
	if err := timedQuery(&stats, func() error {
		return db.NewSelect().Model(&sickDays).
			ModelTableExpr(`schedule.meal_sickness_status_history AS "sickness_history"`).
			ColumnExpr(`"sickness_history".id`).
			ColumnExpr(`"sickness_history".date`).
			ColumnExpr(`"sickness_history".changed_at`).
			ColumnExpr(`"sickness_history".reported_at`).
			ColumnExpr(`"sickness_history".cleared_at`).
			Where(`"sickness_history".tenant_id = ?`, tenantID).
			Where(`"sickness_history".student_id = ?`, studentID).
			Where(`"sickness_history".date BETWEEN ? AND ?`, start, end).
			OrderExpr(`"sickness_history".date ASC, "sickness_history".changed_at ASC, "sickness_history".id ASC`).
			Scan(ctx)
	}); err != nil {
		return domain.ParticipationData{}, stats, fmt.Errorf("meal plan postgres: find sick days: %w", err)
	}

	data := domain.ParticipationData{
		Schedules: make([]domain.ParticipationSchedule, 0, len(schedules)),
		Overrides: make([]domain.DayOverride, 0, len(overrides)),
		SickDays:  make([]domain.SickDay, 0, len(sickDays)),
	}
	for _, schedule := range schedules {
		data.Schedules = append(data.Schedules, domain.ParticipationSchedule{EffectiveFrom: schedule.EffectiveFrom, Weekdays: weekdaysFromRow(schedule)})
	}
	for _, override := range overrides {
		data.Overrides = append(data.Overrides, domain.DayOverride{Date: override.Date, Participating: override.Participating})
	}
	for _, sick := range sickDays {
		data.SickDays = append(data.SickDays, domain.SickDay{ID: sick.ID, Date: sick.Date, ChangedAt: sick.ChangedAt, ReportedAt: sick.ReportedAt, ClearedAt: sick.ClearedAt})
	}
	return data, stats, nil
}

func (s *Store) InsertParticipationSchedule(ctx context.Context, studentID, accountID int64, effectiveFrom domain.Date, weekdays []domain.Weekday) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := participationScheduleRow{TenantID: tenantID, StudentID: studentID, EffectiveFrom: effectiveFrom, ChangedBy: accountID}
	for _, weekday := range weekdays {
		switch weekday {
		case 1:
			row.Monday = true
		case 2:
			row.Tuesday = true
		case 3:
			row.Wednesday = true
		case 4:
			row.Thursday = true
		case 5:
			row.Friday = true
		}
	}
	stats := domain.OperationStats{}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).
		ModelTableExpr(`schedule.meal_participation_schedules`).
		On(`CONFLICT (tenant_id, student_id, effective_from) DO UPDATE`).
		Set(`monday = EXCLUDED.monday, tuesday = EXCLUDED.tuesday, wednesday = EXCLUDED.wednesday, thursday = EXCLUDED.thursday, friday = EXCLUDED.friday, changed_by_account_id = EXCLUDED.changed_by_account_id, updated_at = NOW()`).
		Exec(ctx)
	stats.Queries = 1
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: save participation schedule: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) UpsertParticipationOverride(ctx context.Context, studentID, accountID int64, date domain.Date, participating bool) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	row := participationOverrideRow{TenantID: tenantID, StudentID: studentID, Date: date, Participating: participating, ChangedBy: accountID}
	stats := domain.OperationStats{}
	started := time.Now()
	result, err := db.NewInsert().Model(&row).
		ModelTableExpr(`schedule.meal_participation_overrides`).
		On(`CONFLICT (tenant_id, student_id, date) DO UPDATE`).
		Set(`participating = EXCLUDED.participating, changed_by_account_id = EXCLUDED.changed_by_account_id, updated_at = NOW()`).
		Exec(ctx)
	stats.Queries = 1
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: save participation override: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) DeleteParticipationOverride(ctx context.Context, studentID int64, date domain.Date) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	result, err := db.NewDelete().Model((*participationOverrideRow)(nil)).
		ModelTableExpr(`schedule.meal_participation_overrides AS "meal_participation_override"`).
		Where(`"meal_participation_override".tenant_id = ?`, tenantID).
		Where(`"meal_participation_override".student_id = ?`, studentID).
		Where(`"meal_participation_override".date = ?`, date).
		Exec(ctx)
	stats.Queries = 1
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: delete participation override: %w", err)
	}
	stats.Rows, _ = result.RowsAffected()
	return stats, nil
}

func (s *Store) FindDailyCandidates(ctx context.Context, date domain.Date, cutoff time.Time) ([]domain.DailyCandidate, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	weekdayColumn := map[time.Weekday]string{
		time.Monday: "monday", time.Tuesday: "tuesday", time.Wednesday: "wednesday", time.Thursday: "thursday", time.Friday: "friday",
	}[date.Weekday()]
	var rows []dailyCandidateRow
	stats := domain.OperationStats{}
	err = timedQuery(&stats, func() error {
		return db.NewSelect().Model(&rows).
			ModelTableExpr(`users.students AS "student"`).
			ColumnExpr(`"student".id AS student_id, "person".first_name, "person".last_name, "student".school_class`).
			ColumnExpr(fmt.Sprintf(`COALESCE("regular".%s, FALSE) AS regular`, weekdayColumn)).
			ColumnExpr(`"override".participating AS override`).
			ColumnExpr(`"sick".reported_at AS sick_reported_at`).
			ColumnExpr(`"sick".cleared_at AS sick_cleared_at`).
			Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id AND "person".tenant_id = "student".tenant_id AND "person".deleted_at IS NULL`).
			Join(`LEFT JOIN LATERAL (SELECT "schedule".* FROM schedule.meal_participation_schedules AS "schedule" WHERE "schedule".tenant_id = "student".tenant_id AND "schedule".student_id = "student".id AND "schedule".effective_from <= ? ORDER BY "schedule".effective_from DESC LIMIT 1) AS "regular" ON TRUE`, date).
			Join(`LEFT JOIN schedule.meal_participation_overrides AS "override" ON "override".tenant_id = "student".tenant_id AND "override".student_id = "student".id AND "override".date = ?`, date).
			Join(`LEFT JOIN LATERAL (SELECT "history".reported_at, "history".cleared_at FROM schedule.meal_sickness_status_history AS "history" WHERE "history".tenant_id = "student".tenant_id AND "history".student_id = "student".id AND "history".date = ? AND "history".changed_at <= ? ORDER BY "history".changed_at DESC, "history".id DESC LIMIT 1) AS "sick" ON TRUE`, date, cutoff).
			Where(`"student".tenant_id = ?`, tenantID).
			Where(`"student".status = 'active'`).
			Where(`("student".enrolled_from IS NULL OR "student".enrolled_from <= ?)`, date).
			Where(`("student".enrolled_until IS NULL OR "student".enrolled_until >= ?)`, date).
			OrderExpr(`"student".school_class ASC, "person".last_name ASC, "person".first_name ASC, "student".id ASC`).
			Scan(ctx)
	})
	if err != nil {
		return nil, stats, fmt.Errorf("meal plan postgres: find daily participants: %w", err)
	}
	result := make([]domain.DailyCandidate, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.DailyCandidate{StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName, SchoolClass: row.SchoolClass, Regular: row.Regular, Override: row.Override, SickReportedAt: row.SickReportedAt, SickClearedAt: row.SickClearedAt})
	}
	return result, stats, nil
}

func weekdaysFromRow(row participationScheduleRow) []domain.Weekday {
	values := []bool{row.Monday, row.Tuesday, row.Wednesday, row.Thursday, row.Friday}
	weekdays := make([]domain.Weekday, 0, 5)
	for index, enabled := range values {
		if enabled {
			weekdays = append(weekdays, domain.Weekday(index+1))
		}
	}
	return weekdays
}

func timedQuery(stats *domain.OperationStats, query func() error) error {
	started := time.Now()
	err := query()
	stats.Queries++
	stats.StatementDuration += time.Since(started)
	return err
}
