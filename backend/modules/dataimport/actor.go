package dataimport

import (
	"context"
	"errors"
)

// importerPermissionsKey keeps the authenticated importer's permissions
// available while the generic import service processes individual rows.
type importerPermissionsKey struct{}

// ContextWithImporterPermissions stores the authenticated importer's
// permissions for staff invitation authorization.
func ContextWithImporterPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, importerPermissionsKey{}, permissions)
}

// ImporterPermissionsFromContext returns the authenticated importer's
// permissions, or nil when no authenticated importer was supplied.
func ImporterPermissionsFromContext(ctx context.Context) []string {
	permissions, _ := ctx.Value(importerPermissionsKey{}).([]string)
	return permissions
}

// ErrImportModeForbidden reports that the importer lacks a permission the
// requested import mode needs. Handlers map it to 403.
var ErrImportModeForbidden = errors.New("import mode not permitted")
