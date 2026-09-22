package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// BillingDependencies are the collaborators of the billing report (#2791).
// Every field except Logger is required.
type BillingDependencies struct {
	Transaction ports.Transaction
	Store       ports.BillingStore
	Counts      ports.BillingCounts
	Clock       ports.BillingClock
	// Location is the clock of the school calendar (Europe/Berlin): the key
	// date and the capture hour are read on it.
	Location *time.Location
	Logger   *slog.Logger
}

// Billing implements the operator billing report.
type Billing struct {
	tx       ports.Transaction
	store    ports.BillingStore
	counts   ports.BillingCounts
	clock    ports.BillingClock
	location *time.Location
	logger   *slog.Logger
}

var _ organizationtenancy.BillingReport = (*Billing)(nil)

// NewBilling builds the billing report.
func NewBilling(deps BillingDependencies) (*Billing, error) {
	if deps.Transaction == nil || deps.Store == nil || deps.Counts == nil || deps.Clock == nil || deps.Location == nil {
		return nil, errors.New("organization tenancy billing: all dependencies are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Billing{tx: deps.Transaction, store: deps.Store, counts: deps.Counts, clock: deps.Clock, location: deps.Location, logger: logger}, nil
}

// BillingKeyDay reads the key day and the next key date it produces.
func (b *Billing) BillingKeyDay(ctx context.Context) (organizationtenancy.BillingKeyDay, error) {
	var settings domain.BillingSettings
	err := b.tx.RunAdmin(ctx, func(adminCtx context.Context) error {
		var err error
		settings, err = b.store.BillingSettings(adminCtx)
		return err
	})
	if err != nil {
		return organizationtenancy.BillingKeyDay{}, err
	}
	return b.keyDay(settings), nil
}

// SetBillingKeyDay changes the key day for every school.
func (b *Billing) SetBillingKeyDay(ctx context.Context, keyDay int, operatorID int64) (organizationtenancy.BillingKeyDay, error) {
	if !domain.ValidBillingKeyDay(keyDay) {
		return organizationtenancy.BillingKeyDay{}, organizationtenancy.ErrInvalidBillingKeyDay
	}
	var settings domain.BillingSettings
	err := b.tx.RunAdmin(ctx, func(adminCtx context.Context) error {
		var err error
		settings, err = b.store.UpdateBillingKeyDay(adminCtx, keyDay, operatorID)
		return err
	})
	if err != nil {
		return organizationtenancy.BillingKeyDay{}, err
	}
	b.logger.Info("billing key day changed",
		"key_day", settings.KeyDay,
		"operator_id", operatorID,
	)
	return b.keyDay(settings), nil
}

// ListBillingKeyDateCounts reads every captured row.
func (b *Billing) ListBillingKeyDateCounts(ctx context.Context) ([]organizationtenancy.BillingKeyDateCount, error) {
	var counts []domain.BillingKeyDateCount
	err := b.tx.RunAdmin(ctx, func(adminCtx context.Context) error {
		var err error
		counts, err = b.store.ListBillingKeyDateCounts(adminCtx)
		return err
	})
	if err != nil {
		return nil, err
	}
	result := make([]organizationtenancy.BillingKeyDateCount, 0, len(counts))
	for _, count := range counts {
		count.RecordedAt = count.RecordedAt.In(b.location)
		result = append(result, organizationtenancy.BillingKeyDateCount(count))
	}
	return result, nil
}

// RecordDueBillingKeyDates captures the current month when its key date is
// due. The owners' live counts are read in the same transaction that writes
// the rows, so all schools of one run share one snapshot.
func (b *Billing) RecordDueBillingKeyDates(ctx context.Context, now time.Time) (int, error) {
	written := 0
	err := b.tx.RunAdmin(ctx, func(adminCtx context.Context) error {
		if err := b.store.UseRepeatableReadSnapshot(adminCtx); err != nil {
			return err
		}
		settings, err := b.store.BillingSettings(adminCtx)
		if err != nil {
			return err
		}
		localNow := now.In(b.location)
		keyDate, due := domain.DueBillingKeyDate(localNow, settings.KeyDay)
		if !due || !domain.BillingKeyDateWasConfigured(settings.UpdatedAt, localNow, settings.KeyDay) {
			return nil
		}
		schools, err := b.store.SchoolsMissingBillingPeriod(adminCtx, keyDate)
		if err != nil || len(schools) == 0 {
			return err
		}
		rows, err := b.capture(adminCtx, schools, keyDate)
		if err != nil {
			return err
		}
		written, err = b.store.InsertBillingKeyDateCounts(adminCtx, rows)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("record billing key-date counts: %w", err)
	}
	if written > 0 {
		b.logger.Info("billing key-date counts recorded",
			"rows", written,
		)
	}
	return written, nil
}

func (b *Billing) capture(ctx context.Context, schools []domain.BillingSchool, keyDate string) ([]domain.BillingKeyDateCount, error) {
	students, err := b.counts.CountActiveStudentsByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("count active students: %w", err)
	}
	terminals, err := b.counts.CountActiveTerminalsByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("count active terminals: %w", err)
	}
	period := domain.BillingPeriod(keyDate)
	rows := make([]domain.BillingKeyDateCount, 0, len(schools))
	for _, school := range schools {
		rows = append(rows, domain.BillingKeyDateCount{
			SchoolID: school.ID, SchoolName: school.Name, OrganizationName: school.OrganizationName,
			Period: period, KeyDate: keyDate,
			ActiveStudents: students[school.ID], ActiveTerminals: terminals[school.ID],
		})
	}
	return rows, nil
}

func (b *Billing) keyDay(settings domain.BillingSettings) organizationtenancy.BillingKeyDay {
	local := b.clock().In(b.location)
	return organizationtenancy.BillingKeyDay{
		Day:                 settings.KeyDay,
		NextKeyDate:         domain.NextBillingKeyDate(local, settings.KeyDay),
		UpdatedAt:           settings.UpdatedAt,
		UpdatedByOperatorID: settings.UpdatedByOperatorID,
	}
}
