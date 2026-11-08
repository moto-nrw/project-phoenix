package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Mensa-Monatsfreigabe (#4686, migration 1.15.811). A release carries
// the school and its Träger instead of a tenant_id; the school reads and
// writes it in its school transaction, and every query here names the
// school again. The Träger reads only through
// schedule.traeger_meal_billing_children in its Träger transaction.

// billingMonthUnique keeps a month of a school to one release.
const billingMonthUnique = "uq_meal_billing_months_month"

// TraegerDatabase resolves the Träger transaction of organizationID; any
// other transaction is an error.
type TraegerDatabase func(ctx context.Context, organizationID int64) (bun.IDB, error)

// BillingMonthStore persists the Mensa-Monatsfreigabe.
type BillingMonthStore struct {
	school  Database
	traeger TraegerDatabase
}

// NewBillingMonthStore builds the store over the school and the Träger
// transaction.
func NewBillingMonthStore(school Database, traeger TraegerDatabase) *BillingMonthStore {
	return &BillingMonthStore{school: school, traeger: traeger}
}

type billingMonthRow struct {
	bun.BaseModel `bun:"table:schedule.meal_billing_months"`

	ID                  int64         `bun:"id,pk,autoincrement"`
	OrganizationID      int64         `bun:"organization_id"`
	SchoolID            int64         `bun:"school_id"`
	BillingMonth        calendar.Date `bun:"billing_month,type:date"`
	ReleasedAt          time.Time     `bun:"released_at,nullzero,default:current_timestamp"`
	ReleasedByAccountID *int64        `bun:"released_by_account_id"`
	ChildCount          int           `bun:"child_count"`
	MealCount           int           `bun:"meal_count"`
}

type billingMonthChildRow struct {
	bun.BaseModel `bun:"table:schedule.meal_billing_month_children"`

	ID             int64  `bun:"id,pk,autoincrement"`
	OrganizationID int64  `bun:"organization_id"`
	MonthID        int64  `bun:"month_id"`
	StudentID      int64  `bun:"student_id"`
	FirstName      string `bun:"first_name"`
	LastName       string `bun:"last_name"`
	SchoolClass    string `bun:"school_class"`
	RegisteredDays int    `bun:"registered_days"`
	MealDays       int    `bun:"meal_days"`
}

// ReleasedMonths reads the school's released months of [from, to].
func (s *BillingMonthStore) ReleasedMonths(ctx context.Context, from, to calendar.Date) ([]domain.StoredBillingMonth, error) {
	db, schoolID, err := s.school(ctx)
	if err != nil {
		return nil, err
	}
	var rows []billingMonthRow
	if err := db.NewSelect().Model(&rows).Where("school_id = ?", schoolID).
		Where("billing_month BETWEEN ? AND ?", from, to).OrderExpr("billing_month DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read released meal months: %w", err)
	}
	result := make([]domain.StoredBillingMonth, len(rows))
	for i, row := range rows {
		result[i] = domain.StoredBillingMonth{Month: row.BillingMonth, ReleasedAt: row.ReleasedAt, ChildCount: row.ChildCount, MealCount: row.MealCount}
	}
	return result, nil
}

// ReleasedMonth reads one released month of the school with its children.
func (s *BillingMonthStore) ReleasedMonth(ctx context.Context, month calendar.Date) (domain.StoredBillingMonth, bool, error) {
	db, schoolID, err := s.school(ctx)
	if err != nil {
		return domain.StoredBillingMonth{}, false, err
	}
	row := billingMonthRow{}
	if err := db.NewSelect().Model(&row).Where("school_id = ?", schoolID).Where("billing_month = ?", month).Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.StoredBillingMonth{}, false, nil
		}
		return domain.StoredBillingMonth{}, false, fmt.Errorf("read released meal month: %w", err)
	}
	var children []billingMonthChildRow
	if err := db.NewSelect().Model(&children).Where("organization_id = ?", row.OrganizationID).Where("month_id = ?", row.ID).
		OrderExpr("lower(last_name), lower(first_name), student_id").Scan(ctx); err != nil {
		return domain.StoredBillingMonth{}, false, fmt.Errorf("read released meal month children: %w", err)
	}
	stored := domain.StoredBillingMonth{Month: row.BillingMonth, ReleasedAt: row.ReleasedAt, ChildCount: row.ChildCount,
		MealCount: row.MealCount, Children: make([]domain.BillingChild, len(children))}
	for i, child := range children {
		stored.Children[i] = domain.BillingChild{StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, RegisteredDays: child.RegisteredDays, MealDays: child.MealDays}
	}
	return stored, true, nil
}

