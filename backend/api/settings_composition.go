package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
)

func newSettingsResource(operations settingsHTTP.Operations, homeLayouts settingsHTTP.HomeLayoutOperations, references func(context.Context, string, string) (bool, error)) *settingsHTTP.SettingsResource {
	settingsRuntime := settingsHTTP.NewRuntime(settingsHTTP.RuntimeDependencies{
		Protected: func(r chi.Router, fn func(chi.Router, settingsHTTP.Middleware)) {
			ProtectedTenantRoutes(r, fn)
		},
		Permission: func(access settingsHTTP.Access) settingsHTTP.Middleware {
			switch access {
			case settingsHTTP.AccessRead:
				return RequireConfigRead()
			case settingsHTTP.AccessManage:
				return RequireConfigManage()
			case settingsHTTP.AccessReadOrWrite:
				return RequireConfigReadOrWrite()
			default:
				return RequireConfigWrite()
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
		Editable:  CanEditConfig,
		Success:   apiCommon.Respond,
		NoContent: apiCommon.RespondNoContent,
		Failure: func(w http.ResponseWriter, r *http.Request, status int, err error) {
			switch status {
			case http.StatusBadRequest:
				apiCommon.RenderError(w, r, settingsInputError(err))
			case http.StatusForbidden:
				apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
			case http.StatusNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
			}
		},
		ImageUpload: func(w http.ResponseWriter, r *http.Request, field string, maxBody int64) (*settingsHTTP.UploadedFile, error) {
			file, err := apiCommon.ParseImage(w, r, field, maxBody)
			if err != nil {
				return nil, err
			}
			return &settingsHTTP.UploadedFile{File: file.File, ContentType: file.ContentType}, nil
		},
		PDFUpload: func(w http.ResponseWriter, r *http.Request, field string, maxFile, maxBody int64) (*settingsHTTP.UploadedFile, error) {
			file, err := apiCommon.ParsePDFWithLimits(w, r, field, maxFile, maxBody)
			if err != nil {
				return nil, err
			}
			return &settingsHTTP.UploadedFile{File: file.File, ContentType: file.ContentType}, nil
		},
		ImageSave:  saveImageUpload,
		PDFSave:    savePDFUpload,
		FileRemove: (nativeUploads.UploadFiles{}).RemoveUpload,
		StoredPath: nativeUploads.ResolveStoredPath,
		LegalDocumentReference: func(ctx context.Context, storedURL string) (bool, error) {
			publicURL := enrollmentAPI.PublicLegalDocumentURL(storedURL)
			referenced, err := references(ctx, storedURL, publicURL)
			if err != nil {
				return false, fmt.Errorf("check AGB document references: %w", err)
			}
			return referenced, nil
		},
	})
	return settingsHTTP.NewSettingsResource(operations, homeLayouts, settingsRuntime)
}

// settingsInputError renders a rejected setting value. A visible attendance
// window longer than its retention names the window that blocks it (#3957),
// so the person reads which value is allowed.
func settingsInputError(err error) render.Renderer {
	window, ok := errors.AsType[*settingsModule.AttendanceWindowError](err)
	if !ok {
		return apiCommon.ErrorInvalidRequest(err)
	}
	if window.VisibleWritten() {
		return apiCommon.ErrorInvalidRequestWithDetails(err, apiCommon.CodeSettingsAttendanceVisibleExceedsRetention,
			map[string]any{"retention_days": window.RetentionDays})
	}
	return apiCommon.ErrorInvalidRequestWithDetails(err, apiCommon.CodeSettingsAttendanceRetentionBelowVisible,
		map[string]any{"visible_days": window.VisibleDays})
}
