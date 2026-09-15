package files

import (
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/filestorage"
)

// --- wire types ------------------------------------------------------------

// FolderRequest is the body of a folder create or update. The share lists
// arrive as decimal strings; a JSON number still decodes (common.JSONID), so
// nothing that already talks to this endpoint breaks.
type FolderRequest struct {
	Name       string          `json:"name"`
	Visibility string          `json:"visibility"`
	RoleIDs    []common.JSONID `json:"role_ids"`
	AccountIDs []common.JSONID `json:"account_ids"`
}

// FolderResponse is one folder on the wire. Ids are decimal strings; see
// idString.
type FolderResponse struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Visibility string    `json:"visibility"`
	FileCount  int64     `json:"file_count"`
	RoleIDs    []string  `json:"role_ids"`
	AccountIDs []string  `json:"account_ids"`
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
	ID          string    `json:"id"`
	FolderID    string    `json:"folder_id"`
	Filename    string    `json:"filename"`
	SizeBytes   int64     `json:"size_bytes"`
	ContentType string    `json:"content_type"`
	UploadedAt  time.Time `json:"uploaded_at"`
	UploadedBy  string    `json:"uploaded_by"`
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
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AudienceAccountResponse is one shareable person.
type AudienceAccountResponse struct {
	AccountID string `json:"account_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func newFolderResponse(folder filestorage.Folder) FolderResponse {
	return FolderResponse{
		ID:         idString(folder.ID),
		Name:       folder.Name,
		Visibility: folder.Visibility,
		FileCount:  folder.FileCount,
		RoleIDs:    idStrings(folder.RoleIDs),
		AccountIDs: idStrings(folder.AccountIDs),
		CreatedAt:  folder.CreatedAt,
	}
}

func newFileResponse(file filestorage.File) FileResponse {
	return FileResponse{
		ID:          idString(file.ID),
		FolderID:    idString(file.FolderID),
		Filename:    file.Filename,
		SizeBytes:   file.SizeBytes,
		ContentType: file.ContentType,
		UploadedAt:  file.UploadedAt,
		UploadedBy:  idString(file.UploadedBy),
		CanDelete:   file.CanDelete,
	}
}

func folderInput(req FolderRequest) filestorage.FolderInput {
	return filestorage.FolderInput{
		Name:       req.Name,
		Visibility: req.Visibility,
		RoleIDs:    int64IDs(req.RoleIDs),
		AccountIDs: int64IDs(req.AccountIDs),
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
	overview, err := rs.files.ListFolders(r.Context(), actor)
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
	folder, err := rs.files.CreateFolder(r.Context(), folderInput(req), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := newFolderResponse(folder)
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
	folder, err := rs.files.UpdateFolder(r.Context(), folderID, folderInput(req), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := newFolderResponse(folder)
	common.Respond(w, r, http.StatusOK, &resp, "Folder updated successfully")
}

// deleteFolder serves DELETE /folders/{folderId}. The bytes of the folder's
// files are reclaimed by the scheduler through the intents the owner queues
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
	if err := rs.files.DeleteFolder(r.Context(), folderID, actor); err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"folder_id": idString(folderID)}, "Folder deleted successfully")
}

// listAudience serves GET /audience: the roles and persons a folder can be
// shared with.
func (rs *Resource) listAudience(w http.ResponseWriter, r *http.Request) {
	actor, err := actorFromRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	options, err := rs.files.ListAudienceOptions(r.Context(), actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := &AudienceResponse{
		Roles:    make([]AudienceRoleResponse, 0, len(options.Roles)),
		Accounts: make([]AudienceAccountResponse, 0, len(options.Accounts)),
	}
	for _, role := range options.Roles {
		resp.Roles = append(resp.Roles, AudienceRoleResponse{ID: idString(role.ID), Name: role.Name})
	}
	for _, account := range options.Accounts {
		resp.Accounts = append(resp.Accounts, AudienceAccountResponse{
			AccountID: idString(account.AccountID),
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
	list, err := rs.files.ListFiles(r.Context(), folderID, actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := &FileListResponse{
		Folder: newFolderResponse(list.Folder),
		Files:  make([]FileResponse, 0, len(list.Files)),
	}
	for _, file := range list.Files {
		resp.Files = append(resp.Files, newFileResponse(file))
	}
	common.Respond(w, r, http.StatusOK, resp, "Files retrieved successfully")
}

// uploadFile serves POST /folders/{folderId}/files (multipart: file).
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
	upload, closeUpload, err := parseUpload(w, r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	defer closeUpload()

	file, err := rs.files.UploadFile(r.Context(), folderID, upload, actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	resp := newFileResponse(file)
	common.Respond(w, r, http.StatusCreated, &resp, "File uploaded successfully")
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
	content, err := rs.files.OpenFile(r.Context(), folderID, fileID, actor)
	if err != nil {
		renderError(w, r, err)
		return
	}
	rs.serveContent(w, r, content)
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
	if err := rs.files.DeleteFile(r.Context(), folderID, fileID, actor); err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"folder_id": idString(folderID)}, "File deleted successfully")
}