// SchoolOrganization names the Träger of the school.
func (s *BillingMonthStore) SchoolOrganization(ctx context.Context) (int64, bool, error) {
	db, schoolID, err := s.school(ctx)
	if err != nil {
		return 0, false, err
	}
	var organizationID sql.NullInt64
	if err := db.NewRaw(`SELECT schedule.meal_billing_school_organization(?)`, schoolID).Scan(ctx, &organizationID); err != nil {
		return 0, false, fmt.Errorf("read the Träger of the school: %w", err)
	}
	return organizationID.Int64, organizationID.Valid, nil
}

// InsertRelease stores the release with its children.
func (s *BillingMonthStore) InsertRelease(ctx context.Context, organizationID, accountID int64, month domain.StoredBillingMonth) (time.Time, error) {
	db, schoolID, err := s.school(ctx)
	if err != nil {
		return time.Time{}, err
	}
	row := &billingMonthRow{OrganizationID: organizationID, SchoolID: schoolID, BillingMonth: month.Month,
		ReleasedByAccountID: &accountID, ChildCount: month.ChildCount, MealCount: month.MealCount}
	if _, err := db.NewInsert().Model(row).Returning("id, released_at").Exec(ctx); err != nil {
		var pgErr pgdriver.Error
		if errors.As(err, &pgErr) && pgErr.Field('n') == billingMonthUnique {
			return time.Time{}, mealplan.ErrBillingMonthReleased
		}
		return time.Time{}, fmt.Errorf("insert meal month release: %w", err)
	}
	if len(month.Children) == 0 {
		return row.ReleasedAt, nil
	}
	children := make([]billingMonthChildRow, len(month.Children))
	for i, child := range month.Children {
		children[i] = billingMonthChildRow{OrganizationID: organizationID, MonthID: row.ID, StudentID: child.StudentID,
			FirstName: child.FirstName, LastName: child.LastName, SchoolClass: child.SchoolClass,
			RegisteredDays: child.RegisteredDays, MealDays: child.MealDays}
	}
	if _, err := db.NewInsert().Model(&children).Exec(ctx); err != nil {
		return time.Time{}, fmt.Errorf("insert meal month children: %w", err)
	}
	return row.ReleasedAt, nil
}

type traegerMealRow struct {
	SchoolID     int64          `bun:"school_id"`
	BillingMonth calendar.Date  `bun:"billing_month,type:date"`
	ReleasedAt   time.Time      `bun:"released_at"`
	StudentID    sql.NullInt64  `bun:"student_id"`
	FirstName    sql.NullString `bun:"first_name"`
	LastName     sql.NullString `bun:"last_name"`
	MealDays     sql.NullInt64  `bun:"meal_days"`
}

// TraegerMonths reads the released months of the Träger up to until. The
// function answers only in the Träger transaction of the Träger it names.
func (s *BillingMonthStore) TraegerMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]mealplan.ReleasedMealMonth, error) {
	db, err := s.traeger(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerMealRow
	if err := db.NewRaw(`SELECT school_id, billing_month, released_at, student_id, first_name, last_name, meal_days
		FROM schedule.traeger_meal_billing_children(?)`, until).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the released meal months of the Träger: %w", err)
	}
	result := []mealplan.ReleasedMealMonth{}
	for _, row := range rows {
		last := len(result) - 1
		if last < 0 || result[last].SchoolID != row.SchoolID || result[last].Month != row.BillingMonth {
			result = append(result, mealplan.ReleasedMealMonth{SchoolID: row.SchoolID, Month: row.BillingMonth,
				ReleasedAt: row.ReleasedAt, Children: []mealplan.ReleasedMealChild{}})
			last++
		}
		if row.StudentID.Valid {
			result[last].Children = append(result[last].Children, mealplan.ReleasedMealChild{StudentID: row.StudentID.Int64,
				FirstName: row.FirstName.String, LastName: row.LastName.String, MealDays: int(row.MealDays.Int64)})
		}
	}
	return result, nil
}
