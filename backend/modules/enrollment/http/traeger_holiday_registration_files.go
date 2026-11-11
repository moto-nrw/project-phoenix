package enrollmenthttp

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// The files of the holiday registrations in the Träger-Portal (#6691, ADR
// 0118 addendum): one registration as a PDF, and the registrations of a
// period as a ZIP with one PDF each, so the Träger-Büro keeps what the
// retention deletes. Both need the right "Anmeldung verwalten", stay with
// the session's Träger and stand in the audit trail. health=true adds the
// health information and needs the right "Gesundheitsangaben sehen", like
// the health read on the screen; without it a file names none.

// holidayHealthQuery is the query parameter that asks for the health
// information in a file.
const holidayHealthQuery = "health"

func holidayHealthAsked(r *http.Request) bool {
	return r.URL.Query().Get(holidayHealthQuery) == "true"
}

// requireHealthWhenAsked admits a request for a file with health
// information only with the right "Gesundheitsangaben sehen".
func (rs *HolidayPeriodResource) requireHealthWhenAsked(next http.Handler) http.Handler {
	guarded := rs.requireHealth(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if holidayHealthAsked(r) {
			guarded.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func holidayRecordNamesOf(r *http.Request) holidayRecordNames {
	schools, _ := r.Context().Value(holidaySchoolNamesKey{}).(map[int64]string)
	return holidayRecordNames{schools: schools, generatedAt: time.Now()}
}

// holidayRecordFile renders the file of one registration. Its name carries
// the number of the registration and the family name of the parent.
func holidayRecordFile(period capability.HolidayPeriod, record capability.HolidayRegistrationRecord, names holidayRecordNames) (lists.File, error) {
	base := "ferienanmeldung-" + strconv.FormatInt(record.Detail.ID, 10) + "-" + record.Detail.GuardianLastName
	return lists.NewRecordRenderer().RenderRecords(holidayRegistrationRecordDocument(period, record, names), base)
}

// downloadRegistrationPDF answers GET .../registrations/{registrationId}/pdf.
func (rs *HolidayPeriodResource) downloadRegistrationPDF(w http.ResponseWriter, r *http.Request) {
	reviewer, periodID, registrationID, ok := registrationKey(w, r)
	if !ok {
		return
	}
	set, err := rs.records.HolidayRegistrationRecord(r.Context(), reviewer, periodID, registrationID, holidayHealthAsked(r))
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	file, err := holidayRecordFile(set.Period, set.Records[0], holidayRecordNamesOf(r))
	if err != nil {
		renderHolidayReviewError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", file.Filename))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// heldResponse keeps what the read of an archive answers inside the Träger
// transaction, so the archive itself can be sent after the commit.
type heldResponse struct {
	bytes.Buffer
	header http.Header
	status int
}

func (h *heldResponse) Header() http.Header    { return h.header }
func (h *heldResponse) WriteHeader(status int) { h.status = status }

// readRegistrationRecords reads and records the registrations of the filter
// in the Träger transaction of the request and commits it. A refusal, also
// a failed commit, is answered to w; then ok is false.
func (rs *HolidayPeriodResource) readRegistrationRecords(withTx common.Middleware, w http.ResponseWriter, r *http.Request) (set capability.HolidayRegistrationRecordSet, ok bool) {
	held := &heldResponse{header: http.Header{}}
	withTx(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reviewer, authenticated := reviewerOf(w, r)
		if !authenticated {
			return
		}
		periodID, err := common.ParseIDParam(r, "id")
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		filter, err := registrationFilter(r)
		if err == nil {
			set, err = rs.records.HolidayRegistrationRecords(r.Context(), reviewer, periodID, filter, holidayHealthAsked(r))
		}
		if err != nil {
			renderHolidayReviewError(w, r, err)
			return
		}
		ok = true
	})).ServeHTTP(held, r)
	if ok && held.status < http.StatusBadRequest {
		return set, true
	}
	for key, values := range held.header {
		w.Header()[key] = values
	}
	w.WriteHeader(held.status)
	_, _ = w.Write(held.Bytes())
	return set, false
}

// downloadRegistrationPDFs answers GET .../registrations/pdf: a ZIP with one
// PDF per registration of the filter. The Träger transaction buffers its
// answer until the commit, so the route runs outside it: the registrations
// are read and recorded in the transaction first, and after its commit the
// archive is written to the response file by file. A large period neither
// waits for the whole archive nor holds it on the server, and the read
// stands in the audit trail even when the download breaks off.
func (rs *HolidayPeriodResource) downloadRegistrationPDFs(withTx common.Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		set, ok := rs.readRegistrationRecords(withTx, w, r)
		if !ok {
			return
		}
		names := holidayRecordNamesOf(r)
		// The first file is rendered before the response starts: a period
		// whose files cannot be rendered answers with an error, not with a
		// broken archive.
		first, err := holidayRecordEntry(set, 0, names)
		if err != nil {
			common.RenderError(w, r, common.ErrorInternalServer(err))
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", holidayArchiveName(set.Period)))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		if err := writeHolidayArchive(w, set, first, names); err != nil {
			// The status is sent; the connection is cut so the browser reports
			// a failed download instead of saving half an archive.
			slog.ErrorContext(r.Context(), "holiday registration archive failed",
				slog.Int64("holiday_period_id", set.Period.ID),
				slog.String("error", err.Error()))
			panic(http.ErrAbortHandler)
		}
	}
}

// holidayArchiveName names the archive after the period, in plain ASCII
// letters, digits and dashes.
func holidayArchiveName(period capability.HolidayPeriod) string {
	var name strings.Builder
	dash := true
	for _, r := range strings.ToLower(textnorm.ASCII(period.Name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			name.WriteRune(r)
			dash = false
		} else if !dash {
			name.WriteByte('-')
			dash = true
		}
	}
	return strings.TrimSuffix("ferienanmeldungen-"+name.String(), "-") + ".zip"
}

// holidayRecordEntry renders the file at index; nil without one.
func holidayRecordEntry(set capability.HolidayRegistrationRecordSet, index int, names holidayRecordNames) (*lists.File, error) {
	if index >= len(set.Records) {
		return nil, nil
	}
	file, err := holidayRecordFile(set.Period, set.Records[index], names)
	if err != nil {
		return nil, fmt.Errorf("render holiday registration %d: %w", set.Records[index].Detail.ID, err)
	}
	return &file, nil
}

// writeHolidayArchive writes one PDF per record into a ZIP on w, each
// rendered just before it is written. The names are plain ASCII, so every
// unpacker shows them alike, and carry the number of the registration, so
// two families of the same name never collide.
func writeHolidayArchive(w http.ResponseWriter, set capability.HolidayRegistrationRecordSet, first *lists.File, names holidayRecordNames) error {
	archive := zip.NewWriter(w)
	file := first
	for index := 0; file != nil; index++ {
		// The embedded fonts of a file deflate to about half its size.
		entry, err := archive.CreateHeader(&zip.FileHeader{Name: textnorm.ASCII(file.Filename), Method: zip.Deflate, Modified: names.generatedAt})
		if err != nil {
			return fmt.Errorf("add %s to the archive: %w", file.Filename, err)
		}
		if _, err := entry.Write(file.Data); err != nil {
			return fmt.Errorf("write %s to the archive: %w", file.Filename, err)
		}
		// Each file leaves at once; a writer that cannot flush still sends.
		_ = http.NewResponseController(w).Flush()
		if file, err = holidayRecordEntry(set, index+1, names); err != nil {
			return err
		}
	}
	return archive.Close()
}
