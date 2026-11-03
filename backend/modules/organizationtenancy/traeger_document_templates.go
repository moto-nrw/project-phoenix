package organizationtenancy

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
)

// Document templates of a Träger (#4677, spec #4611 user stories 91 and 92,
// ADR 0149): for every kind of document the Träger-Büro writes subject,
// salutation, body and closing with placeholders. Every save is a new
// version; a version never changes, so an issued document keeps the text it
// was written with. Without a version of its own a kind uses the standard
// template of Document Rendering (version 0). Like the master data, the
// templates belong to the Träger and are read and written in its Träger
// transaction (ADR 0105).

var (
	// ErrTraegerDocumentTemplateKindNotFound reports a kind of document that
	// does not exist.
	ErrTraegerDocumentTemplateKindNotFound = errors.New("unknown kind of document")
	// ErrTraegerDocumentTemplateUnchanged reports a save that equals the
	// current version.
	ErrTraegerDocumentTemplateUnchanged = errors.New("the template equals its current version")
)

// TraegerDocumentTemplate is one version of the template of a kind. Version
// 0 is the standard template, which has no creation time.
type TraegerDocumentTemplate struct {
	Kind      templates.Kind `json:"kind"`
	Version   int            `json:"version"`
	Standard  bool           `json:"standard"`
	CreatedAt *time.Time     `json:"created_at"`
	templates.Template
}

// TraegerDocumentTemplateSummary is the current state of a kind.
type TraegerDocumentTemplateSummary struct {
	Kind      templates.Kind `json:"kind"`
	Version   int            `json:"version"`
	Standard  bool           `json:"standard"`
	UpdatedAt *time.Time     `json:"updated_at"`
}

// TraegerDocumentTemplateDetail is the current template of a kind with its
// placeholders and its versions, the newest first.
type TraegerDocumentTemplateDetail struct {
	Current      TraegerDocumentTemplate   `json:"current"`
	Standard     templates.Template        `json:"standard_template"`
	Placeholders []templates.Placeholder   `json:"placeholders"`
	Versions     []TraegerDocumentTemplate `json:"versions"`
}

// TraegerDocumentTemplates lets the Träger-Büro keep the document templates
// of its Träger. Every operation joins the caller's Träger transaction of
// organizationID; accountID is the acting account the log records.
type TraegerDocumentTemplates interface {
	// Overview reads the current state of every kind of document.
	Overview(ctx context.Context, organizationID int64) ([]TraegerDocumentTemplateSummary, error)
	// Detail reads the current template of a kind with its versions.
	Detail(ctx context.Context, organizationID int64, kind templates.Kind) (TraegerDocumentTemplateDetail, error)
	// AddVersion checks the template and stores it as the next version. Unknown
	// placeholders are refused as *templates.UnknownPlaceholdersError, a
	// missing or too long part as validation errors by field.
	AddVersion(ctx context.Context, organizationID, accountID int64, kind templates.Kind, template templates.Template) (TraegerDocumentTemplate, error)
	// Preview checks the template like AddVersion and fills it with the sample
	// values of its kind; it stores nothing.
	Preview(ctx context.Context, organizationID int64, kind templates.Kind, template templates.Template) (templates.Filled, error)
	// Current reads the template a document of the kind is written with
	// now: the newest version or the standard template.
	Current(ctx context.Context, organizationID int64, kind templates.Kind) (TraegerDocumentTemplate, error)
}
