package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	configAPI "github.com/moto-nrw/project-phoenix/api/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/uptrace/bun"
)

func newSettingsResource(operations configAPI.Operations, homeLayouts configAPI.HomeLayoutOperations, references func(context.Context, string, string) (bool, error), db *bun.DB) *configAPI.SettingsResource {
	settingsRuntime := configAPI.NewRuntime(configAPI.RuntimeDependencies{
		Protected: func(r chi.Router, fn func(chi.Router, configAPI.Middleware)) {
			apiCommon.ProtectedTenantGroup(r, db, fn)
		},
		Permission: func(access configAPI.Access) configAPI.Middleware {
			switch access {
			case configAPI.AccessRead:
				return apiCommon.RequireConfigRead()
			case configAPI.AccessManage:
				return apiCommon.RequireConfigManage()
			case configAPI.AccessReadOrWrite:
				return apiCommon.RequireConfigReadOrWrite()
			default:
				return apiCommon.RequireConfigWrite()
			}
		},
		TenantGuard: apiCommon.TenantOperationMiddleware,
		RequestActor: func(ctx context.Context) (int64, int64, []string) {
			principal, err := apiCommon.CurrentPrincipal(ctx)
			if err != nil {
				return 0, 0, nil
			}
			return principal.TenantID(), principal.AccountID(), principal.Permissions()
		},
		Editable:  apiCommon.CanEditConfig,
		Success:   apiCommon.Respond,
		NoContent: apiCommon.RespondNoContent,
		Failure: func(w http.ResponseWriter, r *http.Request, status int, err error) {
			switch status {
			case http.StatusBadRequest:
				apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
			case http.StatusForbidden:
				apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
			case http.StatusNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
			}
		},
		ImageUpload: func(w http.ResponseWriter, r *http.Request, field string, maxBody int64) (*configAPI.UploadedFile, error) {
			file, err := apiCommon.ParseImage(w, r, field, maxBody)
			if err != nil {
				return nil, err
			}
			return &configAPI.UploadedFile{File: file.File, ContentType: file.ContentType}, nil
		},
		PDFUpload: func(w http.ResponseWriter, r *http.Request, field string, maxFile, maxBody int64) (*configAPI.UploadedFile, error) {
			file, err := apiCommon.ParsePDFWithLimits(w, r, field, maxFile, maxBody)
			if err != nil {
				return nil, err
			}
			return &configAPI.UploadedFile{File: file.File, ContentType: file.ContentType}, nil
		},
		ImageSave:  apiCommon.SaveImage,
		PDFSave:    apiCommon.SavePDF,
		FileRemove: apiCommon.RemoveImage,
		StoredPath: apiCommon.ResolveStoredPath,
		LegalDocumentReference: func(ctx context.Context, storedURL string) (bool, error) {
			publicURL := enrollmentSvc.PublicEnrollmentLegalDocumentURL(storedURL)
			referenced, err := references(ctx, storedURL, publicURL)
			if err != nil {
				return false, fmt.Errorf("check AGB document references: %w", err)
			}
			return referenced, nil
		},
	})
	return configAPI.NewSettingsResource(operations, homeLayouts, settingsRuntime)
}
