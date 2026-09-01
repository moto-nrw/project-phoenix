// Package filestore is the business boundary of the school file storage
// (#2596): folder visibility, who may upload and delete, the storage quota,
// and the audit trail. The handler owns the bytes (multipart parsing, magic
// number validation, storage writes); this package owns authority and
// metadata.
//
// Authority, in one paragraph. Admins and files:manage holders ("managers")
// see every folder, create and change folders, and upload or delete any file.
// Everybody else sees the folders whose visibility admits them, and — only
// while files.staff_upload_enabled is on — may upload into those folders and
// delete their own uploads. Nothing else exists: no per-file rights, no
// delegated folder ownership.
package filestore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	repositoryBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	// UploadDeadline bounds every step an upload request may still perform
	// once its cleanup intent exists (object write + metadata transaction).
	// The handler enforces it; see the student document service for why.
	UploadDeadline = 2 * time.Minute
	// cleanupDelay keeps a queued intent ineligible until well past
	// UploadDeadline. It MUST stay larger than UploadDeadline.
	cleanupDelay = 5 * time.Minute

	bytesPerMB = 1024 * 1024
)

// ErrInvalid marks a semantically invalid payload — HTTP 400.
var ErrInvalid = errors.New("invalid file storage request")

// ErrForbidden marks an action the caller may not perform — HTTP 403.
var ErrForbidden = errors.New("file storage action not permitted")

// ErrFolderNameTaken marks a duplicate folder name — HTTP 409.
var ErrFolderNameTaken = errors.New("folder name already exists")

// ErrQuotaExceeded marks an upload the school's storage quota does not
// admit — HTTP 409.
var ErrQuotaExceeded = errors.New("file storage quota exceeded")

// Actor identifies the acting account for authority checks and audit rows.
type Actor struct {
	AccountID   int64
	Name        string
	Permissions []string
}

// IsManager reports whether the actor may manage folders and every file.
func (a Actor) IsManager() bool {
	return authorize.HasPermission(permissions.FilesManage, a.Permissions)
}

func (a Actor) viewer() filestore.Viewer {
	return filestore.Viewer{AccountID: a.AccountID, Manager: a.IsManager()}
}

// FolderInput carries a folder create or update.
type FolderInput struct {
	Name       string
	Visibility string
	RoleIDs    []int64
	AccountIDs []int64
}

// FolderView is a folder as the list endpoint returns it: the row, its live
// file count and (for managers) its share list.
type FolderView struct {
	filestore.FolderListItem
	Audience filestore.Audience
}

// Overview is the response of the folder list.
type Overview struct {
	Folders []*FolderView
	// CanManage is true for admins and files:manage holders.
	CanManage bool
	// CanUpload is true when the actor may add files to the folders they see.
	CanUpload bool
	// StaffUploadEnabled mirrors files.staff_upload_enabled so the UI can
	// explain who uploads when the actor cannot.
	StaffUploadEnabled bool
	UsedBytes          int64
	MaxBytes           int64
}

// CreateFileInput carries the metadata of an already-stored upload.
type CreateFileInput struct {
	FolderID        int64
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
}

// AudienceOptions are the roles and accounts a folder can be shared with.
type AudienceOptions struct {
	Roles    []*filestore.AudienceRole
	Accounts []*filestore.AudienceAccount
}

