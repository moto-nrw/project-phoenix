// Package filestore persists the school file storage (#2596): folders with
// their share lists, and the files inside them through the shared document
// repository.
package filestore

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/database/repositories/documents"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	"github.com/uptrace/bun"
)

const (
	folderTableExpr        = `documents.folders AS "folder"`
	folderRoleTableExpr    = `documents.folder_roles AS "folder_role"`
	folderAccountTableExpr = `documents.folder_accounts AS "folder_account"`

	// visibleFolderCondition is the whole visibility rule in one place. It
	// takes the viewer's account id twice (accounts share, role share).
	//
	// A "selected" folder is visible through a direct account share or through
	// any role the account holds AT THIS SCHOOL (auth.account_roles carries
	// tenant_id), so a role held at another school never opens a folder here.
	// Every viewer must also retain an active account-to-school mapping. A JWT
	// can outlive a membership change, so its tenant claim alone is not enough
	// to keep a former staff member able to read shared files.
	activeViewerMembershipCondition = `EXISTS (
		SELECT 1 FROM auth.account_tenants AS account_tenant
		WHERE account_tenant.tenant_id = "folder".tenant_id
			AND account_tenant.account_id = ?
			AND account_tenant.status = 'active'
	)`

	visibleFolderCondition = `(
		"folder".visibility = 'all_staff'
		OR (
			"folder".visibility = 'selected' AND (
				EXISTS (
					SELECT 1 FROM documents.folder_accounts AS fa
					WHERE fa.tenant_id = "folder".tenant_id
						AND fa.folder_id = "folder".id
						AND fa.account_id = ?
				)
				OR EXISTS (
					SELECT 1 FROM documents.folder_roles AS fr
					JOIN auth.account_roles AS ar
						ON ar.tenant_id = fr.tenant_id AND ar.role_id = fr.role_id
					WHERE fr.tenant_id = "folder".tenant_id
						AND fr.folder_id = "folder".id
						AND ar.account_id = ?
				)
			)
		)
	)`
)

// FolderRepository implements filestore.FolderRepository.
type FolderRepository struct {
	*base.Repository[*filestore.Folder]
	db *bun.DB
}

// NewFolderRepository creates the folder repository.
func NewFolderRepository(db *bun.DB) filestore.FolderRepository {
	repo := base.NewRepository[*filestore.Folder](db, "documents.folders", "Folder")
	repo.TenantScoped = true
	return &FolderRepository{Repository: repo, db: db}
}

// Create persists a folder and hydrates its database-generated values.
func (r *FolderRepository) Create(ctx context.Context, folder *filestore.Folder) error {
	if err := folder.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, folder)
	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(folder).
		ModelTableExpr(folderTableExpr).
		Returning("*").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create folder", Err: err}
	}
	return nil
}

// Update rewrites name and visibility of a folder.
func (r *FolderRepository) Update(ctx context.Context, folder *filestore.Folder) error {
	if err := folder.Validate(); err != nil {
		return err
	}
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(folder).
		ModelTableExpr(folderTableExpr).
		Column("name", "visibility").
		Where(`"folder".id = ?`, folder.ID).
		Returning("*")
	query = base.WithTenantFilter(ctx, query, "folder")
	res, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update folder", Err: err}
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return &modelBase.DatabaseError{Op: "update folder", Err: sql.ErrNoRows}
	}
	return nil
}

// Delete removes the folder row; files and share lists cascade.
func (r *FolderRepository) Delete(ctx context.Context, folderID int64) error {
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*filestore.Folder)(nil)).
		ModelTableExpr(folderTableExpr).
		Where(`"folder".id = ?`, folderID)
	query = base.WithTenantFilter(ctx, query, "folder")
	res, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete folder", Err: err}
	}
	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return &modelBase.DatabaseError{Op: "delete folder", Err: sql.ErrNoRows}
	}
	return nil
}

// FindByID loads one folder of the tenant.
func (r *FolderRepository) FindByID(ctx context.Context, folderID int64) (*filestore.Folder, error) {
	folder := new(filestore.Folder)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(folder).
		ModelTableExpr(folderTableExpr).
		Where(`"folder".id = ?`, folderID)
	query = base.WithTenantFilter(ctx, query, "folder")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find folder", Err: err}
	}
	return folder, nil
}

