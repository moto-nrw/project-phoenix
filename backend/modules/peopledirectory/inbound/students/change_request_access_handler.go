package students

import (
	"errors"
	"net/http"
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// changeRequestAccessResponse is the caller's parent-request capability. With
// ?student_id it also says whether the review scopes reach that child.
type changeRequestAccessResponse struct {
	ReviewAccess string                     `json:"review_access"`
	Student      *studentReviewCoverageBody `json:"student,omitempty"`
}

// studentReviewCoverageBody tells a message thread which request pills of one
// child may open their detail (#3886): requests covers every request kind,
// absences the sick and excused requests, whose scope a school may set apart.
type studentReviewCoverageBody struct {
	Requests bool `json:"requests"`
	Absences bool `json:"absences"`
}

// changeRequestAccess reports the caller's effective parent-request
// capability for the shared navigation. The retained review policy answers
// it; an unwired policy is a configuration error.
func (rs *Resource) changeRequestAccess(w http.ResponseWriter, r *http.Request) {
	if rs.RequestReviewAccess == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("parent request review policy is not configured")))
		return
	}
	permissions := jwt.PermissionsFromCtx(r.Context())
	access, err := rs.RequestReviewAccess.AccessLevel(r.Context(), permissions)
	if err != nil {
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return
	}
	if access == "" {
		renderError(w, r, common.ErrorInternalServer(errors.New("parent request review policy is not configured")))
		return
	}
	response := changeRequestAccessResponse{ReviewAccess: access}
	if raw := r.URL.Query().Get("student_id"); raw != "" {
		coverage, ok := rs.studentReviewCoverage(w, r, raw, access, permissions)
		if !ok {
			return
		}
		response.Student = coverage
	}
	common.Respond(w, r, http.StatusOK, response, "Change request access retrieved")
}

// studentReviewCoverage resolves whether the review scopes reach one child.
// A caller without any review scope gets no for both without a student
// lookup, so the authenticated-only route never tells them whether an ID
// exists.
func (rs *Resource) studentReviewCoverage(
	w http.ResponseWriter,
	r *http.Request,
	raw string,
	access string,
	permissions []string,
) (*studentReviewCoverageBody, bool) {
	studentID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || studentID <= 0 {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return nil, false
	}
	if access == "none" {
		return &studentReviewCoverageBody{}, true
	}
	student, err := rs.findStudent(r.Context(), studentID)
	if err != nil {
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}
	schoolWide, groupIDs, err := rs.RequestReviewAccess.Scope(r.Context(), permissions)
	if err != nil {
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return nil, false
	}
	absenceWide, absenceGroupIDs, err := rs.RequestReviewAccess.AbsenceScope(r.Context(), permissions)
	if err != nil {
		renderError(w, r, parentRequestQueueErrorRenderer(err))
		return nil, false
	}
	return &studentReviewCoverageBody{
		Requests: reviewScopeCovers(schoolWide, groupIDs, student.GroupID),
		Absences: reviewScopeCovers(absenceWide, absenceGroupIDs, student.GroupID),
	}, true
}

// reviewScopeCovers applies a review scope to one child the way the request
// queues and the detail routes do: a school-wide scope covers every child, a
// group scope only children of its groups.
func reviewScopeCovers(schoolWide bool, groupIDs []int64, groupID *int64) bool {
	return schoolWide || (groupID != nil && slices.Contains(groupIDs, *groupID))
}
