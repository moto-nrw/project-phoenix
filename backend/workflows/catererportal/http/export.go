package catererportalhttp

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

var errExportFormat = errors.New("format must be pdf, xlsx or csv")

// exportWindow reads the window and the format of a delivery list.
func exportWindow(r *http.Request) (from, to mealplan.Date, format string, err error) {
	from, fromErr := mealplan.ParseDate(r.URL.Query().Get("from"))
	to, toErr := mealplan.ParseDate(r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		return "", "", "", errors.New("from and to must be in YYYY-MM-DD format")
	}
	format = r.URL.Query().Get("format")
	if format != "pdf" && format != "xlsx" && format != "csv" {
		return "", "", "", errExportFormat
	}
	return from, to, format, nil
}

// exportSchoolDeliveries downloads the school's own delivery list (#3873).
func (rs *Resource) exportSchoolDeliveries(w http.ResponseWriter, r *http.Request) {
	from, to, format, err := exportWindow(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	export, err := rs.portal.SchoolDeliveryExport(r.Context(), from, to)
	rs.writeExport(w, r, export, format, err)
}

// exportDeliveries downloads the caterer's delivery list of every school it
// supplies (#3873).
func (rs *Resource) exportDeliveries(w http.ResponseWriter, r *http.Request) {
	from, to, format, err := exportWindow(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	export, err := rs.portal.DeliveryExport(r.Context(), rs.currentAccountID(r.Context()), from, to)
	rs.writeExport(w, r, export, format, err)
}

func (rs *Resource) writeExport(w http.ResponseWriter, r *http.Request, export mealplan.ListExport, format string, err error) {
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	var file ExportFile
	if format == "csv" {
		data, csvErr := export.CSV()
		file, err = ExportFile{Data: data, ContentType: "text/csv; charset=utf-8", Filename: export.Filename + ".csv"}, csvErr
	} else {
		file, err = rs.runtime.RenderList(export, format)
	}
	if err != nil {
		renderFailure(w, r, err)
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Data)
}
