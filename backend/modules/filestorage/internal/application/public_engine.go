package application

import (
	"context"
	"errors"
	"io"
	"sync"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// NewPublicEngine serves the public File Storage contract over service:
// actors carry the manager permission hasPermission grants, results and
// errors are the public ones.
func NewPublicEngine(service *Service, hasPermission func(string, []string) bool) filestorage.Engine {
	return engine{purger: purger{side: service}, service: service, hasPermission: hasPermission}
}

// NewPublicPurger serves the announcement side of the attachments over the
// purger service with the public errors.
func NewPublicPurger(service *Service) filestorage.AttachmentPurger {
	return purger{side: service}
}

// --- public engine -------------------------------------------------------------

type engine struct {
	purger
	service       *Service
	hasPermission func(string, []string) bool
}

// attachmentSide is the announcement side of the attachments, served by
// the whole service and by the attachment purger alone.
type attachmentSide interface {
	QueueAttachmentCleanupForAnnouncement(context.Context, int64) error
	CountAttachments(context.Context, int64) (int, error)
	AttachmentDigests(context.Context, int64, func(io.Reader) (string, int64, error)) ([]AttachmentDigest, error)
}

// purger serves filestorage.AttachmentPurger with the public errors.
type purger struct{ side attachmentSide }

func (e engine) actor(value filestorage.Actor) domain.Actor {
	return domain.Actor{
		AccountID: value.AccountID,
		Name:      value.Name,
		Manager:   e.hasPermission("files:manage", value.Permissions),
	}
}

func (e engine) ListFolders(ctx context.Context, who filestorage.Actor) (filestorage.FolderOverview, error) {
	overview, err := e.service.ListFolders(ctx, e.actor(who))
	if err != nil {
		return filestorage.FolderOverview{}, MapPublicError(err)
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
	roles, accounts, err := e.service.ListAudienceOptions(ctx, e.actor(who))
	if err != nil {
		return filestorage.AudienceOptions{}, MapPublicError(err)
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
	view, err := e.service.CreateFolder(ctx, folderInput(input), e.actor(who))
	return toFolder(view), MapPublicError(err)
}

func (e engine) UpdateFolder(ctx context.Context, folderID int64, input filestorage.FolderInput, who filestorage.Actor) (filestorage.Folder, error) {
	view, err := e.service.UpdateFolder(ctx, folderID, folderInput(input), e.actor(who))
	return toFolder(view), MapPublicError(err)
}

func (e engine) DeleteFolder(ctx context.Context, folderID int64, who filestorage.Actor) error {
	return MapPublicError(e.service.DeleteFolder(ctx, folderID, e.actor(who)))
}

func (e engine) ListFiles(ctx context.Context, folderID int64, who filestorage.Actor) (filestorage.FileList, error) {
	folder, files, err := e.service.ListFiles(ctx, folderID, e.actor(who))
	if err != nil {
		return filestorage.FileList{}, MapPublicError(err)
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
	file, object, err := e.service.OpenFile(ctx, folderID, fileID, e.actor(who))
	if err != nil {
		return filestorage.Content{}, MapPublicError(err)
	}
	return toContent(file, object), nil
}

func publicUpload(value filestorage.Upload) domain.Upload {
	return domain.Upload{FilenameDisplay: value.Filename, ContentType: value.ContentType, Extension: value.Extension, Content: value.Content}
}

func (e engine) UploadFile(ctx context.Context, folderID int64, value filestorage.Upload, who filestorage.Actor) (filestorage.File, error) {
	file, err := e.service.UploadFile(ctx, folderID, publicUpload(value), e.actor(who))
	if err != nil {
		return filestorage.File{}, MapPublicError(err)
	}
	return toFile(file, true), nil
}

func (e engine) DeleteFile(ctx context.Context, folderID, fileID int64, who filestorage.Actor) error {
	return MapPublicError(e.service.DeleteFile(ctx, folderID, fileID, e.actor(who)))
}

func (e engine) ListAttachments(ctx context.Context, announcementID int64) (filestorage.AttachmentList, error) {
	attachments, editable, err := e.service.ListAttachments(ctx, announcementID)
	if err != nil {
		return filestorage.AttachmentList{}, MapPublicError(err)
	}
	return filestorage.AttachmentList{Attachments: toAttachments(attachments), Editable: editable}, nil
}

func (e engine) OpenAttachment(ctx context.Context, announcementID, attachmentID int64) (filestorage.Content, error) {
	attachment, object, err := e.service.OpenAttachment(ctx, announcementID, attachmentID)
	if err != nil {
		return filestorage.Content{}, MapPublicError(err)
	}
	return toContent(attachment, object), nil
}

func (e engine) ListGuardianAttachments(ctx context.Context, accountID, announcementID int64) ([]filestorage.Attachment, error) {
	_, attachments, err := e.service.ListGuardianAttachments(ctx, accountID, announcementID)
	if err != nil {
		return nil, MapPublicError(err)
	}
	return toAttachments(attachments), nil
}

func (e engine) OpenGuardianAttachment(ctx context.Context, accountID, announcementID, attachmentID int64) (filestorage.Content, error) {
	attachment, object, err := e.service.OpenGuardianAttachment(ctx, accountID, announcementID, attachmentID)
	if err != nil {
		return filestorage.Content{}, MapPublicError(err)
	}
	return toContent(attachment, object), nil
}

func (e engine) OpenGuardianDeclarationProofAttachment(ctx context.Context, accountID, announcementID, studentID, attachmentID int64) (filestorage.Content, error) {
	attachment, object, err := e.service.OpenGuardianDeclarationProofAttachment(ctx, accountID, announcementID, studentID, attachmentID)
	if err != nil {
		return filestorage.Content{}, MapPublicError(err)
	}
	return toContent(attachment, object), nil
}

func (e engine) UploadAttachment(ctx context.Context, announcementID int64, value filestorage.Upload, who filestorage.Actor) (filestorage.Attachment, error) {
	attachment, err := e.service.UploadAttachment(ctx, announcementID, publicUpload(value), e.actor(who))
	if err != nil {
		return filestorage.Attachment{}, MapPublicError(err)
	}
	return toAttachment(attachment), nil
}

func (e engine) DeleteAttachment(ctx context.Context, announcementID, attachmentID int64, who filestorage.Actor) error {
	return MapPublicError(e.service.DeleteAttachment(ctx, announcementID, attachmentID, e.actor(who)))
}

func (p purger) QueueAttachmentCleanupForAnnouncement(ctx context.Context, announcementID int64) error {
	return MapPublicError(p.side.QueueAttachmentCleanupForAnnouncement(ctx, announcementID))
}

func (p purger) CountAttachments(ctx context.Context, announcementID int64) (int, error) {
	count, err := p.side.CountAttachments(ctx, announcementID)
	return count, MapPublicError(err)
}

func (p purger) AttachmentDigests(ctx context.Context, announcementID int64, digest func(io.Reader) (string, int64, error)) ([]filestorage.AttachmentDigest, error) {
	digests, err := p.side.AttachmentDigests(ctx, announcementID, digest)
	if err != nil {
		return nil, MapPublicError(err)
	}
	out := make([]filestorage.AttachmentDigest, 0, len(digests))
	for _, digest := range digests {
		out = append(out, filestorage.AttachmentDigest(digest))
	}
	return out, nil
}

func (e engine) CleanupOrphanedFiles(ctx context.Context) (int, error) {
	removed, err := e.service.CleanupOrphanedFiles(ctx)
	return removed, MapPublicError(err)
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

func MapPublicError(err error) error {
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
