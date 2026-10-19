package operatordashboard

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/operatordashboard/contract"
)

// Rows maps the projection's rows onto the consumer's values. The owner's
// composition supplies it, so the projection never names the owner's types.
// Mapped answers the projection's reads in the consumer's values.
type Mapped[C, O, S, P any] struct {
	projection *Projection
	rows       contract.Rows[C, O, S, P]
}

// Map binds rows to the projection. Every mapping is required.
func Map[C, O, S, P any](projection *Projection, rows contract.Rows[C, O, S, P]) Mapped[C, O, S, P] {
	if projection == nil || rows.Counts == nil || rows.Organization == nil || rows.School == nil || rows.PWAUsage == nil {
		panic("organization tenancy operator dashboard: projection and every row mapping are required")
	}
	return Mapped[C, O, S, P]{projection: projection, rows: rows}
}

// contract.Counts maps Projection.Counts.
func (m Mapped[C, O, S, P]) Counts(ctx context.Context) (C, error) {
	counts, err := m.projection.Counts(ctx)
	return m.rows.Counts(counts), err
}

// OrganizationSummaries maps Projection.OrganizationSummaries.
func (m Mapped[C, O, S, P]) OrganizationSummaries(ctx context.Context) ([]O, error) {
	rows, err := m.projection.OrganizationSummaries(ctx)
	return mapRows(rows, m.rows.Organization), err
}

// SchoolSummaries maps Projection.SchoolSummaries.
func (m Mapped[C, O, S, P]) SchoolSummaries(ctx context.Context, organizationID *int64) ([]S, error) {
	rows, err := m.projection.SchoolSummaries(ctx, organizationID)
	return mapRows(rows, m.rows.School), err
}

// PWAUsage maps Projection.PWAUsage.
func (m Mapped[C, O, S, P]) PWAUsage(ctx context.Context, tenantID int64, window time.Duration) ([]P, error) {
	rows, err := m.projection.PWAUsage(ctx, tenantID, window)
	return mapRows(rows, m.rows.PWAUsage), err
}

// mapRows keeps a nil result nil, so a failed read stays empty.
func mapRows[T, U any](rows []T, convert func(T) U) []U {
	if rows == nil {
		return nil
	}
	result := make([]U, 0, len(rows))
	for _, row := range rows {
		result = append(result, convert(row))
	}
	return result
}

func (m Mapped[C, O, S, P]) AllSchoolsPWAUsage(ctx context.Context, window time.Duration) ([]P, error) {
	rows, err := m.projection.AllSchoolsPWAUsage(ctx, window)
	return mapRows(rows, m.rows.PWAUsage), err
}
