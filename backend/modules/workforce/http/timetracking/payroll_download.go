package timetracking

import (
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// PayrollDownloadResource serves the one-time download link the payroll
// office receives by e-mail (#4805). It is public: the link is the only
// credential, so the answer for an unknown, expired or used link is the
// same 404, and the link opens with POST only. A mail scanner that follows
// every link with GET must not spend it.
type PayrollDownloadResource struct {
	dispatch workforce.PayrollDispatch
	logger   *slog.Logger
}

// NewPayrollDownloadResource requires the dispatch.
func NewPayrollDownloadResource(dispatch workforce.PayrollDispatch, logger *slog.Logger) *PayrollDownloadResource {
	if dispatch == nil {
		panic("payroll download resource: the payroll dispatch is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PayrollDownloadResource{dispatch: dispatch, logger: logger}
}

// PublicRouter serves POST /{link}, where link is "<school id>.<token>".
func (rs *PayrollDownloadResource) PublicRouter() chi.Router {
	router := chi.NewRouter()
	router.Post("/{link}", rs.download)
	return router
}

var errPayrollLinkMalformed = errors.New("payroll download link is malformed")

func parsePayrollLink(link string) (int64, string, error) {
	school, token, found := strings.Cut(link, ".")
	if !found || token == "" || len(token) > 128 {
		return 0, "", errPayrollLinkMalformed
	}
	tenantID, err := strconv.ParseInt(school, 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, "", errPayrollLinkMalformed
	}
	return tenantID, token, nil
}

func (rs *PayrollDownloadResource) download(w http.ResponseWriter, r *http.Request) {
	tenantID, token, err := parsePayrollLink(chi.URLParam(r, "link"))
	if err == nil {
		var file workforce.PayrollDownload
		file, err = rs.dispatch.Download(r.Context(), tenantID, token)
		if err == nil {
			writePayrollFile(w, file)
			return
		}
	}
	if errors.Is(err, workforce.ErrPayrollDownloadUnavailable) || errors.Is(err, errPayrollLinkMalformed) {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(workforce.ErrPayrollDownloadUnavailable, common.CodeWorkforcePayrollDownloadUnavailable))
		return
	}
	rs.logger.Error("failed to open payroll download", "error", err.Error())
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

func writePayrollFile(w http.ResponseWriter, file workforce.PayrollDownload) {
	contentType := file.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": file.Filename}))
	w.Header().Set("Content-Length", fmt.Sprint(len(file.Data)))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
