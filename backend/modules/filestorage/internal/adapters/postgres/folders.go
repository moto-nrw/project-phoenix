package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/uptrace/bun"
)

type folderRow struct {
	bun.BaseModel `bun:"table:documents.folders,alias:folder"`
	ID            int64     `bun:"id,pk,autoincrement"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	Name          string    `bun:"name,notnull"`
	Visibility    string    `bun:"visibility,notnull"`
	CreatedBy     int64     `bun:"created_by,notnull"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r folderRow) folder() domain.Folder {
	return domain.Folder{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Visibility: r.Visibility,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type folderListRow struct {
	folderRow
	FileCount int64 `bun:"file_count,scanonly"`
}

type folderRoleRow struct {
	bun.BaseModel `bun:"table:documents.folder_roles,alias:share"`
	TenantID      int64 `bun:"tenant_id,notnull"`
	FolderID      int64 `bun:"folder_id,notnull"`
	RoleID        int64 `bun:"role_id,notnull"`
}

type folderAccountRow struct {
	bun.BaseModel `bun:"table:documents.folder_accounts,alias:share"`
	TenantID      int64 `bun:"tenant_id,notnull"`
	FolderID      int64 `bun:"folder_id,notnull"`
	AccountID     int64 `bun:"account_id,notnull"`
}

// FolderStore implements ports.FolderStore.
type FolderStore struct{ database Database }

// NewFolderStore wires the folder store.
func NewFolderStore(database Database) *FolderStore {
	if database == nil {
		panic("file storage postgres: database runtime is required")
	}
	return &FolderStore{database: database}
}

func (s *FolderStore) Create(ctx context.Context, folder domain.Folder) (domain.Folder, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.Folder{}, domain.OperationStats{}, err
	}
	row := folderRow{TenantID: tenantID, Name: folder.Name, Visibility: folder.Visibility, CreatedBy: folder.CreatedBy}
	started := time.Now()
	err = db.NewInsert().Model(&row).ModelTableExpr(`documents.folders`).Returning("*").Scan(ctx)
	stats := measure(started, 1)
	if err != nil {
		return domain.Folder{}, stats, fmt.Errorf("file storage postgres: create folder: %w", classifyWriteError(err))
	}
	return row.folder(), stats, nil
}

func (s *FolderStore) Update(ctx context.Context, folder domain.Folder) (domain.Folder, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.Folder{}, domain.OperationStats{}, err
	}
	row := folderRow{}
	started := time.Now()
	err = db.NewUpdate().Model((*folderRow)(nil)).
		ModelTableExpr(`documents.folders AS "folder"`).
		Set(`name = ?`, folder.Name).
		Set(`visibility = ?`, folder.Visibility).
		Where(`"folder".id = ?`, folder.ID).
		Where(`"folder".tenant_id = ?`, tenantID).
		Returning("*").
		Scan(ctx, &row)
	stats := measure(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Folder{}, stats, domain.ErrNotFound
	}
	if err != nil {
		return domain.Folder{}, stats, fmt.Errorf("file storage postgres: update folder: %w", classifyWriteError(err))
	}
	return row.folder(), stats, nil
}

func (s *FolderStore) Delete(ctx context.Context, folderID int64) (domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	started := time.Now()
	result, err := db.NewDelete().Model((*folderRow)(nil)).
		ModelTableExpr(`documents.folders AS "folder"`).
		Where(`"folder".id = ?`, folderID).
		Where(`"folder".tenant_id = ?`, tenantID).
		Exec(ctx)
	stats := measure(started, 0)
	if err != nil {
		return stats, fmt.Errorf("file storage postgres: delete folder: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("file storage postgres: count deleted folders: %w", err)
	}
	if affected == 0 {
		return stats, domain.ErrNotFound
	}
	stats.Rows = affected
	return stats, nil
}

func (s *FolderStore) FindByID(ctx context.Context, folderID int64) (domain.Folder, bool, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.Folder{}, false, domain.OperationStats{}, err
	}
	row := folderRow{}
	started := time.Now()
	err = db.NewSelect().Model(&row).
		ModelTableExpr(`documents.folders AS "folder"`).
		Where(`"folder".id = ?`, folderID).
		Where(`"folder".tenant_id = ?`, tenantID).
		Scan(ctx)
	stats := measure(started, 1)
	if errors.Is(err, sql.ErrNoRows) {
		stats.Rows = 0
		return domain.Folder{}, false, stats, nil
	}
	if err != nil {
		return domain.Folder{}, false, stats, fmt.Errorf("file storage postgres: find folder: %w", err)
	}
	return row.folder(), true, stats, nil
}

