package postgres

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The kind of document of every service of a Träger (#4688) in
// parent_billing.document_kinds, under the Träger policy. A service without a
// row keeps its default.

type documentKindRow struct {
	bun.BaseModel `bun:"table:parent_billing.document_kinds"`

	OrganizationID int64     `bun:"organization_id,pk"`
	Service        string    `bun:"service,pk"`
	DocumentKind   string    `bun:"document_kind"`
	UpdatedAt      time.Time `bun:"updated_at,nullzero,default:current_timestamp"`
}

// DocumentKinds reads the chosen kind of every service of the Träger.
func (s *Store) DocumentKinds(ctx context.Context, organizationID int64) (map[string]parentbilling.ServiceDocumentKind, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []documentKindRow
	if err := db.NewSelect().Model(&rows).Where("organization_id = ?", organizationID).Scan(ctx); err != nil {
		return nil, err
	}
	kinds := make(map[string]parentbilling.ServiceDocumentKind, len(rows))
	for _, row := range rows {
		updatedAt := row.UpdatedAt
		kinds[row.Service] = parentbilling.ServiceDocumentKind{Service: row.Service, DocumentKind: row.DocumentKind, UpdatedAt: &updatedAt}
	}
	return kinds, nil
}

// SetDocumentKind stores or replaces the kind of a service.
func (s *Store) SetDocumentKind(ctx context.Context, organizationID int64, service, documentKind string) error {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	row := &documentKindRow{OrganizationID: organizationID, Service: service, DocumentKind: documentKind}
	_, err = db.NewInsert().Model(row).
		On("CONFLICT (organization_id, service) DO UPDATE").Set("document_kind = EXCLUDED.document_kind").
		Exec(ctx)
	return err
}
