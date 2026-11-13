package careplanhttp

import (
	json "encoding/json"
	errors "errors"
	http "net/http"
	strconv "strconv"
	time "time"

	common "github.com/moto-ogs/project-phoenix/api/common"
	careplan "github.com/moto-ogs/project-phoenix/modules/careplan"
	sessiontoken "github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
)

// pickupChangeNoticeResponse is one unread marker of a changed pickup time
// (#6855). It names the child by first name only; the link opens the child.
type pickupChangeNoticeResponse struct {
	RequestID        string    `json:"request_id"`
	StudentID        string    `json:"student_id"`
	StudentFirstName string    `json:"student_first_name"`
	Date             string    `json:"date"`
	PickupTime       string    `json:"pickup_time,omitempty"`
	Withdrawn        bool      `json:"withdrawn"`
	InformedAt       time.Time `json:"informed_at"`
}

var errPickupChangeNoticesUnbound = errors.New("pickup change notices are not configured")

// listPickupChangeNotices serves the reader's unread markers for the header.
// They reach only the reader they were stored for.
func (rs *Resource) listPickupChangeNotices(w http.ResponseWriter, r *http.Request) {
	if rs.PickupChangeNotices == nil {
		renderError(w, r, common.ErrorInternalServer(errPickupChangeNoticesUnbound))
		return
	}
	notices, err := rs.PickupChangeNotices.ListUnreadPickupChangeNotices(r.Context(), int64(sessiontoken.ClaimsFromCtx(r.Context()).ID))
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load pickup change notices", err))
		return
	}
	common.Respond(w, r, http.StatusOK, renderPickupChangeNotices(notices), "Pickup change notices retrieved")
}

// readPickupChangeNotices clears the markers the reader loaded for one child
// once the child is open, and answers with the markers still unread. A marker
// told again while the page loaded keeps its unread state.
func (rs *Resource) readPickupChangeNotices(w http.ResponseWriter, r *http.Request) {
	if rs.PickupChangeNotices == nil {
		renderError(w, r, common.ErrorInternalServer(errPickupChangeNoticesUnbound))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid student id")
	if !ok {
		return
	}
	var body struct {
		Notices []struct {
			RequestID  int64     `json:"request_id,string"`
			InformedAt time.Time `json:"informed_at"`
		} `json:"notices"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New("invalid pickup change notice revisions")))
		return
	}
	observed := make([]careplan.PickupChangeNoticeRevision, 0, len(body.Notices))
	for _, notice := range body.Notices {
		observed = append(observed, careplan.PickupChangeNoticeRevision{RequestID: notice.RequestID, InformedAt: notice.InformedAt})
	}
	notices, err := rs.PickupChangeNotices.MarkPickupChangeNoticesRead(r.Context(), int64(sessiontoken.ClaimsFromCtx(r.Context()).ID), studentID, observed)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to mark pickup change notices read", err))
		return
	}
	common.Respond(w, r, http.StatusOK, renderPickupChangeNotices(notices), "Pickup change notices read")
}

func renderPickupChangeNotices(notices []careplan.PickupChangeTeamNotice) []pickupChangeNoticeResponse {
	out := make([]pickupChangeNoticeResponse, 0, len(notices))
	for _, notice := range notices {
		out = append(out, pickupChangeNoticeResponse{
			RequestID: strconv.FormatInt(notice.RequestID, 10), StudentID: strconv.FormatInt(notice.StudentID, 10),
			StudentFirstName: notice.StudentFirstName, Date: notice.Date.String(), PickupTime: notice.PickupTime,
			Withdrawn: notice.Withdrawn, InformedAt: notice.InformedAt,
		})
	}
	return out
}
