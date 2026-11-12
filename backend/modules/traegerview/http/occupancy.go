package traegerviewhttp

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/traegerview"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// occupancy answers GET /traeger/schools/occupancy: the children of every
// school of the Träger of the session by OGS group and care form, today
// (#4676). The answer holds counts and the names of groups and offers, no
// child, so the read is not written to the access log (ADR 0084 section 7).
func (rs *Resource) occupancy(w http.ResponseWriter, r *http.Request) {
	occupancy, ok := rs.readOccupancy(w, r)
	if !ok {
		return
	}
	common.Respond(w, r, http.StatusOK, occupancy, "Occupancy retrieved successfully")
}

// occupancyExport answers GET /traeger/schools/occupancy/export: the same
// figures as CSV, one row per school and figure, for planning in a
// spreadsheet.
func (rs *Resource) occupancyExport(w http.ResponseWriter, r *http.Request) {
	occupancy, ok := rs.readOccupancy(w, r)
	if !ok {
		return
	}
	data, err := occupancyCSV(occupancy)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", textnorm.ContentDisposition("attachment", fmt.Sprintf("belegung-%s.csv", occupancy.Day)))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (rs *Resource) readOccupancy(w http.ResponseWriter, r *http.Request) (traegerview.Occupancy, bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return traegerview.Occupancy{}, false
	}
	occupancy, err := rs.overview.Occupancy(r.Context(), principal.OrganizationID())
	switch {
	case errors.Is(err, traegerview.ErrTraegerUnavailable):
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return traegerview.Occupancy{}, false
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return traegerview.Occupancy{}, false
	}
	return occupancy, true
}

// The kinds of a row of the occupancy export.
const (
	occupancyRowTotal           = "Alle Kinder"
	occupancyRowGroup           = "Gruppe"
	occupancyRowWithoutGroup    = "Ohne Gruppe"
	occupancyRowCareForm        = "Betreuungsform"
	occupancyRowWithoutCareForm = "Ohne Betreuungsform"
)

// occupancyCSV writes one row per school and figure: the school, its town,
// the kind of the row, the name of the group or offer, the children and the
// places of the offer. A spreadsheet filters and sums it by kind.
func occupancyCSV(occupancy traegerview.Occupancy) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("\xEF\xBB\xBF")
	writer := csv.NewWriter(&buf)
	writer.Comma = ';'
	day := occupancy.Day.Format("02.01.2006")
	rows := [][]string{{"Datum", "Schule", "Ort", "Art", "Name", "Kinder", "Plätze"}}
	for _, school := range occupancy.Schools {
		row := func(kind, name string, children int, capacity *int) []string {
			places := ""
			if capacity != nil {
				places = strconv.Itoa(*capacity)
			}
			return []string{day, csvSafe(school.Name), csvSafe(school.City), kind, csvSafe(name), strconv.Itoa(children), places}
		}
		rows = append(rows, row(occupancyRowTotal, "", school.Children, nil))
		for _, group := range school.Groups {
			rows = append(rows, row(occupancyRowGroup, group.Name, group.Children, nil))
		}
		rows = append(rows, row(occupancyRowWithoutGroup, "", school.WithoutGroup, nil))
		for _, form := range school.CareForms {
			rows = append(rows, row(occupancyRowCareForm, form.Name, form.Children, form.Capacity))
		}
		rows = append(rows, row(occupancyRowWithoutCareForm, "", school.WithoutCareForm, nil))
	}
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
