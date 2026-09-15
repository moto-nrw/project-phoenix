// Package compose wires the File Storage module over the shared tenant
// runtime, the Bun database, the object-store backend and the owners it asks
// for facts: Identity & Access for memberships and roles, People Directory
// for the names in the audience picker, the Settings Platform for the two
// file settings, the Audit Platform for the trail, and Communication for the
// announcement side of attachments.
//
// documents.file_cleanup has no policy owner yet: the retained generic
// document repository reaches it only dynamically, so the ratchet records no
// finding to adopt it from (ADR 0015). Its intent operations therefore stay
// bound to that repository here, as a compatibility binding, until the table
// can be adopted; every other table is served by the owner's own adapter.
package compose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	documentsRepo "github.com/moto-nrw/project-phoenix/database/repositories/documents"
	"github.com/moto-nrw/project-phoenix/internal/storage"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	documentsModel "github.com/moto-nrw/project-phoenix/models/documents"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Observation is one observed operation of the module.
type Observation = ports.Observation

// ObjectBackend is the object store the module writes bytes to, keyed
// {kind}/{tenant}/{stored name} below the uploads root the root supplies.
type ObjectBackend = storage.Backend

const (
	// fileObjectKind is the storage key prefix of the school file storage.
	fileObjectKind = "files"
	// attachmentObjectKind is the prefix of the attachments. It is deliberately
	// distinct: the two have separate tables, intents and audiences, and a
	// shared prefix would let one sweep reach the other's objects.
	attachmentObjectKind = "announcement-attachments"

	bytesPerMB = 1024 * 1024
)

// Identity is the Identity & Access surface the module consumes.
type Identity interface {
	identityaccess.SchoolMembershipQuery
	identityaccess.SchoolRoleQuery
}

// People is the People Directory surface the module consumes.
type People interface {
	ListPersonsByAccount(context.Context, []int64) ([]peopledirectory.Person, error)
}

// Settings is the Settings Platform surface the module consumes.
type Settings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// Announcements is the Communication surface for the staff side of
// attachments; see ports.Announcements.
type Announcements = ports.Announcements

// GuardianAudience is the parents-portal surface for the guardian side of
// attachments; see ports.GuardianAudience.
type GuardianAudience = ports.GuardianAudience

// Dependencies are the bindings composition supplies. Announcements and
// GuardianAudience are optional: a setup without them serves folders and
// files and refuses every attachment path loudly.
type Dependencies struct {
	DB               *bun.DB
	Objects          ObjectBackend
	Identity         Identity
	People           People
	Settings         Settings
	Events           auditModels.FileEventRepository
	Announcements    Announcements
	GuardianAudience GuardianAudience
	Observe          func(Observation)
	Logger           *slog.Logger
	// Now is the clock; nil means time.Now.
	Now func() time.Time
}

// New composes the File Storage module. Every operation resolves the tenant
// from the context; the stores apply it as a defense-in-depth predicate.
func New(dependencies Dependencies) (*filestorage.Module, error) {
	if dependencies.DB == nil || dependencies.Objects == nil || dependencies.Identity == nil ||
		dependencies.People == nil || dependencies.Settings == nil || dependencies.Events == nil ||
		dependencies.Observe == nil || dependencies.Logger == nil {
		return nil, errors.New("file storage compose: all dependencies are required")
	}
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	runtime := databaseRuntime(dependencies.DB)
	service := application.New(application.Dependencies{
		Folders:            postgres.NewFolderStore(runtime),
		Files:              postgres.NewFileStore(runtime),
		FileCleanups:       fileCleanups{repo: newLegacyFileCleanupRepository(dependencies.DB)},
		Attachments:        postgres.NewAttachmentStore(runtime),
		AttachmentCleanups: postgres.NewAttachmentCleanupStore(runtime),
		FileObjects:        objectStore{kind: fileObjectKind, backend: dependencies.Objects},
		AttachmentObjects:  objectStore{kind: attachmentObjectKind, backend: dependencies.Objects},
		Identity:           identity{module: dependencies.Identity},
		People:             people{query: dependencies.People},
		Settings:           settings{service: dependencies.Settings},
		Events:             events{repo: dependencies.Events},
		Announcements:      dependencies.Announcements,
		GuardianAudience:   dependencies.GuardianAudience,
		Tx:                 transaction{},
		StoredNames:        newStoredName,
		Observe: func(observation Observation) {
			observation.Err = mapError(observation.Err)
			dependencies.Observe(observation)
		},
		Logger: dependencies.Logger,
		Now:    now,
	})
	return filestorage.NewModule(engine{service: service}), nil
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("file storage postgres: tenant is required: %w", err)
		}
		transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			return db, tenantID.Int64(), nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID.Int64(), nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID.Int64(), nil
			}
			return db, tenantID.Int64(), nil
		default:
			return nil, 0, fmt.Errorf("file storage postgres: unsupported transaction %T", transaction)
		}
	}
}

