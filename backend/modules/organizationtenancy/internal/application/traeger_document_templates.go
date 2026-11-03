package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// Document templates of a Träger (#4677, ADR 0150). Like the master data,
// the service opens no transaction: the Träger-Portal route runs it in the
// Träger transaction of the session's Träger. A save checks the template
// against the placeholders of its kind before it writes, locks the Träger,
// adds the next version and logs it in the same transaction. Versions never
// change; the newest one is the template documents are written with.

// TraegerDocumentTemplates implements organizationtenancy.TraegerDocumentTemplates.
type TraegerDocumentTemplates struct {
	store ports.TraegerDocumentTemplateStore
}

var _ organizationtenancy.TraegerDocumentTemplates = (*TraegerDocumentTemplates)(nil)

// NewTraegerDocumentTemplates builds the service.
func NewTraegerDocumentTemplates(store ports.TraegerDocumentTemplateStore) *TraegerDocumentTemplates {
	if store == nil {
		panic("organization tenancy: the document template store is required")
	}
	return &TraegerDocumentTemplates{store: store}
}

// Overview reads the current state of every kind of document.
func (t *TraegerDocumentTemplates) Overview(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocumentTemplateSummary, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	latest, err := t.store.LatestTemplates(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	byKind := make(map[templates.Kind]organizationtenancy.TraegerDocumentTemplate, len(latest))
	for _, template := range latest {
		byKind[template.Kind] = template
	}
	summaries := make([]organizationtenancy.TraegerDocumentTemplateSummary, 0, len(templates.Kinds()))
	for _, kind := range templates.Kinds() {
		current, ok := byKind[kind]
		if !ok {
			current = standardTemplate(kind)
		}
		summaries = append(summaries, organizationtenancy.TraegerDocumentTemplateSummary{
			Kind: kind, Version: current.Version, Standard: current.Standard, UpdatedAt: current.CreatedAt,
		})
	}
	return summaries, nil
}

// Detail reads the current template of a kind with its versions.
func (t *TraegerDocumentTemplates) Detail(ctx context.Context, organizationID int64, kind templates.Kind) (organizationtenancy.TraegerDocumentTemplateDetail, error) {
	versions, err := t.versions(ctx, organizationID, kind)
	if err != nil {
		return organizationtenancy.TraegerDocumentTemplateDetail{}, err
	}
	current := standardTemplate(kind)
	if len(versions) > 0 {
		current = versions[0]
	}
	return organizationtenancy.TraegerDocumentTemplateDetail{
		Current: current, Standard: templates.Default(kind), Placeholders: templates.Placeholders(kind), Versions: versions,
	}, nil
}

// Current reads the template a document of the kind is written with now.
func (t *TraegerDocumentTemplates) Current(ctx context.Context, organizationID int64, kind templates.Kind) (organizationtenancy.TraegerDocumentTemplate, error) {
	versions, err := t.versions(ctx, organizationID, kind)
	if err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, err
	}
	if len(versions) == 0 {
		return standardTemplate(kind), nil
	}
	return versions[0], nil
}

// AddVersion checks the template and stores it as the next version. A template
// equal to the current one adds no version.
func (t *TraegerDocumentTemplates) AddVersion(ctx context.Context, organizationID, accountID int64, kind templates.Kind, template templates.Template) (organizationtenancy.TraegerDocumentTemplate, error) {
	if organizationID <= 0 {
		return organizationtenancy.TraegerDocumentTemplate{}, errTraegerRequired
	}
	template, err := checkTemplate(kind, template)
	if err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, err
	}
	if err := t.store.LockTemplates(ctx, organizationID); err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, err
	}
	current, err := t.Current(ctx, organizationID, kind)
	if err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, err
	}
	if current.Template == template {
		return organizationtenancy.TraegerDocumentTemplate{}, organizationtenancy.ErrTraegerDocumentTemplateUnchanged
	}
	saved, err := t.store.InsertTemplateVersion(ctx, organizationID, accountID, kind, current.Version+1, template)
	if err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, err
	}
	oldValue, _ := json.Marshal(map[string]any{"kind": kind, "version": current.Version})
	newValue, _ := json.Marshal(map[string]any{"kind": kind, "version": saved.Version})
	if err := t.store.AppendAudit(ctx, domain.TraegerMasterDataAuditEntry{
		OrganizationID: organizationID, Subject: domain.TraegerAuditSubjectTemplate, Action: domain.TraegerAuditActionCreate,
		OldValue: oldValue, NewValue: newValue, ActorAccountID: accountID,
	}); err != nil {
		return organizationtenancy.TraegerDocumentTemplate{}, fmt.Errorf("organization tenancy: log document template: %w", err)
	}
	return saved, nil
}

// Preview checks the template and fills it with the sample values of its
// kind.
func (t *TraegerDocumentTemplates) Preview(_ context.Context, organizationID int64, kind templates.Kind, template templates.Template) (templates.Filled, error) {
	if organizationID <= 0 {
		return templates.Filled{}, errTraegerRequired
	}
	template, err := checkTemplate(kind, template)
	if err != nil {
		return templates.Filled{}, err
	}
	return templates.Fill(template, templates.Sample(kind)), nil
}

func (t *TraegerDocumentTemplates) versions(ctx context.Context, organizationID int64, kind templates.Kind) ([]organizationtenancy.TraegerDocumentTemplate, error) {
	if organizationID <= 0 {
		return nil, errTraegerRequired
	}
	if !templates.Known(kind) {
		return nil, organizationtenancy.ErrTraegerDocumentTemplateKindNotFound
	}
	return t.store.TemplateVersions(ctx, organizationID, kind)
}

// checkTemplate normalizes the template and reports a missing or too long
// part as validation errors by field; unknown placeholders stay a
// *templates.UnknownPlaceholdersError.
func checkTemplate(kind templates.Kind, template templates.Template) (templates.Template, error) {
	template = templates.Normalize(template)
	err := templates.Check(kind, template)
	var invalid *templates.InvalidError
	switch {
	case err == nil:
		return template, nil
	case errors.Is(err, templates.ErrUnknownKind):
		return template, organizationtenancy.ErrTraegerDocumentTemplateKindNotFound
	case errors.As(err, &invalid):
		fields := validation.Errors{}
		for field, reason := range invalid.Fields {
			fields[field] = errors.New(reason)
		}
		return template, fields
	}
	return template, err
}

func standardTemplate(kind templates.Kind) organizationtenancy.TraegerDocumentTemplate {
	return organizationtenancy.TraegerDocumentTemplate{Kind: kind, Standard: true, Template: templates.Default(kind)}
}
