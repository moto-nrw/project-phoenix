package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// Document templates of a Träger (#4677, ADR 0152) in
// platform.traeger_document_templates, under the same Träger policy as the
// master data. A version is only ever added; a trigger refuses every change.

type traegerTemplateRow struct {
	Kind       string    `bun:"kind"`
	Version    int       `bun:"version"`
	Subject    string    `bun:"subject"`
	Salutation string    `bun:"salutation"`
	Body       string    `bun:"body"`
	Closing    string    `bun:"closing"`
	CreatedAt  time.Time `bun:"created_at"`
	// The layout of the table of amounts (#5691).
	GroupByChild   bool `bun:"invoice_group_by_child"`
	HideTimeModels bool `bun:"invoice_hide_time_models"`
	// The legal remedy of a fee notice (#4688).
	LegalRemedy string `bun:"legal_remedy"`
}

func (r traegerTemplateRow) toDomain() organizationtenancy.TraegerDocumentTemplate {
	createdAt := r.CreatedAt
	return organizationtenancy.TraegerDocumentTemplate{
		Kind: templates.Kind(r.Kind), Version: r.Version, CreatedAt: &createdAt,
		Template: templates.Template{
			Subject: r.Subject, Salutation: r.Salutation, Body: r.Body, Closing: r.Closing,
			Invoice:     templates.InvoiceLayout{GroupByChild: r.GroupByChild, HideTimeModels: r.HideTimeModels},
			LegalRemedy: r.LegalRemedy,
		},
	}
}

const (
	traegerTemplateColumns = `kind, version, subject, salutation, body, closing, created_at,
		invoice_group_by_child, invoice_hide_time_models, legal_remedy`
	latestTraegerTemplateQuery = `SELECT DISTINCT ON (kind) ` + traegerTemplateColumns + `
		FROM platform.traeger_document_templates
		WHERE organization_id = ?
		ORDER BY kind, version DESC`
	traegerTemplateVersionsQuery = `SELECT ` + traegerTemplateColumns + `
		FROM platform.traeger_document_templates
		WHERE organization_id = ? AND kind = ?
		ORDER BY version DESC`
	insertTraegerTemplateQuery = `INSERT INTO platform.traeger_document_templates
		(organization_id, kind, version, subject, salutation, body, closing, created_by,
			invoice_group_by_child, invoice_hide_time_models, legal_remedy)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING ` + traegerTemplateColumns
)

// LatestTemplates reads the newest version of every kind that has one.
func (s *TraegerMasterDataStore) LatestTemplates(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocumentTemplate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerTemplateRow
	if err := db.NewRaw(latestTraegerTemplateQuery, organizationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: read latest document templates: %w", err)
	}
	return templatesOf(rows), nil
}

// TemplateVersions reads every version of a kind, the newest first.
func (s *TraegerMasterDataStore) TemplateVersions(ctx context.Context, organizationID int64, kind templates.Kind) ([]organizationtenancy.TraegerDocumentTemplate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []traegerTemplateRow
	if err := db.NewRaw(traegerTemplateVersionsQuery, organizationID, string(kind)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: read document template versions: %w", err)
	}
	return templatesOf(rows), nil
}

// LockTemplates locks the Träger's profile row, creating it where missing.
func (s *TraegerMasterDataStore) LockTemplates(ctx context.Context, organizationID int64) error {
	_, err := s.LockProfile(ctx, organizationID, true)
	return err
}

// InsertTemplateVersion adds a version.
func (s *TraegerMasterDataStore) InsertTemplateVersion(ctx context.Context, organizationID, accountID int64, kind templates.Kind, version int, template templates.Template) (organizationtenancy.TraegerDocumentTemplate, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, err
	}
	var rows []traegerTemplateRow
	if err := db.NewRaw(insertTraegerTemplateQuery, organizationID, string(kind), version,
		template.Subject, template.Salutation, template.Body, template.Closing, traegerActor(accountID),
		template.Invoice.GroupByChild, template.Invoice.HideTimeModels, template.LegalRemedy).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, fmt.Errorf("organization tenancy postgres: insert document template: %w", err)
	}
	if len(rows) != 1 {
		return organizationtenancy.TraegerDocumentTemplate{}, fmt.Errorf("organization tenancy postgres: insert document template returned %d rows", len(rows))
	}
	return rows[0].toDomain(), nil
}

func templatesOf(rows []traegerTemplateRow) []organizationtenancy.TraegerDocumentTemplate {
	result := make([]organizationtenancy.TraegerDocumentTemplate, len(rows))
	for i, row := range rows {
		result[i] = row.toDomain()
	}
	return result
}