// Service is the business boundary of the file storage.
type Service interface {
	ListFolders(ctx context.Context, actor Actor) (*Overview, error)
	CreateFolder(ctx context.Context, input FolderInput, actor Actor) (*FolderView, error)
	UpdateFolder(ctx context.Context, folderID int64, input FolderInput, actor Actor) (*FolderView, error)
	// DeleteFolder queues cleanup for every file still on disk and removes the
	// folder; the file rows cascade. Bytes are removed by the scheduler.
	DeleteFolder(ctx context.Context, folderID int64, actor Actor) error
	ListAudienceOptions(ctx context.Context, actor Actor) (*AudienceOptions, error)

	// ListFiles returns the folder and its live files, newest first.
	ListFiles(ctx context.Context, folderID int64, actor Actor) (*filestore.Folder, []*filestore.File, error)
	// CanDeleteFile answers, for an already-listed file, whether the actor may
	// delete it (manager, or own upload while staff uploads are enabled).
	CanDeleteFile(ctx context.Context, file *filestore.File, actor Actor) (bool, error)
	// AuthorizeUpload answers "may this caller add a file to this folder"
	// WITHOUT writing anything, so an unauthorized request costs nothing but
	// the request itself. CreateFile repeats the check inside its transaction.
	AuthorizeUpload(ctx context.Context, folderID int64, actor Actor) error
	CreateFile(ctx context.Context, input CreateFileInput, actor Actor) (*filestore.File, error)
	ResolveDownload(ctx context.Context, folderID, fileID int64, actor Actor) (*filestore.File, error)
	DeleteFile(ctx context.Context, folderID, fileID int64, actor Actor) (*filestore.File, error)
	ResolveCleanup(ctx context.Context, folderID, fileID int64, actor Actor) (*filestore.File, error)
	ListDeletedFilesPendingCleanup(ctx context.Context, folderID int64, actor Actor) ([]*filestore.File, error)
	ListDeletedFilesPendingCleanups(ctx context.Context) ([]*filestore.File, error)
	QueueFileCleanup(ctx context.Context, folderID int64, storedName string) error
	ListQueuedFileCleanups(ctx context.Context) ([]*filestore.FileCleanup, error)

	// The remaining methods implement the coordinator's Store interface.
	MarkFileDeleted(ctx context.Context, fileID int64) error
	MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error
	MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error
	ActivateQueuedCleanup(ctx context.Context, storedName string) error

	// Anhänge an Elternmitteilungen (#2890) teilen sich mit der Dateiablage
	// alles außer der Frage, wer sie sehen darf.
	AttachmentService
}

type service struct {
	db          *bun.DB
	folders     filestore.FolderRepository
	files       filestore.FileRepository
	attachments filestore.AnnouncementAttachmentRepository
	events      auditModels.FileEventRepository
	settings    configSvc.SettingsService
	// announcements and audience are the announcement-side ports (#2890). They
	// stay nil in setups that do not serve attachments; every path that needs
	// them refuses loudly rather than deciding on its own.
	announcements AnnouncementGuard
	audience      AnnouncementAudience
	logger        *slog.Logger
}

// NewService wires the file storage service.
func NewService(
	db *bun.DB,
	folders filestore.FolderRepository,
	files filestore.FileRepository,
	attachments filestore.AnnouncementAttachmentRepository,
	events auditModels.FileEventRepository,
	settings configSvc.SettingsService,
	logger *slog.Logger,
) Service {
	return &service{
		db:          db,
		folders:     folders,
		files:       files,
		attachments: attachments,
		events:      events,
		settings:    settings,
		logger:      logger,
	}
}

// SetAnnouncementPorts injects the announcement-side questions the file
// storage cannot answer itself (#2890). It is a setter rather than a
// constructor argument because the announcement services are built after this
// one in the composition root, and one of them needs a purger pointing back
// here — passing both ways round at construction time is not possible.
func (s *service) SetAnnouncementPorts(guard AnnouncementGuard, audience AnnouncementAudience) {
	s.announcements = guard
	s.audience = audience
}

// AnnouncementPortSetter is what the composition root needs to complete the
// wiring after both sides exist.
type AnnouncementPortSetter interface {
	SetAnnouncementPorts(guard AnnouncementGuard, audience AnnouncementAudience)
}

func (s *service) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// --- settings ----------------------------------------------------------------

func (s *service) staffUploadEnabled(ctx context.Context) (bool, error) {
	if s.settings == nil {
		return false, nil
	}
	enabled, err := s.settings.ResolveBool(ctx, configModel.KeyFilesStaffUploadEnabled)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModel.KeyFilesStaffUploadEnabled, err)
	}
	return enabled, nil
}

func (s *service) maxBytes(ctx context.Context) (int64, error) {
	if s.settings == nil {
		return 0, nil
	}
	mb, err := s.settings.ResolveInt(ctx, configModel.KeyFilesMaxStorageMB)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModel.KeyFilesMaxStorageMB, err)
	}
	return int64(mb) * bytesPerMB, nil
}

// --- authority ---------------------------------------------------------------

func requireManager(actor Actor) error {
	if !actor.IsManager() {
		return fmt.Errorf("%w: files:manage required", ErrForbidden)
	}
	return nil
}