// --- tenant runtime ------------------------------------------------------------

type transaction struct{}

func (transaction) TenantID(ctx context.Context) int64 { return tenant.FromContext(ctx) }

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunInTenant(ctx context.Context, tenantID int64, callback func(context.Context) error) error {
	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		return err
	}
	return tenant.WithinTenant(ctx, id, callback)
}

func (transaction) AcquireLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

func (transaction) AfterCommit(ctx context.Context, fn func()) {
	tenant.RegisterAfterCommit(ctx, fn)
}

func (transaction) Detach(ctx context.Context) context.Context {
	return context.WithoutCancel(tenant.ContextWithoutAfterCommitHooks(tenant.ContextWithoutTransaction(ctx)))
}

// --- object store --------------------------------------------------------------

type objectStore struct {
	kind    string
	backend storage.Backend
}

func (s objectStore) Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error) {
	key, err := storage.TenantKey(s.kind, tenantID, storedName)
	if err != nil {
		return 0, err
	}
	return s.backend.Save(ctx, key, source, storage.SaveOptions{Private: true})
}

func (s objectStore) Open(ctx context.Context, tenantID int64, storedName string) (ports.Object, error) {
	key, err := storage.TenantKey(s.kind, tenantID, storedName)
	if err != nil {
		return nil, err
	}
	return s.backend.Open(ctx, key)
}

func (s objectStore) Remove(ctx context.Context, tenantID int64, storedName string) error {
	key, err := storage.TenantKey(s.kind, tenantID, storedName)
	if err != nil {
		return err
	}
	return s.backend.Remove(ctx, key)
}

// newStoredName generates the storage name for a validated content type.
func newStoredName(extension string) (string, error) {
	if extension == "" {
		return "", errors.New("unsupported document content type")
	}
	id, err := uuid.NewV4()
	if err != nil {
		return "", errors.New("failed to generate document filename")
	}
	return id.String() + extension, nil
}

// --- foreign facts -------------------------------------------------------------

type identity struct{ module Identity }

func (i identity) HasActiveMembership(ctx context.Context, accountID int64) (bool, error) {
	return i.module.HasActiveSchoolMembership(ctx, accountID)
}

func (i identity) ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return i.module.ListAccountRoleIDs(ctx, accountID)
}

// ListShareableRoles drops the guardian tier: parents never reach the tenant
// portal, so a folder shared with them would be shared with nobody.
func (i identity) ListShareableRoles(ctx context.Context) ([]domain.AudienceRole, error) {
	roles, err := i.module.ListSchoolRoles(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AudienceRole, 0, len(roles))
	for _, role := range roles {
		if role == nil || role.Name == "guardian" || (role.BaseRole != nil && *role.BaseRole == "guardian") {
			continue
		}
		result = append(result, domain.AudienceRole{ID: role.ID, Name: role.Name})
	}
	return result, nil
}

func (i identity) ListActiveAccountIDs(ctx context.Context) ([]int64, error) {
	return i.module.ListActiveSchoolAccountIDs(ctx)
}

type people struct{ query People }

