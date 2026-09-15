// Package postgres is the File Storage owner's persistence adapter over
// documents.folders, documents.folder_roles, documents.folder_accounts,
// documents.files, documents.announcement_attachments and
// documents.announcement_attachment_cleanup. Every statement names its table
// statically and carries the tenant predicate: RLS is the first guard, the
// predicate the second, for connections that bypass RLS.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Database resolves the ambient transaction (or the root connection) and the
// tenant in context.
type Database func(context.Context) (bun.IDB, int64, error)

func requireTenant(db Database, ctx context.Context) (bun.IDB, int64, error) {
	idb, tenantID, err := db(ctx)
	if err != nil {
		return nil, 0, err
	}
	if tenantID <= 0 {
		return nil, 0, fmt.Errorf("file storage postgres: %w", domain.ErrTenantRequired)
	}
	return idb, tenantID, nil
}

func measure(started time.Time, rows int64) domain.OperationStats {
	return domain.OperationStats{Queries: 1, Rows: rows, StatementDuration: time.Since(started)}
}

// folderNameUniqueConstraint is the unique index behind the one-name-per-school rule.
const folderNameUniqueConstraint = "uq_documents_folders_name"

func classifyWriteError(err error) error {
	var postgresError pgdriver.Error
	if !errors.As(err, &postgresError) || !postgresError.IntegrityViolation() {
		return err
	}
	if postgresError.Field('n') == folderNameUniqueConstraint {
		return domain.ErrFolderNameTaken
	}
	return err
}
