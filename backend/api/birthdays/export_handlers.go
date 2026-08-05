package birthdays

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

type staffExportRequest struct {
	Format listexport.Format `json:"format"`
	Title  string            `json:"title"`
	// Months restricts the list to the given birth months ("01".."12"), empty
	// means the whole year. Same wire shape as the child birthday list so the
	// frontend month picker is shared between them.
	Months []string `json:"months"`
}

const staffBirthdayExportTitle = "Geburtstagsliste Personal"

// exportStaffBirthdays renders the staff Geburtstagsliste (#1542).
//
// The document deliberately carries no age column. The child list has one
// because a group celebrates a child turning seven; a colleague's age is
// nobody's business, and the birth date itself is already visible only to the
// roles that can open the Stammdaten it comes from.
func (rs *Resource) exportStaffBirthdays(w http.ResponseWriter, r *http.Request) {
	if rs.ListExportService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("list export service is not configured")))
		return
	}

	var req staffExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	months, err := parseExportMonths(req.Months)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	entries, err := rs.BirthdayService.ListStaffBirthdays(r.Context(), months)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	title := req.Title
	if title == "" {
		title = staffBirthdayExportTitle
	}

	doc := listexport.Document{
		Title:       title,
		Subtitle:    staffExportSubtitle(len(entries)),
		GeneratedAt: time.Now(),
		Filters:     monthFilterLabels(req.Months),
		Columns:     listexport.ResolveColumns(nil, listexport.PresetStaffBirthdayList),
		Rows:        buildStaffBirthdayRows(entries),
	}

	file, err := rs.ListExportService.Render(doc, req.Format, doc.Title)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}

// parseExportMonths turns the wire month filter ("01".."12") into a lookup
// set. An empty filter means every month; an unparseable one is a client
// mistake and is refused rather than silently widened to the whole year.
func parseExportMonths(values []string) (map[time.Month]bool, error) {
	if len(values) == 0 {
		return nil, nil
	}
	months := make(map[time.Month]bool, len(values))
	for _, value := range values {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > 12 {
			return nil, fmt.Errorf("invalid birthday month %q, expected \"01\" to \"12\"", value)
		}
		months[time.Month(parsed)] = true
	}
	return months, nil
}

func buildStaffBirthdayRows(entries []userModels.BirthdayEntry) []listexport.Row {
	rows := make([]listexport.Row, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, listexport.Row{
			Values: map[listexport.ColumnID]string{
				listexport.ColumnName:     entry.FullName(),
				listexport.ColumnBirthday: entry.Birthday.Format("02.01.2006"),
			},
		})
	}
	return rows
}

func staffExportSubtitle(count int) string {
	if count == 1 {
		return "1 Person"
	}
	return fmt.Sprintf("%d Personen", count)
}

var germanMonthNames = [12]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

// monthFilterLabels documents the selected months in the document header, so a
// printed list says which part of the year it covers. Months are named
// chronologically regardless of the order they arrived in, matching the order
// of the list below the header — same wording as the child birthday list.
func monthFilterLabels(values []string) []string {
	months, err := parseExportMonths(values)
	if err != nil || len(months) == 0 {
		return nil
	}
	names := make([]string, 0, len(months))
	for month := time.January; month <= time.December; month++ {
		if months[month] {
			names = append(names, germanMonthNames[month-1])
		}
	}
	if len(names) == 1 {
		return []string{"Geburtsmonat: " + names[0]}
	}
	return []string{"Geburtsmonate: " + strings.Join(names, ", ")}
}