func (p people) PersonNames(ctx context.Context, accountIDs []int64) (map[int64]ports.PersonName, error) {
	persons, err := p.query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	newest := make(map[int64]peopledirectory.Person, len(persons))
	for _, person := range persons {
		if person.AccountID == nil {
			continue
		}
		current, found := newest[*person.AccountID]
		if !found || person.UpdatedAt.After(current.UpdatedAt) {
			newest[*person.AccountID] = person
		}
	}
	result := make(map[int64]ports.PersonName, len(newest))
	for accountID, person := range newest {
		result[accountID] = ports.PersonName{FirstName: person.FirstName, LastName: person.LastName}
	}
	return result, nil
}

type settings struct{ service Settings }

func (s settings) StaffUploadEnabled(ctx context.Context) (bool, error) {
	enabled, err := s.service.ResolveBool(ctx, configModel.KeyFilesStaffUploadEnabled)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", configModel.KeyFilesStaffUploadEnabled, err)
	}
	return enabled, nil
}

func (s settings) MaxStorageBytes(ctx context.Context) (int64, error) {
	mb, err := s.service.ResolveInt(ctx, configModel.KeyFilesMaxStorageMB)
	if err != nil {
		return 0, fmt.Errorf("resolve %s: %w", configModel.KeyFilesMaxStorageMB, err)
	}
	return int64(mb) * bytesPerMB, nil
}

type events struct {
	repo auditModels.FileEventRepository
}

func (e events) Record(ctx context.Context, event ports.Event) error {
	name := strings.TrimSpace(event.Actor.Name)
	if name == "" {
		name = "Unbekannt"
	}
	row := &auditModels.FileEvent{
		FolderID:       event.FolderID,
		AnnouncementID: event.AnnouncementID,
		FileID:         event.FileID,
		Action:         event.Action,
		ActorName:      name,
		Detail:         event.Detail,
	}
	if event.Actor.AccountID > 0 {
		id := event.Actor.AccountID
		row.ActorAccountID = &id
	}
	return e.repo.Create(ctx, row)
}

// --- file cleanup intents over the retained generic repository -----------------

// legacyFileRow satisfies the generic repository's entity constraint; only the
// cleanup half of that repository is used here.
type legacyFileRow struct {
	documentsModel.File
	FolderID int64 `bun:"folder_id,notnull"`
}

func (r *legacyFileRow) GetOwnerID() int64 { return r.FolderID }

func (r *legacyFileRow) Validate() error { return documentsModel.ValidateFile(&r.File) }

type legacyFileCleanupRepository = documentsRepo.Repository[*legacyFileRow, *documentsModel.FileCleanup]

func newLegacyFileCleanupRepository(db *bun.DB) *legacyFileCleanupRepository {
	return documentsRepo.NewRepository[*legacyFileRow, *documentsModel.FileCleanup](db, documentsRepo.Config{
		Table:        "documents.files",
		Alias:        "file",
		OwnerColumn:  "folder_id",
		CleanupTable: "documents.file_cleanup",
		CleanupAlias: "file_cleanup",
	})
}

type fileCleanups struct{ repo *legacyFileCleanupRepository }

func (c fileCleanups) Queue(ctx context.Context, ownerID int64, storedName string, retryAfter time.Time) (domain.OperationStats, error) {
	intent := &documentsModel.FileCleanup{OwnerID: ownerID, FilenameStored: storedName, RetryAfter: retryAfter}
	started := time.Now()
	err := c.repo.QueueFileCleanup(ctx, intent)
	return domain.OperationStats{Queries: 1, Rows: 1, StatementDuration: time.Since(started)}, err
}

func (c fileCleanups) ListQueued(ctx context.Context) ([]domain.CleanupIntent, domain.OperationStats, error) {
	started := time.Now()
	rows, err := c.repo.ListQueuedFileCleanups(ctx)
	stats := domain.OperationStats{Queries: 1, Rows: int64(len(rows)), StatementDuration: time.Since(started)}
	if err != nil {
		return nil, stats, err
	}
	result := make([]domain.CleanupIntent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.CleanupIntent{
			ID: row.ID, TenantID: row.TenantID, OwnerID: row.OwnerID, FilenameStored: row.FilenameStored,
			RetryAfter: row.RetryAfter, CleanedAt: row.CleanedAt,
		})
	}
	return result, stats, nil
}