// requireVisible loads the folder and refuses when the actor may not see it.
// A folder the actor cannot see is reported as missing, not forbidden: the
// share list must not be probeable by ID.
func (s *service) requireVisible(ctx context.Context, folderID int64, actor Actor) (*filestore.Folder, error) {
	folder, err := s.folders.FindByID(ctx, folderID)
	if err != nil {
		return nil, err
	}
	visible, err := s.folders.IsVisible(ctx, folderID, actor.viewer())
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, &modelBase.DatabaseError{Op: "find folder", Err: modelBase.ErrNotFound}
	}
	return folder, nil
}

// canUpload answers whether the actor may add files to a folder they can
// already see.
func (s *service) canUpload(ctx context.Context, actor Actor) (bool, error) {
	if actor.IsManager() {
		return true, nil
	}
	return s.staffUploadEnabled(ctx)
}

func (s *service) CanDeleteFile(ctx context.Context, file *filestore.File, actor Actor) (bool, error) {
	if actor.IsManager() {
		return true, nil
	}
	if file.UploadedBy != actor.AccountID {
		return false, nil
	}
	return s.staffUploadEnabled(ctx)
}

// --- folders -----------------------------------------------------------------

func (s *service) ListFolders(ctx context.Context, actor Actor) (*Overview, error) {
	items, err := s.folders.ListVisible(ctx, actor.viewer())
	if err != nil {
		return nil, err
	}
	overview := &Overview{
		Folders:   make([]*FolderView, 0, len(items)),
		CanManage: actor.IsManager(),
	}
	if overview.StaffUploadEnabled, err = s.staffUploadEnabled(ctx); err != nil {
		return nil, err
	}
	overview.CanUpload = overview.CanManage || overview.StaffUploadEnabled

	var audiences map[int64]filestore.Audience
	if overview.CanManage {
		ids := make([]int64, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		if audiences, err = s.folders.GetAudience(ctx, ids); err != nil {
			return nil, err
		}
		if overview.UsedBytes, err = s.files.TotalStoredBytes(ctx); err != nil {
			return nil, err
		}
		if overview.MaxBytes, err = s.maxBytes(ctx); err != nil {
			return nil, err
		}
	}
	for _, item := range items {
		overview.Folders = append(overview.Folders, &FolderView{
			FolderListItem: *item,
			Audience:       audiences[item.ID],
		})
	}
	return overview, nil
}

func normalizeFolderInput(input *FolderInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalid)
	}
	if len([]rune(input.Name)) > filestore.MaxFolderNameLength {
		return fmt.Errorf("%w: name is too long", ErrInvalid)
	}
	if !filestore.IsValidVisibility(input.Visibility) {
		return fmt.Errorf("%w: unknown visibility", ErrInvalid)
	}
	input.RoleIDs = dedupeIDs(input.RoleIDs)
	input.AccountIDs = dedupeIDs(input.AccountIDs)
	if input.Visibility != filestore.VisibilitySelected {
		input.RoleIDs, input.AccountIDs = nil, nil
	} else if len(input.RoleIDs) == 0 && len(input.AccountIDs) == 0 {
		return fmt.Errorf("%w: a selected folder needs at least one role or person", ErrInvalid)
	}
	return nil
}

func dedupeIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// validateAudience refuses roles and accounts that are not shareable at this
// school, so a share list can never point outside the tenant.
func (s *service) validateAudience(ctx context.Context, audience filestore.Audience) error {
	if len(audience.RoleIDs) > 0 {
		roles, err := s.folders.ListAudienceRoles(ctx)
		if err != nil {
			return err
		}
		allowed := make(map[int64]struct{}, len(roles))
		for _, role := range roles {
			allowed[role.ID] = struct{}{}
		}
		for _, id := range audience.RoleIDs {
			if _, ok := allowed[id]; !ok {
				return fmt.Errorf("%w: unknown role", ErrInvalid)
			}
		}
	}
	if len(audience.AccountIDs) > 0 {
		accounts, err := s.folders.ListAudienceAccounts(ctx)
		if err != nil {
			return err
		}
		allowed := make(map[int64]struct{}, len(accounts))
		for _, account := range accounts {
			allowed[account.AccountID] = struct{}{}
		}
		for _, id := range audience.AccountIDs {
			if _, ok := allowed[id]; !ok {
				return fmt.Errorf("%w: unknown person", ErrInvalid)
			}
		}
	}
	return nil
}

