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

// billingMonthUnique keeps a version of a month of a school to one release.
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
	Version             int           `bun:"version"`
	Reason              *string       `bun:"reason"`
}

func (row billingMonthRow) version() domain.ReleaseVersion {
	reason := ""
	if row.Reason != nil {
		reason = *row.Reason
	}
	return domain.ReleaseVersion{Version: row.Version, ReleasedAt: row.ReleasedAt, Reason: reason, ChildCount: row.ChildCount,
		MealCount: row.MealCount}
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
	if err := db.NewSelect().Model(&rows).DistinctOn("billing_month").Where("school_id = ?", schoolID).
		Where("billing_month BETWEEN ? AND ?", from, to).OrderExpr("billing_month DESC, version DESC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read released meal months: %w", err)
	}
	result := make([]domain.StoredBillingMonth, len(rows))
	for i, row := range rows {
		result[i] = domain.StoredBillingMonth{Month: row.BillingMonth, ReleasedAt: row.ReleasedAt, ChildCount: row.ChildCount,
			MealCount: row.MealCount, Version: row.Version}
	}
	return result, nil
}

// ReleasedMonth reads one released month of the school with its children.
func (s *BillingMonthStore) ReleasedMonth(ctx context.Context, month calendar.Date) (domain.StoredBillingMonth, bool, error) {
	db, schoolID, err := s.school(ctx)
	if err != nil {
		return domain.StoredBillingMonth{}, false, err
	}
	var versions []billingMonthRow
	if err := db.NewSelect().Model(&versions).Where("school_id = ?", schoolID).Where("billing_month = ?", month).
		OrderExpr("version DESC").Scan(ctx); err != nil {
		return domain.StoredBillingMonth{}, false, fmt.Errorf("read released meal month: %w", err)
	}
	if len(versions) == 0 {
		return domain.StoredBillingMonth{}, false, nil
	}
	row := versions[0]
	var children []billingMonthChildRow
	if err := db.NewSelect().Model(&children).Where("organization_id = ?", row.OrganizationID).Where("month_id = ?", row.ID).
		OrderExpr("lower(last_name), lower(first_name), student_id").Scan(ctx); err != nil {
		return domain.StoredBillingMonth{}, false, fmt.Errorf("read released meal month children: %w", err)
	}
	latest := row.version()
	stored := domain.StoredBillingMonth{Month: row.BillingMonth, ReleasedAt: row.ReleasedAt, ChildCount: row.ChildCount,
		MealCount: row.MealCount, Version: latest.Version, Reason: latest.Reason, Children: make([]domain.BillingChild, len(children)),
		Versions: make([]domain.ReleaseVersion, len(versions))}
	for i, version := range versions {
		stored.Versions[i] = version.version()
	}
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
	version := max(month.Version, 1)
	row := &billingMonthRow{OrganizationID: organizationID, SchoolID: schoolID, BillingMonth: month.Month,
		ReleasedByAccountID: &accountID, ChildCount: month.ChildCount, MealCount: month.MealCount, Version: version}
	if version > 1 {
		row.Reason = &month.Reason
	}
	if _, err := db.NewInsert().Model(row).Returning("id, released_at").Exec(ctx); err != nil {
		var pgErr pgdriver.Error
		if errors.As(err, &pgErr) && pgErr.Field('n') == billingMonthUnique {
			// Another release of the month came first: the first release
			// is taken, a correction no longer follows the version seen.
			if version == 1 {
				return time.Time{}, mealplan.ErrBillingMonthReleased
			}
			return time.Time{}, mealplan.ErrBillingMonthChanged
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
	Version      int            `bun:"version"`
	StudentID    sql.NullInt64  `bun:"student_id"`
	FirstName    sql.NullString `bun:"first_name"`
	LastName     sql.NullString `bun:"last_name"`
	MealDays     sql.NullInt64  `bun:"meal_days"`
}

// TraegerMonths reads the released months of the Träger within [from, until]. The
// function answers only in the Träger transaction of the Träger it names.
func (s *BillingMonthStore) TraegerMonths(ctx context.Context, organizationID int64, from, until calendar.Date) ([]domain.ReleasedMealRow, error) {
	db, err := s.traeger(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerMealRow
	if err := db.NewRaw(`SELECT school_id, billing_month, released_at, version, student_id, first_name, last_name, meal_days
		FROM schedule.traeger_meal_billing_children(?)
		WHERE billing_month >= ?
		ORDER BY school_id, billing_month, student_id`, until, from).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("read the released meal months of the Träger: %w", err)
	}
	result := make([]domain.ReleasedMealRow, len(rows))
	for i, row := range rows {
		result[i] = domain.ReleasedMealRow{SchoolID: row.SchoolID, Month: row.BillingMonth, ReleasedAt: row.ReleasedAt, Version: row.Version}
		if row.StudentID.Valid {
			result[i].Child = &domain.ReleasedMealChild{StudentID: row.StudentID.Int64, FirstName: row.FirstName.String,
				LastName: row.LastName.String, MealDays: int(row.MealDays.Int64)}
		}
	}
	return result, nil
}

// NewTraegerDatabase resolves the Träger transaction of organizationID from
// the runtime's accessors; any other transaction is an error.
func NewTraegerDatabase(transactionFromContext func(context.Context) (any, bool), traegerOf func(context.Context) int64) TraegerDatabase {
	return func(ctx context.Context, organizationID int64) (bun.IDB, error) {
		if organizationID <= 0 || traegerOf(ctx) != organizationID {
			return nil, errors.New("meal plan postgres: the Träger transaction of this Träger is required")
		}
		transaction, ok := transactionFromContext(ctx)
		if !ok {
			return nil, errors.New("meal plan postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("meal plan postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	}
}
