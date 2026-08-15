package parent

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// TodayStatusResponse ist die vollstaendige Elternsicht auf den Betreuungstag.
// Die vier Felder sind bewusst die einzigen: jedes weitere waere ein Leck aus
// der internen Anwesenheitserfassung. Ein Wire-Format-Test pinnt das.
//
// AtOgs ist die erste Anzeigeebene, die Ja/Nein-Aussage "in der OGS". null
// bedeutet, dass wir keine belastbare Aussage treffen koennen; die Oberflaeche
// laesst die Ebene dann weg, statt zu raten. State ist die zweite Ebene und
// erklaert das Wann und Warum.
type TodayStatusResponse struct {
	AtOgs        *bool  `json:"at_ogs"`
	State        string `json:"state"`
	Since        string `json:"since,omitempty"`
	Until        string `json:"until,omitempty"`
	ExpectedFrom string `json:"expected_from,omitempty"`
}

// getChildTodayStatus liefert den reduzierten Betreuungsstatus des laufenden
// Berliner Kalendertages fuer ein verknuepftes Kind.
func (rs *Resource) getChildTodayStatus(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	status, err := rs.ParentService.GetChildTodayStatus(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, TodayStatusResponse{
		AtOgs:        status.AtOgs,
		State:        string(status.State),
		Since:        status.Since,
		Until:        status.Until,
		ExpectedFrom: status.ExpectedFrom,
	}, "Today status retrieved")
}
