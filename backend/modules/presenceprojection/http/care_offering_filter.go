package presenceprojectionhttp

import (
	context "context"
	errors "errors"
	http "net/http"
	strconv "strconv"

	common "github.com/moto-nrw/project-phoenix/api/common"
)

// errCareOfferingFilter refuses a care_offering_id that is no positive id or
// that comes without location_state=present. Ignoring it would silently
// widen the list to every present child.
var errCareOfferingFilter = errors.New("care_offering_id must be a positive id and requires location_state=present")

// parseCareOfferingFilter reads the care_offering_id filter (#4571). It
// narrows the present children only: the booking answers who belongs to the
// offering, presence who is here to pick.
func parseCareOfferingFilter(r *http.Request, params *studentListParams) error {
	raw := r.URL.Query().Get("care_offering_id")
	if raw == "" {
		return nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 || params.locationState != "present" {
		return errCareOfferingFilter
	}
	params.careOfferingID = id
	return nil
}

// resolveCareOfferingFilter keeps the pre-filtered children Care Plan counts
// to the offering on the planning date. It reports nonEmpty=false when none
// is left.
func (rs *Resource) resolveCareOfferingFilter(ctx context.Context, params *studentListParams) (bool, error) {
	if params.careOfferingID == 0 {
		return true, nil
	}
	if rs.OfferingDayRoster == nil {
		return false, errors.New("student list: offering day roster is not configured")
	}
	ids, err := rs.OfferingDayRoster.ChildrenOnOfferingDay(ctx, params.careOfferingID, params.studentIDs, params.careOfferingDate)
	if err != nil {
		return false, err
	}
	params.studentIDs = ids
	return len(ids) > 0, nil
}

// CareOfferingOptionResponse is one entry of the care_offering_id filter's
// selection field.
type CareOfferingOptionResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// listCareOfferingOptions answers the active care offerings that count as
// care, for the care_offering_id filter. It needs only users:read, like the
// list it filters; the offering catalog itself stays behind config:read.
func (rs *Resource) listCareOfferingOptions(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingDayRoster == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering day roster is not configured")))
		return
	}
	offerings, err := rs.OfferingDayRoster.ActiveCareOfferings(r.Context())
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := make([]CareOfferingOptionResponse, 0, len(offerings))
	for _, offering := range offerings {
		out = append(out, CareOfferingOptionResponse{ID: offering.ID, Name: offering.Name})
	}
	common.Respond(w, r, http.StatusOK, out, "Care offerings retrieved successfully")
}
