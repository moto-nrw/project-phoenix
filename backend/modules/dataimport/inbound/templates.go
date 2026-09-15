package importapi

import (
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
)

func (rs *Resource) downloadTemplate(w http.ResponseWriter, r *http.Request, kind dataimport.TemplateKind, filename string) {
	format := dataimport.CSV
	contentType := "text/csv; charset=utf-8"
	if r.URL.Query().Get("format") == string(dataimport.XLSX) {
		format = dataimport.XLSX
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename="+filename+"."+string(format))
	contents, err := rs.files.Template(kind, format)
	if err != nil {
		slog.Default().Error("Error creating import template", slog.String("error", err.Error()))
		http.Error(w, errTemplateCreation, http.StatusInternalServerError)
		return
	}
	if _, err := w.Write(contents); err != nil {
		slog.Default().Error("Error writing import template", slog.String("error", err.Error()))
	}
}