// visibleFolders narrows a folder query to what a non-manager may see: every
// all_staff folder, plus the selected folders shared with the viewer's account
// or with one of the roles the viewer holds at this school.
func visibleFolders(db bun.IDB, query *bun.SelectQuery, tenantID int64, viewer domain.Viewer) *bun.SelectQuery {
	if viewer.Manager {
		return query
	}
	accountShare := db.NewSelect().Model((*folderAccountRow)(nil)).
		ModelTableExpr(`documents.folder_accounts AS "share"`).
		ColumnExpr(`"share".folder_id`).
		Where(`"share".tenant_id = ?`, tenantID).
		Where(`"share".account_id = ?`, viewer.AccountID)
	return query.WhereGroup(" AND ", func(query *bun.SelectQuery) *bun.SelectQuery {
		query = query.Where(`"folder".visibility = ?`, domain.VisibilityAllStaff)
		return query.WhereGroup(" OR ", func(query *bun.SelectQuery) *bun.SelectQuery {
			query = query.Where(`"folder".visibility = ?`, domain.VisibilitySelected)
			return query.WhereGroup(" AND ", func(query *bun.SelectQuery) *bun.SelectQuery {
				query = query.Where(`"folder".id IN (?)`, accountShare)
				if len(viewer.RoleIDs) == 0 {
					return query
				}
				roleShare := db.NewSelect().Model((*folderRoleRow)(nil)).
					ModelTableExpr(`documents.folder_roles AS "share"`).
					ColumnExpr(`"share".folder_id`).
					Where(`"share".tenant_id = ?`, tenantID).
					Where(`"share".role_id IN (?)`, bun.List(viewer.RoleIDs))
				return query.WhereOr(`"folder".id IN (?)`, roleShare)
			})
		})
	})
}

func (s *FolderStore) ListVisible(ctx context.Context, viewer domain.Viewer) ([]domain.FolderListItem, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []folderListRow
	fileCount := db.NewSelect().Model((*fileRow)(nil)).
		ModelTableExpr(`documents.files AS "document"`).
		ColumnExpr("COUNT(*)").
		Where(`"document".tenant_id = "folder".tenant_id`).
		Where(`"document".folder_id = "folder".id`).
		Where(`"document".deleted_at IS NULL`)
	query := db.NewSelect().Model(&rows).
		ModelTableExpr(`documents.folders AS "folder"`).
		ColumnExpr(`"folder".*`).
		ColumnExpr(`(?) AS file_count`, fileCount).
		Where(`"folder".tenant_id = ?`, tenantID).
		OrderExpr(`lower("folder".name) ASC, "folder".id ASC`)
	query = visibleFolders(db, query, tenantID, viewer)
	started := time.Now()
	err = query.Scan(ctx)
	stats := measure(started, int64(len(rows)))
	if err != nil {
		return nil, stats, fmt.Errorf("file storage postgres: list visible folders: %w", err)
	}
	result := make([]domain.FolderListItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.FolderListItem{Folder: row.folder(), FileCount: row.FileCount})
	}
	return result, stats, nil
}

func (s *FolderStore) IsVisible(ctx context.Context, folderID int64, viewer domain.Viewer) (bool, domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	query := db.NewSelect().Model((*folderRow)(nil)).
		ModelTableExpr(`documents.folders AS "folder"`).
		Where(`"folder".id = ?`, folderID).
		Where(`"folder".tenant_id = ?`, tenantID)
	query = visibleFolders(db, query, tenantID, viewer)
	started := time.Now()
	exists, err := query.Exists(ctx)
	stats := measure(started, 0)
	if err != nil {
		return false, stats, fmt.Errorf("file storage postgres: check folder visibility: %w", err)
	}
	if exists {
		stats.Rows = 1
	}
	return exists, stats, nil
}

