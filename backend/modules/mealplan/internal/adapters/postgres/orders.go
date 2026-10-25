package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
)

type orderRow struct {
	bun.BaseModel `bun:"table:meal_orders,alias:meal_order"`
	ID            int64         `bun:"id,pk,autoincrement"`
	TenantID      int64         `bun:"tenant_id"`
	PeriodStart   calendar.Date `bun:"period_start,type:date"`
	PeriodEnd     calendar.Date `bun:"period_end,type:date"`
	DeadlineAt    time.Time     `bun:"deadline_at"`
	FrozenAt      time.Time     `bun:"frozen_at"`
	FormatVersion int           `bun:"format_version"`
	ConfirmedAt   *time.Time    `bun:"confirmed_at"`
	ConfirmedBy   *int64        `bun:"confirmed_by_account_id"`
	CreatedAt     time.Time     `bun:"created_at,scanonly"`
	Days          []string      `bun:"days,array,scanonly"`
}

type orderDayRow struct {
	bun.BaseModel `bun:"table:meal_order_days,alias:meal_order_day"`
	TenantID      int64         `bun:"tenant_id"`
	OrderID       int64         `bun:"order_id"`
	Date          calendar.Date `bun:"date,type:date"`
}

// orderLineRow and orderChangeRow read a deleted child (NULL) as StudentID 0.
type orderLineRow struct {
	bun.BaseModel `bun:"table:meal_order_lines,alias:meal_order_line"`
	TenantID      int64         `bun:"tenant_id"`
	OrderID       int64         `bun:"order_id"`
	Date          calendar.Date `bun:"date,type:date"`
	StudentID     int64         `bun:"student_id,nullzero"`
	SchoolClass   string        `bun:"school_class"`
}

type orderChangeRow struct {
	bun.BaseModel `bun:"table:meal_order_changes,alias:meal_order_change"`
	TenantID      int64         `bun:"tenant_id"`
	OrderID       int64         `bun:"order_id"`
	Sequence      int           `bun:"sequence"`
	Date          calendar.Date `bun:"date,type:date"`
	StudentID     int64         `bun:"student_id,nullzero"`
	SchoolClass   string        `bun:"school_class"`
	Delta         int           `bun:"delta"`
	DetectedAt    time.Time     `bun:"detected_at"`
	ChangedBy     *int64        `bun:"changed_by_account_id"`
}

// LockOrders takes the school's order lock for the rest of the transaction:
// fixing an order and numbering its changes happen one at a time.
func (s *Store) LockOrders(ctx context.Context) (domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	err = timedQuery(&stats, func() error {
		_, execErr := db.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, fmt.Sprintf("meal-orders:%d", tenantID))
		return execErr
	})
	if err != nil {
		return stats, fmt.Errorf("meal plan postgres: lock orders: %w", err)
	}
	return stats, nil
}

// FindOrders reads the orders with a day in [from, to], with all their days,
// and the lines and changes of the window.
func (s *Store) FindOrders(ctx context.Context, from, to domain.Date) (domain.OrderState, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return domain.OrderState{}, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	var orders []orderRow
	if err := timedQuery(&stats, func() error {
		return db.NewSelect().Model(&orders).
			ModelTableExpr(`schedule.meal_orders AS "meal_order"`).
			ColumnExpr(`"meal_order".*`).
			ColumnExpr(`(SELECT array_agg(day.date::text ORDER BY day.date) FROM schedule.meal_order_days AS day WHERE day.tenant_id = "meal_order".tenant_id AND day.order_id = "meal_order".id) AS days`).
			Where(`"meal_order".tenant_id = ?`, tenantID).
			Where(`"meal_order".period_end >= ? AND "meal_order".period_start <= ?`, from, to).
			OrderExpr(`"meal_order".period_start ASC`).
			Scan(ctx)
	}); err != nil {
		return domain.OrderState{}, stats, fmt.Errorf("meal plan postgres: find orders: %w", err)
	}
	var lines []orderLineRow
	if err := timedQuery(&stats, func() error {
		return db.NewSelect().Model(&lines).
			ModelTableExpr(`schedule.meal_order_lines AS "meal_order_line"`).
			Where(`"meal_order_line".tenant_id = ?`, tenantID).
			Where(`"meal_order_line".date BETWEEN ? AND ?`, from, to).
			Scan(ctx)
	}); err != nil {
		return domain.OrderState{}, stats, fmt.Errorf("meal plan postgres: find order lines: %w", err)
	}
	var changes []orderChangeRow
	if err := timedQuery(&stats, func() error {
		return db.NewSelect().Model(&changes).
			ModelTableExpr(`schedule.meal_order_changes AS "meal_order_change"`).
			Where(`"meal_order_change".tenant_id = ?`, tenantID).
			Where(`"meal_order_change".date BETWEEN ? AND ?`, from, to).
			OrderExpr(`"meal_order_change".sequence ASC`).
			Scan(ctx)
	}); err != nil {
		return domain.OrderState{}, stats, fmt.Errorf("meal plan postgres: find order changes: %w", err)
	}
	stats.Rows += int64(len(orders) + len(lines) + len(changes))
	return orderState(orders, lines, changes), stats, nil
}