func (s *service) CreateFolder(ctx context.Context, input FolderInput, actor Actor) (*FolderView, error) {
	if err := requireManager(actor); err != nil {
		return nil, err
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("%w: actor account id is required", ErrInvalid)
	}
	if err := normalizeFolderInput(&input); err != nil {
		return nil, err
	}
	audience := filestore.Audience{RoleIDs: input.RoleIDs, AccountIDs: input.AccountIDs}

	folder := &filestore.Folder{Name: input.Name, Visibility: input.Visibility, CreatedBy: actor.AccountID}
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.validateAudience(ctx, audience); err != nil {
			return err
		}
		if err := s.folders.Create(ctx, folder); err != nil {
			return mapFolderWriteError(err)
		}
		if err := s.folders.ReplaceAudience(ctx, folder.ID, audience); err != nil {
			return err
		}
		return s.recordEvent(ctx, actor, auditModels.FileEventFolderCreated, &folder.ID, nil,
			folderEventDetail(folder, audience))
	})
	if err != nil {
		return nil, err
	}
	return &FolderView{FolderListItem: filestore.FolderListItem{Folder: *folder}, Audience: audience}, nil
}

func (s *service) UpdateFolder(ctx context.Context, folderID int64, input FolderInput, actor Actor) (*FolderView, error) {
	if err := requireManager(actor); err != nil {
		return nil, err
	}
	if err := normalizeFolderInput(&input); err != nil {
		return nil, err
	}
	audience := filestore.Audience{RoleIDs: input.RoleIDs, AccountIDs: input.AccountIDs}

	var folder *filestore.Folder
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		existing, err := s.folders.FindByID(ctx, folderID)
		if err != nil {
			return err
		}
		if err := s.validateAudience(ctx, audience); err != nil {
			return err
		}
		existing.Name = input.Name
		existing.Visibility = input.Visibility
		if err := s.folders.Update(ctx, existing); err != nil {
			return mapFolderWriteError(err)
		}
		if err := s.folders.ReplaceAudience(ctx, existing.ID, audience); err != nil {
			return err
		}
		folder = existing
		return s.recordEvent(ctx, actor, auditModels.FileEventFolderUpdated, &existing.ID, nil,
			folderEventDetail(existing, audience))
	})
	if err != nil {
		return nil, err
	}
	return &FolderView{FolderListItem: filestore.FolderListItem{Folder: *folder}, Audience: audience}, nil
}

func (s *service) DeleteFolder(ctx context.Context, folderID int64, actor Actor) error {
	if err := requireManager(actor); err != nil {
		return err
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		folder, err := s.folders.FindByID(ctx, folderID)
		if err != nil {
			return err
		}
		// Every object still on disk gets an immediately eligible intent
		// BEFORE the rows cascade away — the intents are all that survive.
		pending, err := s.files.ListPendingFileCleanupByOwnerID(ctx, folderID)
		if err != nil {
			return err
		}
		for _, file := range pending {
			if err := s.queueCleanup(ctx, folderID, file.FilenameStored, time.Now()); err != nil {
				return err
			}
		}
		if err := s.folders.Delete(ctx, folderID); err != nil {
			return err
		}
		s.getLogger().Info("folder deleted",
			"folder_id", folderID,
			"files_pending_cleanup", len(pending))
		return s.recordEvent(ctx, actor, auditModels.FileEventFolderDeleted, &folderID, nil,
			fmt.Sprintf("Ordner „%s“ gelöscht (%d Dateien)", folder.Name, len(pending)))
	})
}

func (s *service) ListAudienceOptions(ctx context.Context, actor Actor) (*AudienceOptions, error) {
	if err := requireManager(actor); err != nil {
		return nil, err
	}
	roles, err := s.folders.ListAudienceRoles(ctx)
	if err != nil {
		return nil, err
	}
	accounts, err := s.folders.ListAudienceAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return &AudienceOptions{Roles: roles, Accounts: accounts}, nil
}

// mapFolderWriteError turns the unique-name violation into the domain error.
func mapFolderWriteError(err error) error {
	if err != nil && strings.Contains(err.Error(), "uq_documents_folders_name") {
		return ErrFolderNameTaken
	}
	return err
}

// --- files -------------------------------------------------------------------

func (s *service) ListFiles(ctx context.Context, folderID int64, actor Actor) (*filestore.Folder, []*filestore.File, error) {
	folder, err := s.requireVisible(ctx, folderID, actor)
	if err != nil {
		return nil, nil, err
	}
	files, err := s.files.ListByOwnerID(ctx, folderID, []string{filestore.FileCategory})
	if err != nil {
		return nil, nil, err
	}
	return folder, files, nil
}

