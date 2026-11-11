package datareturn

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

var errNoBearer = errors.New("data return: send the token as Authorization: Bearer")

// bearer reads the token from the Authorization header only: a token in a
// URL would land in logs and browser histories.
func bearer(r *http.Request) (string, bool) {
	scheme, token, found := strings.Cut(r.Header.Get("Authorization"), " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func (rs *Resource) describe(w http.ResponseWriter, r *http.Request) {
	token, ok := bearer(r)
	if !ok {
		renderFailure(w, r, errNoBearer)
		return
	}
	pkg, err := rs.packages.Package(r.Context(), token)
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	common.RespondWithJSON(w, r, http.StatusOK, pkg)
}

func (rs *Resource) download(w http.ResponseWriter, r *http.Request) {
	token, ok := bearer(r)
	if !ok {
		renderFailure(w, r, errNoBearer)
		return
	}
	started := false
	err := rs.packages.Download(r.Context(), token, func(pkg organizationtenancy.DataReturnPackage) io.Writer {
		started = true
		writeArchiveHeaders(w, pkg)
		return w
	})
	if err != nil && !started {
		renderFailure(w, r, err)
		return
	}
	if err != nil {
		// The body is cut short: the client sees fewer bytes than
		// Content-Length and a digest that does not match.
		slog.Default().WarnContext(r.Context(), "data return download interrupted", "error", err.Error())
	}
}

func writeArchiveHeaders(w http.ResponseWriter, pkg organizationtenancy.DataReturnPackage) {
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Length", strconv.FormatInt(pkg.ArchiveBytes, 10))
	w.Header().Set("Content-Disposition", `attachment; filename="moto-rueckgabe-`+pkg.Package+`.zip"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if digest, err := hex.DecodeString(pkg.ArchiveSHA256); err == nil {
		// RFC 9530: the client verifies the archive it received.
		w.Header().Set("Repr-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(digest)+":")
	}
	w.WriteHeader(http.StatusOK)
}

// renderFailure answers every token that opens nothing with the same 401,
// so a caller learns nothing about why.
func renderFailure(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errNoBearer), errors.Is(err, organizationtenancy.ErrDataReturnDenied):
		w.Header().Set("WWW-Authenticate", `Bearer realm="moto-data-return"`)
		common.RenderError(w, r, common.ErrorUnauthorizedWithCode(organizationtenancy.ErrDataReturnDenied, common.CodeGeneralPermission))
	case errors.Is(err, organizationtenancy.ErrDataReturnArchiveMissing):
		common.RenderError(w, r, common.ErrorServiceUnavailableWithCode(errors.New("the package is not available; contact moto"), common.CodeGeneralUnavailable))
	default:
		slog.Default().ErrorContext(r.Context(), "data return failed", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("data return failed")))
	}
}