func orderState(orders []orderRow, lines []orderLineRow, changes []orderChangeRow) domain.OrderState {
	state := domain.OrderState{
		Orders:  make([]domain.Order, 0, len(orders)),
		Lines:   make([]domain.OrderLine, 0, len(lines)),
		Changes: make([]domain.OrderChange, 0, len(changes)),
	}
	for _, order := range orders {
		days := make([]domain.Date, 0, len(order.Days))
		for _, day := range order.Days {
			days = append(days, domain.Date(day))
		}
		state.Orders = append(state.Orders, domain.Order{
			ID: order.ID, PeriodStart: order.PeriodStart, PeriodEnd: order.PeriodEnd,
			DeadlineAt: order.DeadlineAt, FrozenAt: order.FrozenAt, ConfirmedAt: order.ConfirmedAt, Days: days,
		})
	}
	for _, line := range lines {
		state.Lines = append(state.Lines, domain.OrderLine{Date: line.Date, StudentID: line.StudentID, SchoolClass: line.SchoolClass})
	}
	for _, change := range changes {
		state.Changes = append(state.Changes, domain.OrderChange{
			OrderID: change.OrderID, Sequence: change.Sequence, Date: change.Date, StudentID: change.StudentID,
			SchoolClass: change.SchoolClass, Delta: change.Delta, DetectedAt: change.DetectedAt, ChangedBy: change.ChangedBy,
		})
	}
	return state
}

// InsertOrder fixes an order with its days and lines.
func (s *Store) InsertOrder(ctx context.Context, order domain.NewOrder, frozenAt time.Time) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	row := orderRow{TenantID: tenantID, PeriodStart: order.PeriodStart, PeriodEnd: order.PeriodEnd, DeadlineAt: order.DeadlineAt, FrozenAt: frozenAt, FormatVersion: domain.OrderFormatVersion}
	if err := timedQuery(&stats, func() error {
		_, insertErr := db.NewInsert().Model(&row).ModelTableExpr(`schedule.meal_orders`).
			ExcludeColumn("id", "confirmed_at", "confirmed_by_account_id").Returning("id").Exec(ctx)
		return insertErr
	}); err != nil {
		return 0, stats, fmt.Errorf("meal plan postgres: insert order: %w", err)
	}
	days := make([]orderDayRow, 0, len(order.Days))
	for _, date := range order.Days {
		days = append(days, orderDayRow{TenantID: tenantID, OrderID: row.ID, Date: date})
	}
	if err := timedQuery(&stats, func() error {
		_, insertErr := db.NewInsert().Model(&days).ModelTableExpr(`schedule.meal_order_days`).Exec(ctx)
		return insertErr
	}); err != nil {
		return 0, stats, fmt.Errorf("meal plan postgres: insert order days: %w", err)
	}
	if len(order.Lines) == 0 {
		return row.ID, stats, nil
	}
	lines := make([]orderLineRow, 0, len(order.Lines))
	for _, line := range order.Lines {
		lines = append(lines, orderLineRow{TenantID: tenantID, OrderID: row.ID, Date: line.Date, StudentID: line.StudentID, SchoolClass: line.SchoolClass})
	}
	if err := timedQuery(&stats, func() error {
		_, insertErr := db.NewInsert().Model(&lines).ModelTableExpr(`schedule.meal_order_lines`).Exec(ctx)
		return insertErr
	}); err != nil {
		return 0, stats, fmt.Errorf("meal plan postgres: insert order lines: %w", err)
	}
	stats.Rows += int64(1 + len(days) + len(lines))
	return row.ID, stats, nil
}

// InsertOrderChanges numbers the changes after the school's last number and
// stores them. The number comes from the school's counter, which never goes
// back, so a number once shown is never given to another change. The caller
// holds the order lock.
func (s *Store) InsertOrderChanges(ctx context.Context, changes []domain.OrderChange, detectedAt time.Time) ([]domain.OrderChange, domain.OperationStats, error) {
	if len(changes) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	var last int
	if err := timedQuery(&stats, func() error {
		return db.NewRaw(`
			INSERT INTO schedule.meal_order_change_counters AS counter (tenant_id, last_sequence) VALUES (?, ?)
			ON CONFLICT (tenant_id) DO UPDATE SET last_sequence = counter.last_sequence + EXCLUDED.last_sequence
			RETURNING last_sequence - ?
		`, tenantID, len(changes), len(changes)).Scan(ctx, &last)
	}); err != nil {
		return nil, stats, fmt.Errorf("meal plan postgres: number order changes: %w", err)
	}
	rows := make([]orderChangeRow, 0, len(changes))
	numbered := make([]domain.OrderChange, 0, len(changes))
	for index, change := range changes {
		change.Sequence, change.DetectedAt = last+index+1, detectedAt
		numbered = append(numbered, change)
		rows = append(rows, orderChangeRow{
			TenantID: tenantID, OrderID: change.OrderID, Sequence: change.Sequence, Date: change.Date, StudentID: change.StudentID,
			SchoolClass: change.SchoolClass, Delta: change.Delta, DetectedAt: detectedAt, ChangedBy: change.ChangedBy,
		})
	}
	if err := timedQuery(&stats, func() error {
		_, insertErr := db.NewInsert().Model(&rows).ModelTableExpr(`schedule.meal_order_changes`).Exec(ctx)
		return insertErr
	}); err != nil {
		return nil, stats, fmt.Errorf("meal plan postgres: insert order changes: %w", err)
	}
	stats.Rows += int64(len(rows))
	return numbered, stats, nil
}