func (s *service) AuthorizeUpload(ctx context.Context, folderID int64, actor Actor) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.requireUpload(ctx, folderID, actor)
	})
}

func (s *service) requireUpload(ctx context.Context, folderID int64, actor Actor) error {
	if _, err := s.requireVisible(ctx, folderID, actor); err != nil {
		return err
	}
	allowed, err := s.canUpload(ctx, actor)
	if err != nil {
		return err
	}
	if !allowed {
		return fmt.Errorf("%w: uploads are reserved for the leadership", ErrForbidden)
	}
	return nil
}

func (s *service) CreateFile(ctx context.Context, input CreateFileInput, actor Actor) (*filestore.File, error) {
	if s.events == nil {
		return nil, errors.New("file event repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("%w: actor account id is required", ErrInvalid)
	}
	input.FilenameDisplay = strings.TrimSpace(input.FilenameDisplay)
	if input.FilenameDisplay == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrInvalid)
	}

	file := &filestore.File{FolderID: input.FolderID}
	file.Category = filestore.FileCategory
	file.FilenameDisplay = input.FilenameDisplay
	file.FilenameStored = input.FilenameStored
	file.SizeBytes = input.SizeBytes
	file.ContentType = input.ContentType
	file.UploadedBy = actor.AccountID
	if err := file.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err.Error())
	}

	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.requireUpload(ctx, input.FolderID, actor); err != nil {
			return err
		}
		// The quota is a tenant-wide aggregate rather than a row we can lock.
		// Serialize its check and the metadata insert so concurrent uploads
		// cannot each admit themselves against the same previous total.
		if err := repositoryBase.AcquireXactLock(ctx, s.db, fmt.Sprintf("filestore-quota:%d", tenantID)); err != nil {
			return fmt.Errorf("lock file storage quota: %w", err)
		}
		if err := s.requireQuota(ctx, input.SizeBytes); err != nil {
			return err
		}
		if err := s.files.Create(ctx, file); err != nil {
			return err
		}
		if err := s.files.MarkQueuedFileCleanupCompleteByFilename(ctx, input.FilenameStored); err != nil {
			return fmt.Errorf("complete file upload cleanup intent: %w", err)
		}
		return s.recordEvent(ctx, actor, auditModels.FileEventFileUploaded, &input.FolderID, &file.ID,
			fmt.Sprintf("Datei „%s“ hochgeladen (%d Bytes)", file.FilenameDisplay, file.SizeBytes))
	})
	if err != nil {
		return nil, err
	}
	return file, nil
}

// requireQuota refuses an upload that would push the school past its storage
// quota. Soft-deleted files still count until the sweep removed their bytes,
// which is what actually occupies the disk.
func (s *service) requireQuota(ctx context.Context, sizeBytes int64) error {
	maxBytes, err := s.maxBytes(ctx)
	if err != nil {
		return err
	}
	if maxBytes <= 0 {
		return nil
	}
	used, err := s.files.TotalStoredBytes(ctx)
	if err != nil {
		return err
	}
	if used+sizeBytes > maxBytes {
		return ErrQuotaExceeded
	}
	return nil
}

func (s *service) ResolveDownload(ctx context.Context, folderID, fileID int64, actor Actor) (*filestore.File, error) {
	var file *filestore.File
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.requireVisible(ctx, folderID, actor); err != nil {
			return err
		}
		found, err := s.files.FindForOwner(ctx, folderID, fileID)
		if err != nil {
			return err
		}
		file = found
		return nil
	})
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *service) DeleteFile(ctx context.Context, folderID, fileID int64, actor Actor) (*filestore.File, error) {
	if s.events == nil {
		return nil, errors.New("file event repository is not wired; refusing unaudited change")
	}
	if actor.AccountID <= 0 {
		return nil, fmt.Errorf("%w: actor account id is required", ErrInvalid)
	}
	var deleted *filestore.File
	tenantID := tenant.FromContext(ctx)
	err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if _, err := s.requireVisible(ctx, folderID, actor); err != nil {
			return err
		}
		file, err := s.files.FindForOwner(ctx, folderID, fileID)
		if err != nil {
			return err
		}
		allowed, err := s.CanDeleteFile(ctx, file, actor)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("%w: only own uploads can be deleted", ErrForbidden)
		}
		if err := s.files.SoftDelete(ctx, file, actor.AccountID); err != nil {
			return err
		}
		deleted = file
		return s.recordEvent(ctx, actor, auditModels.FileEventFileDeleted, &folderID, &file.ID,
			fmt.Sprintf("Datei „%s“ gelöscht", file.FilenameDisplay))
	})
	if err != nil {
		return nil, err
	}
	return deleted, nil
}

