package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// TraegerGroupNames reads the groups of a school set on the caller's
// administrative transaction, for the Träger's occupancy (#4676).
type TraegerGroupNames struct {
	database func(context.Context) (any, bool)
	admin    func(context.Context) bool
}

var _ schoolstructure.TraegerGroupNames = TraegerGroupNames{}

// NewTraegerGroupNames builds the read over the caller's transaction and its
// administrative mark.
func NewTraegerGroupNames(transaction func(context.Context) (any, bool), admin func(context.Context) bool) TraegerGroupNames {
	return TraegerGroupNames{database: transaction, admin: admin}
}

// groupsByTenantSQL takes the school set; the name column sorts in German.
const groupsByTenantSQL = `SELECT tenant_id, id, name FROM education.groups
WHERE tenant_id = ANY(?) ORDER BY tenant_id, name, id`

// GroupsByTenant lists the groups of each school, ordered by name.
func (g TraegerGroupNames) GroupsByTenant(ctx context.Context, schools []int64) (map[int64][]schoolstructure.NamedGroup, error) {
	result := make(map[int64][]schoolstructure.NamedGroup, len(schools))
	if len(schools) == 0 {
		return result, nil
	}
	if !g.admin(ctx) {
		return nil, errors.New("school structure postgres: group names need the administrative transaction")
	}
	db, err := NewAmbientDatabase(g.database)(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID int64  `bun:"tenant_id"`
		ID       int64  `bun:"id"`
		Name     string `bun:"name"`
	}
	if err := db.NewRaw(groupsByTenantSQL, pgdialect.Array(schools)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("school structure postgres: group names: %w", err)
	}
	for _, row := range rows {
		result[row.TenantID] = append(result[row.TenantID], schoolstructure.NamedGroup{ID: row.ID, Name: row.Name})
	}
	return result, nil
}
