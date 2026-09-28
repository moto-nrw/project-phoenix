package enrollment

import (
	"context"
	"errors"
)

// ErrNoActiveSchema is returned by GetActive when no active form schema
// exists for the tenant. Callers should treat this as "feature not configured
// yet".
var ErrNoActiveSchema = errors.New("no active form schema for tenant")

// FormSchemaAdministration manages form-schema versioning. GetActive feeds
// the public form renderer and admin pre-fill; CreateSchema/UpdateSchema
// create new schemas and versions. Every write runs in the caller's tenant
// transaction, or opens one for a standalone caller.
type FormSchemaAdministration interface {
	// GetActive returns the currently-active form schema for the
	// tenant in context, or ErrNoActiveSchema if none exists.
	GetActive(ctx context.Context) (*FormSchema, error)

	// SchemaVersion returns a specific schema version. Used to render
	// already-submitted requests against their pinned version.
	SchemaVersion(ctx context.Context, id int64) (*FormSchema, error)

	// ListVersions returns all schema versions for the tenant in
	// context, newest-first. Powers the admin "version history" view.
	ListVersions(ctx context.Context) ([]*FormSchema, error)

	// CreateSchema creates a new logical schema (version 1) under the
	// given name. Use this when the admin clicks "Neues Formular" on
	// the schema list page. Names are unique per tenant by convention
	// but not by DB constraint. It rejects an attempt to create a new
	// schema with a name that already exists; callers must use
	// UpdateSchema to add another version to an existing schema instead.
	CreateSchema(ctx context.Context, name string, fields []FormField, createdBy int64, coreRequirements ...CoreRequirements) (*FormSchema, error)
	CreateSchemaWithLegal(ctx context.Context, name string, fields []FormField, createdBy int64, coreRequirements CoreRequirements, legalBlocks []FormLegalBlock) (*FormSchema, error)

	// UpdateSchema publishes a new version of an existing schema,
	// looked up by id. The new row inherits the source row's name,
	// uses max(version)+1 for that name, and is marked active. Phases
	// using an older version of the same logical schema are repointed
	// to the new row, while previously-submitted requests keep their
	// schema reference intact.
	UpdateSchema(ctx context.Context, id int64, fields []FormField, updatedBy int64, coreRequirements ...CoreRequirements) (*FormSchema, error)
	UpdateSchemaWithLegal(ctx context.Context, id int64, fields []FormField, updatedBy int64, coreRequirements *CoreRequirements, legalBlocks *[]FormLegalBlock) (*FormSchema, error)

	// RenameSchema renames the logical schema selected by id. All version
	// rows sharing the source's name are renamed atomically so the version
	// lineage stays intact. Renaming to a name already used by a different
	// schema returns ErrFormSchemaNameExists. The returned schema is the
	// row identified by id with its updated name.
	RenameSchema(ctx context.Context, id int64, newName string) (*FormSchema, error)

	// DeleteSchema removes every version of the logical schema selected
	// by id. It refuses deletion when any version is used by a phase or
	// historical request.
	DeleteSchema(ctx context.Context, id int64) error

	// PublishForm creates or updates a form schema (POST /schema). A
	// non-empty Name creates a new named schema (version 1). An empty
	// Name targets the tenant's active schema (legacy single-schema
	// flow): it updates the active schema when one exists, reuses an
	// existing "Standardformular" lineage, or creates the default
	// "Standardformular" schema.
	PublishForm(ctx context.Context, in PublishFormInput) (*FormSchema, error)

	// PublishFormVersion publishes a new version of an existing schema
	// (PUT /schema/{id}). When Name is set and non-blank it renames the
	// whole lineage first, in the SAME transaction as the publish, so a
	// failed publish rolls the rename back. A rename failure beyond the
	// name-exists / not-found sentinels is wrapped in RenameStepError so
	// the caller can distinguish rename infrastructure faults from
	// publish validation errors.
	PublishFormVersion(ctx context.Context, in PublishFormVersionInput) (*FormSchema, error)
}

// PublishFormInput carries the fields for the create-or-update publish
// flow that POST /schema drives.
type PublishFormInput struct {
	Name             string
	Fields           []FormField
	CoreRequirements *CoreRequirements
	LegalBlocks      *[]FormLegalBlock
	ActorID          int64
}

// PublishFormVersionInput carries the fields for the combined
// rename+publish flow that PUT /schema/{id} drives. Name is optional:
// nil or blank skips the rename step.
type PublishFormVersionInput struct {
	ID               int64
	Name             *string
	Fields           []FormField
	CoreRequirements *CoreRequirements
	LegalBlocks      *[]FormLegalBlock
	ActorID          int64
}

// CopyFormSchema returns a shallow copy of a schema version, or nil.
func CopyFormSchema(value *FormSchema) *FormSchema {
	if value == nil {
		return nil
	}
	schema := *value
	return &schema
}

// RenameStepError tags a failure originating from the rename step of a
// combined rename+publish (PublishFormVersion). It unwraps so errors.Is
// still matches the rename sentinels (name-exists, not-found); a caller
// maps a bare rename infrastructure fault (lock/read/exec) to a 5xx while
// publish/validation failures keep their 400 contract.
type RenameStepError struct{ err error }

// NewRenameStepError tags err as a failure of the rename step.
func NewRenameStepError(err error) RenameStepError { return RenameStepError{err: err} }

func (e RenameStepError) Error() string { return e.err.Error() }
func (e RenameStepError) Unwrap() error { return e.err }