func (r *FolderRepository) ListVisible(ctx context.Context, viewer filestore.Viewer) ([]*filestore.FolderListItem, error) {
	var rows []*filestore.FolderListItem
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(folderTableExpr).
		ColumnExpr(`"folder".*`).
		ColumnExpr(`(
			SELECT COUNT(*) FROM documents.files AS fi
			WHERE fi.tenant_id = "folder".tenant_id
				AND fi.folder_id = "folder".id
				AND fi.deleted_at IS NULL
		) AS file_count`).
		OrderExpr(`lower("folder".name) ASC, "folder".id ASC`)
	query = query.Where(activeViewerMembershipCondition, viewer.AccountID)
	if !viewer.Manager {
		query = query.Where(visibleFolderCondition, viewer.AccountID, viewer.AccountID)
	}
	query = base.WithTenantFilter(ctx, query, "folder")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list visible folders", Err: err}
	}
	return rows, nil
}

func (r *FolderRepository) IsVisible(ctx context.Context, folderID int64, viewer filestore.Viewer) (bool, error) {
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*filestore.Folder)(nil)).
		ModelTableExpr(folderTableExpr).
		Where(`"folder".id = ?`, folderID)
	query = query.Where(activeViewerMembershipCondition, viewer.AccountID)
	if !viewer.Manager {
		query = query.Where(visibleFolderCondition, viewer.AccountID, viewer.AccountID)
	}
	query = base.WithTenantFilter(ctx, query, "folder")
	exists, err := query.Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check folder visibility", Err: err}
	}
	return exists, nil
}

// ReplaceAudience rewrites both share lists. The delete-then-insert is one
// statement pair inside the caller's transaction, so a reader never observes
// a half-written list.
func (r *FolderRepository) ReplaceAudience(ctx context.Context, folderID int64, audience filestore.Audience) error {
	db := base.GetDB(ctx, r.db)

	rolesDelete := db.NewDelete().
		Model((*filestore.FolderRole)(nil)).
		ModelTableExpr(folderRoleTableExpr).
		Where(`"folder_role".folder_id = ?`, folderID)
	rolesDelete = base.WithTenantFilter(ctx, rolesDelete, "folder_role")
	if _, err := rolesDelete.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear folder roles", Err: err}
	}

	accountsDelete := db.NewDelete().
		Model((*filestore.FolderAccount)(nil)).
		ModelTableExpr(folderAccountTableExpr).
		Where(`"folder_account".folder_id = ?`, folderID)
	accountsDelete = base.WithTenantFilter(ctx, accountsDelete, "folder_account")
	if _, err := accountsDelete.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear folder accounts", Err: err}
	}

	if len(audience.RoleIDs) > 0 {
		rows := make([]*filestore.FolderRole, 0, len(audience.RoleIDs))
		for _, id := range audience.RoleIDs {
			row := &filestore.FolderRole{FolderID: folderID, RoleID: id}
			base.EnsureTenantID(ctx, row)
			rows = append(rows, row)
		}
		if _, err := db.NewInsert().
			Model(&rows).
			ModelTableExpr(folderRoleTableExpr).
			On("CONFLICT DO NOTHING").
			Exec(ctx); err != nil {
			return &modelBase.DatabaseError{Op: "insert folder roles", Err: err}
		}
	}

	if len(audience.AccountIDs) > 0 {
		rows := make([]*filestore.FolderAccount, 0, len(audience.AccountIDs))
		for _, id := range audience.AccountIDs {
			row := &filestore.FolderAccount{FolderID: folderID, AccountID: id}
			base.EnsureTenantID(ctx, row)
			rows = append(rows, row)
		}
		if _, err := db.NewInsert().
			Model(&rows).
			ModelTableExpr(folderAccountTableExpr).
			On("CONFLICT DO NOTHING").
			Exec(ctx); err != nil {
			return &modelBase.DatabaseError{Op: "insert folder accounts", Err: err}
		}
	}
	return nil
}

