package filestore

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	apiDocuments "github.com/moto-nrw/project-phoenix/api/common/documents"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	filestoreSvc "github.com/moto-nrw/project-phoenix/services/filestore"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// --- wire types ------------------------------------------------------------

// FolderRequest is the body of a folder create or update.
type FolderRequest struct {
	Name       string  `json:"name"`
	Visibility string  `json:"visibility"`
	RoleIDs    []int64 `json:"role_ids"`
	AccountIDs []int64 `json:"account_ids"`
}

// FolderResponse is one folder on the wire.
type FolderResponse struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Visibility string    `json:"visibility"`
	FileCount  int64     `json:"file_count"`
	RoleIDs    []int64   `json:"role_ids"`
	AccountIDs []int64   `json:"account_ids"`
	CreatedAt  time.Time `json:"created_at"`
}

// FolderListResponse is the folder overview: what the caller sees plus what
// the caller may do, so the UI never has to guess authority.
type FolderListResponse struct {
	Folders            []FolderResponse `json:"folders"`
	CanManage          bool             `json:"can_manage"`
	CanUpload          bool             `json:"can_upload"`
	StaffUploadEnabled bool             `json:"staff_upload_enabled"`
	UsedBytes          int64            `json:"used_bytes"`
	MaxBytes           int64            `json:"max_bytes"`
}