func (s *service) ResolveCleanup(ctx context.Context, folderID, fileID int64, actor Actor) (*filestore.File, error) {
	if _, err := s.requireVisible(ctx, folderID, actor); err != nil {
		return nil, err
	}
	file, err := s.files.FindForOwnerIncludingDeleted(ctx, folderID, fileID)
	if err != nil {
		return nil, err
	}
	allowed, err := s.CanDeleteFile(ctx, file, actor)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, fmt.Errorf("%w: only own uploads can be deleted", ErrForbidden)
	}
	return file, nil
}

func (s *service) ListDeletedFilesPendingCleanup(ctx context.Context, folderID int64, actor Actor) ([]*filestore.File, error) {
	if _, err := s.requireVisible(ctx, folderID, actor); err != nil {
		return nil, err
	}
	return s.files.ListDeletedPendingFileCleanupByOwnerID(ctx, folderID, []string{filestore.FileCategory})
}

func (s *service) ListDeletedFilesPendingCleanups(ctx context.Context) ([]*filestore.File, error) {
	return s.files.ListDeletedPendingFileCleanups(ctx)
}

func (s *service) QueueFileCleanup(ctx context.Context, folderID int64, storedName string) error {
	if folderID <= 0 || strings.TrimSpace(storedName) == "" {
		return fmt.Errorf("%w: cleanup file details are required", ErrInvalid)
	}
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.queueCleanup(ctx, folderID, storedName, time.Now().Add(cleanupDelay))
	})
}

func (s *service) queueCleanup(ctx context.Context, folderID int64, storedName string, retryAfter time.Time) error {
	cleanup := &filestore.FileCleanup{}
	cleanup.OwnerID = folderID
	cleanup.FilenameStored = storedName
	cleanup.RetryAfter = retryAfter
	return s.files.QueueFileCleanup(ctx, cleanup)
}

func (s *service) ListQueuedFileCleanups(ctx context.Context) ([]*filestore.FileCleanup, error) {
	return s.files.ListQueuedFileCleanups(ctx)
}

func (s *service) MarkFileDeleted(ctx context.Context, fileID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.files.MarkFileDeleted(ctx, fileID)
	})
}

func (s *service) MarkQueuedCleanupComplete(ctx context.Context, cleanupID int64) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.files.MarkQueuedFileCleanupComplete(ctx, cleanupID)
	})
}

func (s *service) MarkQueuedCleanupCompleteByFilename(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.files.MarkQueuedFileCleanupCompleteByFilename(ctx, storedName)
	})
}

func (s *service) ActivateQueuedCleanup(ctx context.Context, storedName string) error {
	tenantID := tenant.FromContext(ctx)
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		return s.files.ActivateQueuedFileCleanupByFilename(ctx, storedName)
	})
}

// --- audit -------------------------------------------------------------------

func (s *service) recordEvent(ctx context.Context, actor Actor, action string, folderID, fileID *int64, detail string) error {
	if s.events == nil {
		return errors.New("file event repository is not wired; refusing unaudited change")
	}
	name := strings.TrimSpace(actor.Name)
	if name == "" {
		name = "Unbekannt"
	}
	event := &auditModels.FileEvent{
		FolderID:  folderID,
		FileID:    fileID,
		Action:    action,
		ActorName: name,
		Detail:    detail,
	}
	if actor.AccountID > 0 {
		id := actor.AccountID
		event.ActorAccountID = &id
	}
	if err := s.events.Create(ctx, event); err != nil {
		return fmt.Errorf("write file event: %w", err)
	}
	return nil
}

func folderEventDetail(folder *filestore.Folder, audience filestore.Audience) string {
	switch folder.Visibility {
	case filestore.VisibilitySelected:
		return fmt.Sprintf("Ordner „%s“: Sichtbar für %d Rollen und %d Personen", folder.Name, len(audience.RoleIDs), len(audience.AccountIDs))
	case filestore.VisibilityAdmins:
		return fmt.Sprintf("Ordner „%s“: Nur Leitung", folder.Name)
	default:
		return fmt.Sprintf("Ordner „%s“: Alle Mitarbeitenden", folder.Name)
	}
}