func (r *FolderRepository) GetAudience(ctx context.Context, folderIDs []int64) (map[int64]filestore.Audience, error) {
	result := make(map[int64]filestore.Audience, len(folderIDs))
	if len(folderIDs) == 0 {
		return result, nil
	}
	db := base.GetDB(ctx, r.db)

	var roles []*filestore.FolderRole
	rolesQuery := db.NewSelect().
		Model(&roles).
		ModelTableExpr(folderRoleTableExpr).
		Where(`"folder_role".folder_id IN (?)`, bun.List(folderIDs)).
		OrderExpr(`"folder_role".role_id ASC`)
	rolesQuery = base.WithTenantFilter(ctx, rolesQuery, "folder_role")
	if err := rolesQuery.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list folder roles", Err: err}
	}

	var accounts []*filestore.FolderAccount
	accountsQuery := db.NewSelect().
		Model(&accounts).
		ModelTableExpr(folderAccountTableExpr).
		Where(`"folder_account".folder_id IN (?)`, bun.List(folderIDs)).
		OrderExpr(`"folder_account".account_id ASC`)
	accountsQuery = base.WithTenantFilter(ctx, accountsQuery, "folder_account")
	if err := accountsQuery.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list folder accounts", Err: err}
	}

	for _, row := range roles {
		entry := result[row.FolderID]
		entry.RoleIDs = append(entry.RoleIDs, row.RoleID)
		result[row.FolderID] = entry
	}
	for _, row := range accounts {
		entry := result[row.FolderID]
		entry.AccountIDs = append(entry.AccountIDs, row.AccountID)
		result[row.FolderID] = entry
	}
	return result, nil
}

// ListAudienceRoles returns the roles a folder can be shared with. RLS on
// auth.roles already narrows the set to the school's own roles plus the
// system roles; the guardian tier is dropped because parents never reach the
// tenant portal.
func (r *FolderRepository) ListAudienceRoles(ctx context.Context) ([]*filestore.AudienceRole, error) {
	var rows []*filestore.AudienceRole
	err := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`auth.roles AS "role"`).
		ColumnExpr(`"role".id, "role".name`).
		Where(`"role".name <> 'guardian'`).
		Where(`("role".base_role IS NULL OR "role".base_role <> 'guardian')`).
		OrderExpr(`lower("role".name) ASC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list audience roles", Err: err}
	}
	return rows, nil
}

// ListAudienceAccounts returns every account with an active mapping to the
// school that is backed by a person, so the picker shows names rather than
// e-mail addresses.
func (r *FolderRepository) ListAudienceAccounts(ctx context.Context) ([]*filestore.AudienceAccount, error) {
	var rows []*filestore.AudienceAccount
	query := base.GetDB(ctx, r.db).NewSelect().
		TableExpr(`auth.account_tenants AS "mapping"`).
		ColumnExpr(`"mapping".account_id, "person".first_name, "person".last_name`).
		Join(`JOIN users.persons AS "person" ON "person".account_id = "mapping".account_id AND "person".tenant_id = "mapping".tenant_id AND "person".deleted_at IS NULL`).
		Where(`"mapping".status = 'active'`).
		OrderExpr(`lower("person".last_name) ASC, lower("person".first_name) ASC`)
	query = base.WithTenantFilter(ctx, query, "mapping")
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list audience accounts", Err: err}
	}
	return rows, nil
}

// FileRepository implements filestore.FileRepository over the shared
// document repository.
type FileRepository struct {
	*documents.Repository[*filestore.File, *filestore.FileCleanup]
	db *bun.DB
}

// NewFileRepository creates the file metadata repository.
func NewFileRepository(db *bun.DB) filestore.FileRepository {
	return &FileRepository{
		Repository: documents.NewRepository[*filestore.File, *filestore.FileCleanup](db, documents.Config{
			Table:        "documents.files",
			Alias:        "file",
			OwnerColumn:  "folder_id",
			CleanupTable: "documents.file_cleanup",
			CleanupAlias: "file_cleanup",
		}),
		db: db,
	}
}

// TotalStoredBytes sums every file whose bytes still occupy the storage
// backend. Soft-deleted rows count until the sweep has removed their object.
func (r *FileRepository) TotalStoredBytes(ctx context.Context) (int64, error) {
	var total sql.NullInt64
	query := base.GetDB(ctx, r.db).NewSelect().
		Model((*filestore.File)(nil)).
		ModelTableExpr(`documents.files AS "file"`).
		ColumnExpr(`COALESCE(SUM("file".size_bytes), 0)`).
		Where(`"file".file_deleted_at IS NULL`).
		WhereAllWithDeleted()
	query = base.WithTenantFilter(ctx, query, "file")
	if err := query.Scan(ctx, &total); err != nil {
		return 0, &modelBase.DatabaseError{Op: "sum stored file bytes", Err: err}
	}
	return total.Int64, nil
}