// FileResponse is one file on the wire.
type FileResponse struct {
	ID          int64     `json:"id"`
	FolderID    int64     `json:"folder_id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	UploadedBy  int64     `json:"uploaded_by"`
	CanDelete   bool      `json:"can_delete"`
}

// FileListResponse is the file list of one folder.
type FileListResponse struct {
	Folder FolderResponse `json:"folder"`
	Files  []FileResponse `json:"files"`
}

// AudienceResponse lists what a folder can be shared with.
type AudienceResponse struct {
	Roles    []AudienceRoleResponse    `json:"roles"`
	Accounts []AudienceAccountResponse `json:"accounts"`
}

// AudienceRoleResponse is one shareable role.
type AudienceRoleResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AudienceAccountResponse is one shareable person.
type AudienceAccountResponse struct {
	AccountID int64  `json:"account_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func emptyIfNil(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}

func newFolderResponse(view *filestoreSvc.FolderView) FolderResponse {
	return FolderResponse{
		ID:         view.ID,
		Name:       view.Name,
		Visibility: view.Visibility,
		FileCount:  view.FileCount,
		RoleIDs:    emptyIfNil(view.Audience.RoleIDs),
		AccountIDs: emptyIfNil(view.Audience.AccountIDs),
		CreatedAt:  view.CreatedAt,
	}
}

func newFileResponse(file *filestore.File, canDelete bool) FileResponse {
	return FileResponse{
		ID:          file.ID,
		FolderID:    file.FolderID,
		Filename:    file.FilenameDisplay,
		SizeBytes:   file.SizeBytes,
		ContentType: file.ContentType,
		UploadedAt:  file.CreatedAt,
		UploadedBy:  file.UploadedBy,
		CanDelete:   canDelete,
	}
}

func folderInput(req FolderRequest) filestoreSvc.FolderInput {
	return filestoreSvc.FolderInput{
		Name:       req.Name,
		Visibility: req.Visibility,
		RoleIDs:    req.RoleIDs,
		AccountIDs: req.AccountIDs,
	}
}

// --- folder handlers ---------------------------------------------------------

// listFolders serves GET /folders.
func (rs *Resource) listFolders(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	overview, err := rs.Service.ListFolders(r.Context(), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := &FolderListResponse{
		Folders:            make([]FolderResponse, 0, len(overview.Folders)),
		CanManage:          overview.CanManage,
		CanUpload:          overview.CanUpload,
		StaffUploadEnabled: overview.StaffUploadEnabled,
		UsedBytes:          overview.UsedBytes,
		MaxBytes:           overview.MaxBytes,
	}
	for _, folder := range overview.Folders {
		resp.Folders = append(resp.Folders, newFolderResponse(folder))
	}
	common.Respond(w, r, http.StatusOK, resp, "Folders retrieved successfully")
}

// createFolder serves POST /folders.
func (rs *Resource) createFolder(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req FolderRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	view, err := rs.Service.CreateFolder(r.Context(), folderInput(req), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := newFolderResponse(view)
	common.Respond(w, r, http.StatusCreated, &resp, "Folder created successfully")
}

// updateFolder serves PUT /folders/{folderId}.
func (rs *Resource) updateFolder(w http.ResponseWriter, r *http.Request) {
	folderID, ok := common.ParseInt64IDWithError(w, r, "folderId", "invalid folder ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req FolderRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	view, err := rs.Service.UpdateFolder(r.Context(), folderID, folderInput(req), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := newFolderResponse(view)
	common.Respond(w, r, http.StatusOK, &resp, "Folder updated successfully")
}

// deleteFolder serves DELETE /folders/{folderId}. The bytes of the folder's
// files are reclaimed by the scheduler through the intents the service queues
// before the rows cascade.
func (rs *Resource) deleteFolder(w http.ResponseWriter, r *http.Request) {
	folderID, ok := common.ParseInt64IDWithError(w, r, "folderId", "invalid folder ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	if err := rs.Service.DeleteFolder(r.Context(), folderID, actor); err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]int64{"folder_id": folderID}, "Folder deleted successfully")
}

// listAudience serves GET /audience: the roles and persons a folder can be
// shared with.
func (rs *Resource) listAudience(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	options, err := rs.Service.ListAudienceOptions(r.Context(), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := &AudienceResponse{
		Roles:    make([]AudienceRoleResponse, 0, len(options.Roles)),
		Accounts: make([]AudienceAccountResponse, 0, len(options.Accounts)),
	}
	for _, role := range options.Roles {
		resp.Roles = append(resp.Roles, AudienceRoleResponse{ID: role.ID, Name: role.Name})
	}
	for _, account := range options.Accounts {
		resp.Accounts = append(resp.Accounts, AudienceAccountResponse{
			AccountID: account.AccountID,
			FirstName: account.FirstName,
			LastName:  account.LastName,
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Audience retrieved successfully")
}

// --- file handlers -----------------------------------------------------------

// listFiles serves GET /folders/{folderId}/files.
func (rs *Resource) listFiles(w http.ResponseWriter, r *http.Request) {
	folderID, ok := common.ParseInt64IDWithError(w, r, "folderId", "invalid folder ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	folder, files, err := rs.Service.ListFiles(r.Context(), folderID, actor)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// Retry only objects whose prior post-commit removal did not finish. The
	// hooks run after this read transaction commits.
	rs.retryCleanups(r.Context(), folderID, actor, "retry")

	resp := &FileListResponse{
		Folder: newFolderResponse(&filestoreSvc.FolderView{
			FolderListItem: filestore.FolderListItem{Folder: *folder, FileCount: int64(len(files))},
		}),
		Files: make([]FileResponse, 0, len(files)),
	}
	for _, file := range files {
		canDelete, err := rs.Service.CanDeleteFile(r.Context(), file, actor)
		if err != nil {
			renderError(w, r, err)
			return
		}
		resp.Files = append(resp.Files, newFileResponse(file, canDelete))
	}
	common.Respond(w, r, http.StatusOK, resp, "Files retrieved successfully")
}

// retryCleanups schedules removal of objects whose unlink did not finish, for
// the folder in the URL only. Queued upload intents are the scheduler's job —
// see the student document handler for the reasoning.
func (rs *Resource) retryCleanups(ctx context.Context, folderID int64, actor filestoreSvc.Actor, source string) {
	coordinator, err := rs.coordinator()
	if err != nil {
		rs.getLogger().Warn("file cleanup retry unavailable", "error", err)
		return
	}
	tenantID := tenant.FromContext(ctx)
	files, err := rs.Service.ListDeletedFilesPendingCleanup(ctx, folderID, actor)
	if err != nil {
		rs.getLogger().Warn("file cleanup retry lookup failed",
			"folder_id", folderID,
			"error", err)
		return
	}
	for _, file := range files {
		fileID, storedName := file.ID, file.FilenameStored
		tenant.RegisterAfterCommit(ctx, func() {
			coordinator.CleanupDocument(tenantID, folderID, fileID, storedName, source)
		})
	}
}

// uploadFile serves POST /folders/{folderId}/files (multipart: file). Same
// protocol as child documents: intent → object → metadata → settle.
func (rs *Resource) uploadFile(w http.ResponseWriter, r *http.Request) {
	folderID, ok := common.ParseInt64IDWithError(w, r, "folderId", "invalid folder ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	tenantID := tenant.FromContext(r.Context())
	if tenantID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("no tenant context")))
		return
	}
	coordinator, err := rs.coordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	uploaded, err := common.ParseOfficeFileWithLimits(w, r, "file", maxFile, maxBody)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer common.CloseFile(uploaded.File)

	// Authorize BEFORE anything is written, so a request that was never
	// allowed costs neither disk nor a cleanup row.
	if err := rs.Service.AuthorizeUpload(r.Context(), folderID, actor); err != nil {
		renderError(w, r, err)
		return
	}

	storedName, err := apiDocuments.NewStoredName(common.DocumentFileExtension(uploaded.ContentType))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	if err := rs.Service.QueueFileCleanup(r.Context(), folderID, storedName); err != nil {
		rs.getLogger().Error("file upload cleanup intent failed",
			"folder_id", folderID,
			"error", err)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	uploadCtx, cancelUpload := context.WithTimeout(r.Context(), filestoreSvc.UploadDeadline)
	defer cancelUpload()

	size, ok := rs.writeObject(w, r, uploadCtx, coordinator, tenantID, folderID, storedName, uploaded.File)
	if !ok {
		return
	}

	file, err := rs.Service.CreateFile(uploadCtx, filestoreSvc.CreateFileInput{
		FolderID:        folderID,
		FilenameDisplay: uploaded.Filename,
		FilenameStored:  storedName,
		SizeBytes:       size,
		ContentType:     uploaded.ContentType,
	}, actor)
	if err != nil {
		// Only request-shaped rejections prove the row never landed; anything
		// deeper is left to the queued intent (see the coordinator).
		rejectedBeforeCommit := errors.Is(err, filestoreSvc.ErrInvalid) ||
			errors.Is(err, filestoreSvc.ErrForbidden) ||
			errors.Is(err, filestoreSvc.ErrQuotaExceeded)
		coordinator.ReleaseFailedUpload(r.Context(), tenantID, folderID, storedName, rejectedBeforeCommit, err)
		renderError(w, r, err)
		return
	}

	resp := newFileResponse(file, true)
	common.Respond(w, r, http.StatusCreated, &resp, "File uploaded successfully")
}

// writeObject stores the upload bytes within the upload deadline and reports
// whether the request may continue. On a failed or late write the queued
// intent is activated rather than settled: a half-written object must stay
// reachable for the scheduler (see the student document handler for why).
func (rs *Resource) writeObject(w http.ResponseWriter, r *http.Request, uploadCtx context.Context, coordinator *apiDocuments.Coordinator, tenantID, folderID int64, storedName string, source io.Reader) (int64, bool) {
	size, err := coordinator.Save(uploadCtx, tenantID, storedName, source)
	if err == nil && uploadCtx.Err() != nil {
		err = errors.New("upload exceeded its deadline")
	}
	if err == nil {
		return size, true
	}
	if activateErr := rs.Service.ActivateQueuedCleanup(r.Context(), storedName); activateErr != nil {
		rs.getLogger().Error("file cleanup intent activation failed after write error",
			"folder_id", folderID,
			"error", activateErr)
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
	return 0, false
}

// downloadFile serves GET /folders/{folderId}/files/{fileId}/download
// (?inline=1 for in-browser viewing of PDFs and images).
func (rs *Resource) downloadFile(w http.ResponseWriter, r *http.Request) {
	folderID, ok := common.ParseInt64IDWithError(w, r, "folderId", "invalid folder ID")
	if !ok {
		return
	}
	fileID, ok := common.ParseInt64IDWithError(w, r, "fileId", "invalid file ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	coordinator, err := rs.coordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	file, err := rs.Service.ResolveDownload(r.Context(), folderID, fileID, actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	// ?inline=1 opens PDFs and images in the browser instead of downloading;
	// office files ignore it and download either way.
	served := false
	if r.URL.Query().Get("inline") == "1" {
		served = coordinator.ServeInline(w, r, tenant.FromContext(r.Context()), file.FilenameStored, file.FilenameDisplay, file.ContentType)
	} else {
		served = coordinator.Serve(w, r, tenant.FromContext(r.Context()), file.FilenameStored, file.FilenameDisplay, file.ContentType)
	}
	if !served {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("file not found")))
	}
}

// deleteFile serves DELETE /folders/{folderId}/files/{fileId}: audited soft
// delete of the row, then removal of the bytes after commit.
func (rs *Resource) deleteFile(w http.ResponseWriter, r *http.Request) {
	folderID, ok := common.ParseInt64IDWithError(w, r, "folderId", "invalid folder ID")
	if !ok {
		return
	}
	fileID, ok := common.ParseInt64IDWithError(w, r, "fileId", "invalid file ID")
	if !ok {
		return
	}
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	coordinator, err := rs.coordinator()
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	file, err := rs.Service.DeleteFile(r.Context(), folderID, fileID, actor)
	if err != nil {
		if !modelBase.IsNoRows(err) {
			renderError(w, r, err)
			return
		}
		// Already soft-deleted: retry the byte removal instead of a 404 the
		// caller cannot act on.
		file, err = rs.Service.ResolveCleanup(r.Context(), folderID, fileID, actor)
		if err != nil {
			renderError(w, r, err)
			return
		}
	}

	if !file.IsFileDeleted() {
		tenantID := tenant.FromContext(r.Context())
		id, storedName := file.ID, file.FilenameStored
		tenant.RegisterAfterCommit(r.Context(), func() {
			coordinator.CleanupDocument(tenantID, folderID, id, storedName, "delete")
		})
	}
	common.Respond(w, r, http.StatusOK, map[string]int64{"folder_id": folderID}, "File deleted successfully")
}
