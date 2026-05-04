// Package rooms - students_present_handlers.go
//
// GET /api/rooms/{id}/students-present
//
// Lists the children currently checked-in to any active group taking place in
// the given room. The route is gated by `rooms:read` (matching every other
// /api/rooms/{id}/* read), and per-row PII redaction is layered on top via
// api/common.DetermineStudentAccess so the response respects the tenant's
// `gdpr.student_data_scope` setting.
package rooms

import (
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StudentsPresentInRoomResponse is the envelope returned by
// GET /api/rooms/{id}/students-present.
//
// TotalCount is the true number of students currently in the room (matches
// what a colleague at the kiosk sees). VisibleCount is how many of those
// rows the caller is authorised to see unredacted under the
// gdpr.student_data_scope setting. The frontend uses the gap between the
// two to render the "X von Y sichtbar" hint without needing a second call.
type StudentsPresentInRoomResponse struct {
	RoomID       int64                    `json:"room_id"`
	AsOf         time.Time                `json:"as_of"`
	TotalCount   int                      `json:"total_count"`
	VisibleCount int                      `json:"visible_count"`
	Students     []StudentPresentResponse `json:"students"`
}

// StudentPresentResponse is one row in StudentsPresentInRoomResponse.Students.
//
// Display fields are pointers so they serialise to JSON `null` when the
// caller is not authorised to read them. StudentID is always populated so
// the frontend can diff rows on SSE pushes without leaking PII for the
// redacted set.
type StudentPresentResponse struct {
	StudentID     int64      `json:"student_id"`
	FirstName     *string    `json:"first_name"`
	LastName      *string    `json:"last_name"`
	ActiveGroupID *int64     `json:"active_group_id"`
	GroupName     *string    `json:"group_name"`
	EntryTime     *time.Time `json:"entry_time"`
	HasFullAccess bool       `json:"has_full_access"`
}

// ListStudentsPresent handles GET /api/rooms/{id}/students-present.
//
// Exported so tests in package rooms_test can invoke the handler directly
// against a router without a thin *Handler() wrapper (see backend conventions
// rule 5).
func (rs *Resource) ListStudentsPresent(w http.ResponseWriter, r *http.Request) {
	roomID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidRoomID)))
		return
	}

	visits, err := rs.ActiveService.ListStudentsPresentInRoom(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	access := common.DetermineStudentAccess(r, rs.UserContextService, rs.SettingsService, rs.getLogger())

	students := make([]StudentPresentResponse, len(visits))
	visibleCount := 0
	for i, v := range visits {
		full := access.HasFullAccessByGroupID(v.EducationGroupID)
		if full {
			visibleCount++
		}
		students[i] = newStudentPresentResponse(v, full)
	}

	rs.getLogger().Info("students present in room listed",
		"room_id", roomID,
		"tenant_id", tenant.FromContext(r.Context()),
		"total_count", len(visits),
		"visible_count", visibleCount,
	)

	resp := StudentsPresentInRoomResponse{
		RoomID:       roomID,
		AsOf:         time.Now().UTC(),
		TotalCount:   len(visits),
		VisibleCount: visibleCount,
		Students:     students,
	}
	common.Respond(w, r, http.StatusOK, resp, "Students present in room retrieved successfully")
}

// newStudentPresentResponse builds the response row for a single visit,
// applying redaction when hasFullAccess is false.
func newStudentPresentResponse(v *active.StudentInRoomVisit, hasFullAccess bool) StudentPresentResponse {
	if !hasFullAccess {
		return StudentPresentResponse{
			StudentID:     v.StudentID,
			HasFullAccess: false,
		}
	}

	first := v.FirstName
	last := v.LastName
	activeGroupID := v.ActiveGroupID
	entryTime := v.EntryTime

	var groupName *string
	if v.ActivityName != "" {
		s := v.ActivityName
		groupName = &s
	}

	return StudentPresentResponse{
		StudentID:     v.StudentID,
		FirstName:     &first,
		LastName:      &last,
		ActiveGroupID: &activeGroupID,
		GroupName:     groupName,
		EntryTime:     &entryTime,
		HasFullAccess: true,
	}
}