// ReplaceAudience rewrites both share lists. The delete-then-insert runs
// inside the caller's transaction, so a reader never observes a half-written
// list.
func (s *FolderStore) ReplaceAudience(ctx context.Context, folderID int64, audience domain.Audience) (domain.OperationStats, error) {
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()

	stats.Queries++
	if _, err := db.NewDelete().Model((*folderRoleRow)(nil)).
		ModelTableExpr(`documents.folder_roles AS "share"`).
		Where(`"share".folder_id = ?`, folderID).
		Where(`"share".tenant_id = ?`, tenantID).
		Exec(ctx); err != nil {
		return stats, fmt.Errorf("file storage postgres: clear folder roles: %w", err)
	}
	stats.Queries++
	if _, err := db.NewDelete().Model((*folderAccountRow)(nil)).
		ModelTableExpr(`documents.folder_accounts AS "share"`).
		Where(`"share".folder_id = ?`, folderID).
		Where(`"share".tenant_id = ?`, tenantID).
		Exec(ctx); err != nil {
		return stats, fmt.Errorf("file storage postgres: clear folder accounts: %w", err)
	}
	if len(audience.RoleIDs) > 0 {
		rows := make([]folderRoleRow, 0, len(audience.RoleIDs))
		for _, id := range audience.RoleIDs {
			rows = append(rows, folderRoleRow{TenantID: tenantID, FolderID: folderID, RoleID: id})
		}
		stats.Queries++
		if _, err := db.NewInsert().Model(&rows).
			ModelTableExpr(`documents.folder_roles`).
			On("CONFLICT DO NOTHING").
			Exec(ctx); err != nil {
			return stats, fmt.Errorf("file storage postgres: insert folder roles: %w", err)
		}
		stats.Rows += int64(len(rows))
	}
	if len(audience.AccountIDs) > 0 {
		rows := make([]folderAccountRow, 0, len(audience.AccountIDs))
		for _, id := range audience.AccountIDs {
			rows = append(rows, folderAccountRow{TenantID: tenantID, FolderID: folderID, AccountID: id})
		}
		stats.Queries++
		if _, err := db.NewInsert().Model(&rows).
			ModelTableExpr(`documents.folder_accounts`).
			On("CONFLICT DO NOTHING").
			Exec(ctx); err != nil {
			return stats, fmt.Errorf("file storage postgres: insert folder accounts: %w", err)
		}
		stats.Rows += int64(len(rows))
	}
	return stats, nil
}

func (s *FolderStore) GetAudience(ctx context.Context, folderIDs []int64) (map[int64]domain.Audience, domain.OperationStats, error) {
	result := make(map[int64]domain.Audience, len(folderIDs))
	if len(folderIDs) == 0 {
		return result, domain.OperationStats{}, nil
	}
	db, tenantID, err := requireTenant(s.database, ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 2}
	started := time.Now()
	defer func() { stats.StatementDuration = time.Since(started) }()

	var roles []folderRoleRow
	if err := db.NewSelect().Model(&roles).
		ModelTableExpr(`documents.folder_roles AS "share"`).
		Where(`"share".folder_id IN (?)`, bun.List(folderIDs)).
		Where(`"share".tenant_id = ?`, tenantID).
		OrderExpr(`"share".role_id ASC`).
		Scan(ctx); err != nil {
		return nil, stats, fmt.Errorf("file storage postgres: list folder roles: %w", err)
	}
	var accounts []folderAccountRow
	if err := db.NewSelect().Model(&accounts).
		ModelTableExpr(`documents.folder_accounts AS "share"`).
		Where(`"share".folder_id IN (?)`, bun.List(folderIDs)).
		Where(`"share".tenant_id = ?`, tenantID).
		OrderExpr(`"share".account_id ASC`).
		Scan(ctx); err != nil {
		return nil, stats, fmt.Errorf("file storage postgres: list folder accounts: %w", err)
	}
	stats.Rows = int64(len(roles) + len(accounts))
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
	return result, stats, nil
}
