package students

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// The Gesundheitsliste (#3323) is the child list of stored health notes. It
// shares the student export's filters, sort and renderer; what sets it apart
// lives here: which children it carries, the health cell, and the audit
// record every export of Art. 9 data leaves behind.

// healthListOnlyWithNoteLabel names the default scope on the document, so a
// printed list without a child is not read as "this child has no allergies".
const healthListOnlyWithNoteLabel = "Nur Kinder mit hinterlegten Gesundheitsinformationen"

// healthInfoMissingLabel is the cell of a child without a note, worded like
// the Notfallliste's.
const healthInfoMissingLabel = "Nicht hinterlegt"

// hasHealthInfo reports whether a child has a health note worth printing. A
// whitespace-only note counts as none, the same rule the cell applies.
func hasHealthInfo(student StudentResponse) bool {
	return strings.TrimSpace(student.HealthInfo) != ""
}

// finalizeExportSources runs the checks that need the final row set. Every
// export is held to the document cap. The Gesundheitsliste then gets its
// health cells and its audit record; without the record there is no file.
func (rs *Resource) finalizeExportSources(r *http.Request, req studentExportRequest, responses []StudentResponse, sources []exportRowSource) render.Renderer {
	if errResp := exportSelectionCapError(len(sources)); errResp != nil {
		return errResp
	}
	if req.Preset != lists.PresetHealthList {
		return nil
	}
	if err := addHealthInfoCells(sources, responses); err != nil {
		return common.ErrorInternalServer(err)
	}
	if err := rs.writeHealthListAudit(r, req); err != nil {
		return common.ErrorInternalServerWrap("failed to record audit trail", err)
	}
	return nil
}

// addHealthInfoCells writes each child's health note into its row. The rows
// were built from the responses in the same order, and the Gesundheitsliste
// never merges class-list entries, so the two line up one to one; anything
// else is refused rather than printing a note next to the wrong child.
func addHealthInfoCells(sources []exportRowSource, responses []StudentResponse) error {
	if len(sources) != len(responses) {
		return errors.New("health list rows do not match the exported children")
	}
	for i := range sources {
		sources[i].row.Values[lists.ColumnHealthInfo] = healthInfoExportCell(responses[i].HealthInfo)
	}
	return nil
}

// healthInfoExportCell renders one child's note. The renderer's style markers
// are stripped because the note is parent-written text. Without a note the
// cell says so in words, as the Notfallliste does (#2609): an empty cell reads
// as "no allergies", which is the one reading that could get a child hurt.
func healthInfoExportCell(note string) string {
	note = lists.SanitizeUserText(note)
	if strings.TrimSpace(note) == "" {
		return healthInfoMissingLabel
	}
	return note
}

// writeHealthListAudit records the export in audit.data_access_log. Like the
// day log and the absence overview: no audit record, no data.
func (rs *Resource) writeHealthListAudit(r *http.Request, req studentExportRequest) error {
	logger := rs.getLogger()
	if rs.StudentHistoryService == nil {
		logger.Error("audit log repo not configured, refusing to export health list")
		return errors.New("audit log repository not configured")
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	actorRole := strings.Join(claims.Roles, ",")
	if actorRole == "" {
		actorRole = "unknown"
	}
	today := timezone.TodayDate()
	entry := &studentpresence.DataAccessEvent{
		ActorAccountID: int64(claims.ID),
		ActorRole:      actorRole,
		ResourceType:   dataAccessStudentHealthListExport,
		RangeStart:     today.BerlinMidnight(),
		RangeEnd:       today.EndOfDay(),
		AccessedAt:     time.Now(),
		Scope: &studentpresence.DataAccessScope{
			GroupIDs: parseGroupIDList([]string{req.Filters.GroupID}),
			Date:     today.String(),
		},
	}
	if err := rs.StudentHistoryService.RecordDataAccess(r.Context(), entry); err != nil {
		logger.Error("audit log write failed, refusing to export health list",
			slog.String("resource_type", dataAccessStudentHealthListExport),
			slog.String("error", err.Error()),
		)
		return err
	}
	return nil
}

// exportDocumentFilterLabels lists the filters printed on the document. The
// Gesundheitsliste names its default scope as well, because a missing child
// on it only means "no note stored" when the page says so.
func exportDocumentFilterLabels(req studentExportRequest, planningDate timezone.Date, isToday bool) []string {
	labels := exportFilterLabelsForDate(req.Filters, planningDate, isToday)
	if req.Preset == lists.PresetHealthList && !req.Filters.IncludeWithoutHealthInfo {
		labels = append(labels, healthListOnlyWithNoteLabel)
	}
	return labels
}
