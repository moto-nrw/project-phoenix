package enrollmenthttp

import (
	"bytes"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The proof of benefits of the public holiday registration (#5238): parents
// upload it through the public link, the Träger-Büro opens it from the
// detail of the registration. The transport checks are the ones of the file
// field of the online registration (#4783): the content type comes from the
// bytes, the extension must name it and the size is measured.

// uploadProof stores one proof of benefits for the link's period.
func (rs *HolidayRegistrationResource) uploadProof(w http.ResponseWriter, r *http.Request) {
	upload, err := parseHolidayProof(w, r)
	if err != nil {
		renderUploadRefusal(w, r, err)
		return
	}
	link, err := rs.resolveLink(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	receipt, err := rs.proofs.UploadHolidayProof(r.Context(), link, remoteIPFromRequest(r), upload)
	if err != nil {
		renderHolidayRegistrationError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, UploadReceipt(receipt), "Proof stored")
}

// parseHolidayProof reads the multipart upload of a proof with the limits
// and checks of the online registration's uploads.
func parseHolidayProof(w http.ResponseWriter, r *http.Request) (capability.HolidayProofUpload, error) {
	uploaded, err := common.ParseDocumentWithLimits(w, r, "file", maxEnrollmentUploadSize, maxEnrollmentUploadBody)
	if err != nil {
		return capability.HolidayProofUpload{}, err
	}
	defer common.CloseFile(uploaded.File)
	if !extensionMatchesContent(uploaded.Filename, uploaded.ContentType) {
		return capability.HolidayProofUpload{}, &uploadContentMismatch{filename: uploaded.Filename}
	}
	content, err := io.ReadAll(io.LimitReader(uploaded.File, maxEnrollmentUploadSize+1))
	if err != nil || int64(len(content)) > maxEnrollmentUploadSize {
		return capability.HolidayProofUpload{}, errors.New("cannot read upload within its size limit")
	}
	return capability.HolidayProofUpload{
		Filename: uploaded.Filename, ContentType: normalizeUploadContentType(uploaded.ContentType),
		Extension: common.DocumentFileExtension(uploaded.ContentType), SizeBytes: int64(len(content)), Content: content,
	}, nil
}

// openProof serves one proof of benefits of a registration inline. The read
// is recorded in the Träger transaction, which commits before a byte leaves.
func (rs *HolidayPeriodResource) openProof(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	proofID, err := common.ParseIDParam(r, "proofId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	file, err := rs.review.OpenHolidayRegistrationProof(r.Context(), reviewer, periodID, registrationID, proofID)
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	// An inline proof is the parents' file on the application's own origin:
	// the sandbox denies it scripts, forms and same-origin access, and no
	// cache keeps it.
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": file.Filename}))
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, file.Filename, time.Time{}, bytes.NewReader(file.Content))
}
