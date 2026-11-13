package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/internal/domain"
)

// TraegerDocumentTemplateStore persists the document templates of a Träger
// (#4677). Every method runs in the caller's Träger transaction of
// organizationID and names the Träger in its own filter again.
type TraegerDocumentTemplateStore interface {
	// LatestTemplates reads the newest version of every kind that has one.
	LatestTemplates(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocumentTemplate, error)
	// TemplateVersions reads every version of a kind, the newest first.
	TemplateVersions(ctx context.Context, organizationID int64, kind templates.Kind) ([]organizationtenancy.TraegerDocumentTemplate, error)
	// LockTemplates holds the Träger's profile row until the end of the
	// transaction, so two saves number their versions one after the other.
	LockTemplates(ctx context.Context, organizationID int64) error
	InsertTemplateVersion(ctx context.Context, organizationID, accountID int64, kind templates.Kind, version int, template templates.Template) (organizationtenancy.TraegerDocumentTemplate, error)
	AppendAudit(ctx context.Context, entry domain.TraegerMasterDataAuditEntry) error
}
