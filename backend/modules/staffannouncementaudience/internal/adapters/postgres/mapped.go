package audience

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/contract"
)

// Rows maps the projection's rows onto the consumer's values and the
// consumer's announcement onto the projection's target. The root supplies
// the mapping, so neither owner imports the other's vocabulary.
// Mapped answers the projection's reads in the consumer's values.
type Mapped[A, V, S any] struct {
	projection *Projection
	rows       contract.Rows[A, V, S]
}

// Map binds rows to the projection. Every mapping is required.
func Map[A, V, S any](projection *Projection, rows contract.Rows[A, V, S]) Mapped[A, V, S] {
	if projection == nil || rows.Announcement == nil || rows.Target == nil || rows.View == nil || rows.Stats == nil {
		panic("announcement audience projection: projection and every row mapping are required")
	}
	return Mapped[A, V, S]{projection: projection, rows: rows}
}

// Unread maps Projection.Unread.
func (m Mapped[A, V, S]) Unread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) ([]A, S, error) {
	rows, stats, err := m.projection.Unread(ctx, userID, roles, tenantID, orgID)
	return mapRows(rows, m.rows.Announcement), m.rows.Stats(stats), err
}

// CountUnread maps Projection.CountUnread.
func (m Mapped[A, V, S]) CountUnread(ctx context.Context, userID int64, roles []string, tenantID, orgID int64) (int, S, error) {
	count, stats, err := m.projection.CountUnread(ctx, userID, roles, tenantID, orgID)
	return count, m.rows.Stats(stats), err
}

// TargetCount maps Projection.TargetCount.
func (m Mapped[A, V, S]) TargetCount(ctx context.Context, value A) (int, S, error) {
	count, stats, err := m.projection.TargetCount(ctx, m.rows.Target(value))
	return count, m.rows.Stats(stats), err
}

// ViewDetails maps Projection.ViewDetails.
func (m Mapped[A, V, S]) ViewDetails(ctx context.Context, announcementID int64) ([]V, S, error) {
	rows, stats, err := m.projection.ViewDetails(ctx, announcementID)
	return mapRows(rows, m.rows.View), m.rows.Stats(stats), err
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