func (c fileCleanups) MarkComplete(ctx context.Context, intentID int64) (domain.OperationStats, error) {
	started := time.Now()
	err := c.repo.MarkQueuedFileCleanupComplete(ctx, intentID)
	return domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

func (c fileCleanups) MarkCompleteByFilename(ctx context.Context, storedName string) (domain.OperationStats, error) {
	started := time.Now()
	err := c.repo.MarkQueuedFileCleanupCompleteByFilename(ctx, storedName)
	return domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

func (c fileCleanups) ActivateByFilename(ctx context.Context, storedName string) (domain.OperationStats, error) {
	started := time.Now()
	err := c.repo.ActivateQueuedFileCleanupByFilename(ctx, storedName)
	return domain.OperationStats{Queries: 1, StatementDuration: time.Since(started)}, err
}

// --- public engine -------------------------------------------------------------

type engine struct{ service *application.Service }

func actor(value filestorage.Actor) domain.Actor {
	return domain.Actor{
		AccountID: value.AccountID,
		Name:      value.Name,
		Manager:   authorize.HasPermission(permissions.FilesManage, value.Permissions),
	}
}

func (e engine) ListFolders(ctx context.Context, who filestorage.Actor) (filestorage.FolderOverview, error) {
	overview, err := e.service.ListFolders(ctx, actor(who))
	if err != nil {
		return filestorage.FolderOverview{}, mapError(err)
	}
	result := filestorage.FolderOverview{
		Folders:            make([]filestorage.Folder, 0, len(overview.Folders)),
		CanManage:          overview.CanManage,
		CanUpload:          overview.CanUpload,
		StaffUploadEnabled: overview.StaffUploadEnabled,
		UsedBytes:          overview.UsedBytes,
		MaxBytes:           overview.MaxBytes,
	}
	for _, view := range overview.Folders {
		result.Folders = append(result.Folders, toFolder(view))
	}
	return result, nil
}

func (e engine) ListAudienceOptions(ctx context.Context, who filestorage.Actor) (filestorage.AudienceOptions, error) {
	roles, accounts, err := e.service.ListAudienceOptions(ctx, actor(who))
	if err != nil {
		return filestorage.AudienceOptions{}, mapError(err)
	}
	result := filestorage.AudienceOptions{
		Roles:    make([]filestorage.AudienceRole, 0, len(roles)),
		Accounts: make([]filestorage.AudienceAccount, 0, len(accounts)),
	}
	for _, role := range roles {
		result.Roles = append(result.Roles, filestorage.AudienceRole{ID: role.ID, Name: role.Name})
	}
	for _, account := range accounts {
		result.Accounts = append(result.Accounts, filestorage.AudienceAccount{AccountID: account.AccountID, FirstName: account.FirstName, LastName: account.LastName})
	}
	return result, nil
}

func folderInput(input filestorage.FolderInput) domain.FolderInput {
	return domain.FolderInput{Name: input.Name, Visibility: input.Visibility, RoleIDs: input.RoleIDs, AccountIDs: input.AccountIDs}
}

func (e engine) CreateFolder(ctx context.Context, input filestorage.FolderInput, who filestorage.Actor) (filestorage.Folder, error) {
	view, err := e.service.CreateFolder(ctx, folderInput(input), actor(who))
	return toFolder(view), mapError(err)
}

func (e engine) UpdateFolder(ctx context.Context, folderID int64, input filestorage.FolderInput, who filestorage.Actor) (filestorage.Folder, error) {
	view, err := e.service.UpdateFolder(ctx, folderID, folderInput(input), actor(who))
	return toFolder(view), mapError(err)
}

func (e engine) DeleteFolder(ctx context.Context, folderID int64, who filestorage.Actor) error {
	return mapError(e.service.DeleteFolder(ctx, folderID, actor(who)))
}

func (e engine) ListFiles(ctx context.Context, folderID int64, who filestorage.Actor) (filestorage.FileList, error) {
	folder, files, err := e.service.ListFiles(ctx, folderID, actor(who))
	if err != nil {
		return filestorage.FileList{}, mapError(err)
	}
	result := filestorage.FileList{
		Folder: toFolder(domain.FolderView{FolderListItem: domain.FolderListItem{Folder: folder, FileCount: int64(len(files))}}),
		Files:  make([]filestorage.File, 0, len(files)),
	}
	for _, file := range files {
		result.Files = append(result.Files, toFile(file.Document, file.CanDelete))
	}
	return result, nil
}

func (e engine) OpenFile(ctx context.Context, folderID, fileID int64, who filestorage.Actor) (filestorage.Content, error) {
	file, object, err := e.service.OpenFile(ctx, folderID, fileID, actor(who))
	if err != nil {
		return filestorage.Content{}, mapError(err)
	}
	return toContent(file, object), nil
}

func upload(value filestorage.Upload) domain.Upload {
	return domain.Upload{FilenameDisplay: value.Filename, ContentType: value.ContentType, Extension: value.Extension, Content: value.Content}
}

func (e engine) UploadFile(ctx context.Context, folderID int64, value filestorage.Upload, who filestorage.Actor) (filestorage.File, error) {
	file, err := e.service.UploadFile(ctx, folderID, upload(value), actor(who))
	if err != nil {
		return filestorage.File{}, mapError(err)
	}
	return toFile(file, true), nil
}

func (e engine) DeleteFile(ctx context.Context, folderID, fileID int64, who filestorage.Actor) error {
	return mapError(e.service.DeleteFile(ctx, folderID, fileID, actor(who)))
}

func (e engine) ListAttachments(ctx context.Context, announcementID int64) (filestorage.AttachmentList, error) {
	attachments, editable, err := e.service.ListAttachments(ctx, announcementID)
	if err != nil {
		return filestorage.AttachmentList{}, mapError(err)
	}
	return filestorage.AttachmentList{Attachments: toAttachments(attachments), Editable: editable}, nil
}

func (e engine) OpenAttachment(ctx context.Context, announcementID, attachmentID int64) (filestorage.Content, error) {
	attachment, object, err := e.service.OpenAttachment(ctx, announcementID, attachmentID)
	if err != nil {
		return filestorage.Content{}, mapError(err)
	}
	return toContent(attachment, object), nil
}

func (e engine) ListGuardianAttachments(ctx context.Context, accountID, announcementID int64) ([]filestorage.Attachment, error) {
	_, attachments, err := e.service.ListGuardianAttachments(ctx, accountID, announcementID)
	if err != nil {
		return nil, mapError(err)
	}
	return toAttachments(attachments), nil
}

func (e engine) OpenGuardianAttachment(ctx context.Context, accountID, announcementID, attachmentID int64) (filestorage.Content, error) {
	attachment, object, err := e.service.OpenGuardianAttachment(ctx, accountID, announcementID, attachmentID)
	if err != nil {
		return filestorage.Content{}, mapError(err)
	}
	return toContent(attachment, object), nil
}

func (e engine) UploadAttachment(ctx context.Context, announcementID int64, value filestorage.Upload, who filestorage.Actor) (filestorage.Attachment, error) {
	attachment, err := e.service.UploadAttachment(ctx, announcementID, upload(value), actor(who))
	if err != nil {
		return filestorage.Attachment{}, mapError(err)
	}
	return toAttachment(attachment), nil
}

func (e engine) DeleteAttachment(ctx context.Context, announcementID, attachmentID int64, who filestorage.Actor) error {
	return mapError(e.service.DeleteAttachment(ctx, announcementID, attachmentID, actor(who)))
}

func (e engine) QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error {
	return mapError(e.service.QueueAttachmentCleanupForAnnouncement(ctx, announcementID))
}

func (e engine) CountAttachments(ctx context.Context, announcementID int64) (int, error) {
	count, err := e.service.CountAttachments(ctx, announcementID)
	return count, mapError(err)
}

func (e engine) CleanupOrphanedFiles(ctx context.Context) (int, error) {
	removed, err := e.service.CleanupOrphanedFiles(ctx)
	return removed, mapError(err)
}

// --- mapping -------------------------------------------------------------------

func toFolder(view domain.FolderView) filestorage.Folder {
	return filestorage.Folder{
		ID:         view.ID,
		Name:       view.Name,
		Visibility: view.Visibility,
		FileCount:  view.FileCount,
		RoleIDs:    append([]int64{}, view.Audience.RoleIDs...),
		AccountIDs: append([]int64{}, view.Audience.AccountIDs...),
		CreatedAt:  view.CreatedAt,
	}
}

func toFile(document domain.Document, canDelete bool) filestorage.File {
	return filestorage.File{
		ID: document.ID, FolderID: document.OwnerID, Filename: document.FilenameDisplay,
		SizeBytes: document.SizeBytes, ContentType: document.ContentType,
		UploadedAt: document.CreatedAt, UploadedBy: document.UploadedBy, CanDelete: canDelete,
	}
}

func toAttachment(document domain.Document) filestorage.Attachment {
	return filestorage.Attachment{
		ID: document.ID, AnnouncementID: document.OwnerID, Filename: document.FilenameDisplay,
		SizeBytes: document.SizeBytes, ContentType: document.ContentType, UploadedAt: document.CreatedAt,
	}
}

func toAttachments(documents []domain.Document) []filestorage.Attachment {
	result := make([]filestorage.Attachment, 0, len(documents))
	for _, document := range documents {
		result = append(result, toAttachment(document))
	}
	return result
}

func toContent(document domain.Document, object ports.Object) filestorage.Content {
	return filestorage.Content{Object: &storedObject{Object: object}, ModTime: object.ModTime(), Filename: document.FilenameDisplay, ContentType: document.ContentType}
}

// storedObject adapts the backend's seekable object to the public random
// access shape. The local backend hands out *os.File, which reads at offsets
// natively; any other object is served through a guarded seek-and-read.
type storedObject struct {
	ports.Object
	mu   sync.Mutex
	size int64
	seen bool
}

func (o *storedObject) ReadAt(p []byte, off int64) (int, error) {
	if reader, ok := o.Object.(io.ReaderAt); ok {
		return reader.ReadAt(p, off)
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, err := o.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	// io.ReaderAt reports a short final read as io.EOF, never as
	// io.ErrUnexpectedEOF; a section reader would otherwise fail the last
	// chunk of a download instead of ending it.
	n, err := io.ReadFull(o.Object, p)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		err = io.EOF
	}
	return n, err
}

func (o *storedObject) Size() int64 {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.seen {
		return o.size
	}
	current, err := o.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0
	}
	end, err := o.Seek(0, io.SeekEnd)
	if err != nil {
		return 0
	}
	if _, err := o.Seek(current, io.SeekStart); err != nil {
		return 0
	}
	o.size, o.seen = end, true
	return end
}

// mappedError carries the public sentinel while keeping the internal message.
type mappedError struct {
	public error
	msg    string
}

func (e *mappedError) Error() string { return e.msg }
func (e *mappedError) Unwrap() error { return e.public }

var errorPairs = []struct{ internal, public error }{
	{domain.ErrNotFound, filestorage.ErrNotFound},
	{domain.ErrForbidden, filestorage.ErrForbidden},
	{domain.ErrInvalid, filestorage.ErrInvalid},
	{domain.ErrFolderNameTaken, filestorage.ErrFolderNameTaken},
	{domain.ErrQuotaExceeded, filestorage.ErrQuotaExceeded},
	{domain.ErrObjectNotFound, filestorage.ErrObjectNotFound},
	{domain.ErrAttachmentNotFound, filestorage.ErrAttachmentNotFound},
	{domain.ErrAttachmentPublished, filestorage.ErrAttachmentPublished},
	{domain.ErrAttachmentLimitReached, filestorage.ErrAttachmentLimitReached},
	{domain.ErrTenantRequired, filestorage.ErrTenantRequired},
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	for _, pair := range errorPairs {
		if errors.Is(err, pair.internal) {
			return &mappedError{public: pair.public, msg: err.Error()}
		}
	}
	return err
}
