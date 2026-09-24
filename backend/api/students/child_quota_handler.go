package students

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// childQuotaResponse carries the Kinderkontingent line of the Datenverwaltung
// (#3569). The two numbers use the names of the 409 refusal details
// (students.child_quota_reached), so the client reads both the same way, and
// are left out when the school has no Kinderkontingent.
type childQuotaResponse struct {
	Limited        bool `json:"limited"`
	BookedPlaces   *int `json:"booked_places,omitempty"`
	OccupiedPlaces *int `json:"occupied_places,omitempty"`
}

func (rs *Resource) getChildQuota(w http.ResponseWriter, r *http.Request) {
	if rs.ChildQuota == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("child quota reader not configured")))
		return
	}
	usage, limited, err := rs.ChildQuota.ChildQuotaUsage(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	response := childQuotaResponse{Limited: limited}
	if limited {
		response.BookedPlaces, response.OccupiedPlaces = &usage.Booked, &usage.Occupied
	}
	common.Respond(w, r, http.StatusOK, response, "Child quota retrieved")
}
