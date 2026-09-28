package students

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// deleteStudent permanently deletes an active child through the student
// deletion workflow (#2710). The preview and the explicit confirmation are
// mandatory even when the child currently has no dependent rows: otherwise an
// old or hand-written client could bypass the typed name, acknowledgement and
// audit trail simply by sending an empty DELETE.
func (rs *Resource) deleteStudent(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	authorized, authErr := canDeleteStudent(r.Context(), jwt.PermissionsFromCtx(r.Context()), student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}
	if rs.StudentDeletion == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student deletion workflow not configured")))
		return
	}
	if r.ContentLength == 0 {
		renderError(w, r, common.ErrorConflictMessage("Bitte die Löschvorschau prüfen und die endgültige Löschung bestätigen."))
		return
	}
	body := new(studentDeleteRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, err := rs.StudentDeletion.Execute(r.Context(), student.ID, body.confirmation()); err != nil {
		// The route middleware owns the ambient transaction and commits on
		// every non-5xx response, so the 4xx paths request the rollback here.
		tenant.MarkRollback(r.Context())
		renderError(w, r, studentDeletionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Student and linked data deleted successfully")
}

type studentDeleteImpactResponse struct {
	ConfirmationName string                             `json:"confirmation_name"`
	Fingerprint      string                             `json:"fingerprint"`
	Total            int                                `json:"total"`
	Counts           studentdeletion.Counts             `json:"counts"`
	Preserved        studentDeletePreservedDataResponse `json:"preserved"`
}

type studentDeletePreservedDataResponse struct {
	GuardianProfiles bool `json:"guardian_profiles"`
	ParentAccounts   bool `json:"parent_accounts"`
	OtherStudents    bool `json:"other_students"`
	SharedInstances  bool `json:"shared_instances"`
}

func toStudentDeleteImpactResponse(impact studentdeletion.Preview) studentDeleteImpactResponse {
	return studentDeleteImpactResponse{
		ConfirmationName: impact.ConfirmationName,
		Fingerprint:      impact.Fingerprint,
		Total:            impact.Counts.Total(),
		Counts:           impact.Counts,
		Preserved: studentDeletePreservedDataResponse{
			GuardianProfiles: true,
			ParentAccounts:   true,
			OtherStudents:    true,
			SharedInstances:  true,
		},
	}
}

func (rs *Resource) getStudentDeleteImpact(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	authorized, authErr := canDeleteStudent(r.Context(), jwt.PermissionsFromCtx(r.Context()), student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}
	if rs.StudentDeletion == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student deletion workflow not configured")))
		return
	}
	impact, err := rs.StudentDeletion.Preview(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, studentDeletionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toStudentDeleteImpactResponse(impact), "Student deletion impact retrieved")
}

type studentDeleteRequest struct {
	ExpectedFingerprint string `json:"expected_fingerprint"`
	ConfirmationName    string `json:"confirmation_name"`
	Reason              string `json:"reason"`
	Acknowledged        bool   `json:"acknowledged"`
}

func (req *studentDeleteRequest) Bind(_ *http.Request) error {
	if strings.TrimSpace(req.ExpectedFingerprint) == "" {
		return errors.New("expected_fingerprint is required")
	}
	if strings.TrimSpace(req.ConfirmationName) == "" {
		return errors.New("confirmation_name is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return errors.New("reason is required")
	}
	if !req.Acknowledged {
		return studentdeletion.ErrNotAcknowledged
	}
	return nil
}

func (req *studentDeleteRequest) confirmation() studentdeletion.Confirmation {
	return studentdeletion.Confirmation{
		ExpectedFingerprint: req.ExpectedFingerprint,
		ConfirmationName:    req.ConfirmationName,
		Reason:              req.Reason,
		Acknowledged:        req.Acknowledged,
	}
}

// purgeGraduatedStudent hard-deletes a child that a grade transition graduated.
//
// Graduation is a soft delete: the child disappears from every staff list and
// every per-student route answers 404, which leaves exactly one gap — a school
// that wants a departed child's data actually gone (retention, a parent's
// erasure request) has no way to do it, because the very gate that hides the
// child also blocks the delete. This route is that way, reachable only from the
// Abgänge view of an applied transition.
//
// It is deliberately NOT a flag on deleteStudent: a separate route means the
// alumnus exception is one grep away, and an ordinary delete can never acquire
// it by accident through a stray query parameter. The workflow re-decides the
// alumnus question under the row lock, so a child restored by a concurrent
// revert is never deleted on the strength of a stale list.
func (rs *Resource) purgeGraduatedStudent(w http.ResponseWriter, r *http.Request) {
	// The alumnus-blind lookup: the gate the ordinary path relies on is the
	// thing this route exists to bypass.
	student, ok := rs.parseAndGetStudentIncludingAlumni(w, r)
	if !ok {
		return
	}
	// Only graduates. An active child must go through deleteStudent, which is
	// where the visible-student authorization and UX live.
	if !student.IsAlumnus() {
		renderError(w, r, common.ErrorConflictMessage(
			"Nur Abgänger können endgültig gelöscht werden. Aktive Kinder werden unter „Alle Kinder“ gelöscht."))
		return
	}
	authorized, authErr := canDeleteStudent(r.Context(), jwt.PermissionsFromCtx(r.Context()), student, rs.UserContextService)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return
	}
	if rs.StudentDeletion == nil {
		renderError(w, r, common.ErrorInternalServer(
			errors.New("student deletion workflow not configured, refusing to purge")))
		return
	}
	if _, err := rs.StudentDeletion.PurgeGraduate(r.Context(), student.ID); err != nil {
		tenant.MarkRollback(r.Context())
		renderError(w, r, purgeGraduatedStudentErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Graduated student permanently deleted")
}

// purgeGraduatedStudentErrorRenderer maps the purge workflow error to the wire
// response. It reuses the ordinary delete's classification and adds the one
// purge-specific case: the child was restored between the Abgänge list and
// the locked re-read, so the list is stale rather than the request malformed.
func purgeGraduatedStudentErrorRenderer(err error) render.Renderer {
	if errors.Is(err, studentdeletion.ErrNotGraduated) {
		return common.ErrorConflictMessage(err.Error())
	}
	return studentDeletionErrorRenderer(err)
}

var studentDeletionErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	// The child graduated while this request was in flight. Answered with the
	// same 404 the shared alumnus gate returns, so a delete never depends on
	// which of the two transactions won the race.
	{Target: studentdeletion.ErrGraduatedUnderLock, Render: func(error) render.Renderer {
		return common.ErrorNotFound(errors.New("student not found"))
	}},
	{Target: studentdeletion.ErrStudentNotFound, Render: func(error) render.Renderer {
		return common.ErrorNotFound(errors.New("student not found"))
	}},
	{Target: studentdeletion.ErrUnauthorized, Render: func(err error) render.Renderer {
		return common.ErrorForbidden(err)
	}},
	{Target: studentdeletion.ErrPreviewChanged, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, errCodeStudentDeletionPreviewChanged)
	}},
	{Target: studentdeletion.ErrConfirmationMismatch, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, errCodeStudentDeletionConfirmationMismatch)
	}},
	{Target: studentdeletion.ErrNotAcknowledged, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, errCodeStudentDeletionAcknowledgement)
	}},
	{Target: studentdeletion.ErrInvalidReason, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, errCodeStudentDeletionInvalidReason)
	}},
	{Target: studentdeletion.ErrAlumnus, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, errCodeStudentDeletionAlumnus)
	}},
	{Target: studentdeletion.ErrRetentionNotEnded, Render: func(err error) render.Renderer {
		return common.ErrorInvalidRequestWithCode(err, errCodeStudentDeletionRetentionNotEnded)
	}},
	{Target: studentdeletion.ErrCompanionWouldLoseDeparture, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, errCodeStudentDeletionCompanionBlocked)
	}},
	{Target: studentdeletion.ErrCompanionLockBusy, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, errCodeStudentDeletionCompanionLockBusy)
	}},
	{Target: studentdeletion.ErrWithdrawalNotFound, Render: func(err error) render.Renderer {
		return common.ErrorNotFoundWithCode(err, errCodeCareWithdrawalNotFound)
	}},
	{Target: studentdeletion.ErrWithdrawalAlreadyResolved, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, errCodeCareWithdrawalAlreadyResolved)
	}},
	{Match: common.IsConstraintViolation, Render: func(error) render.Renderer {
		return common.ErrorConflictWithCode(
			//nolint:staticcheck // ST1005: user-facing German message
			errors.New("Kind konnte wegen gleichzeitig geänderter Verknüpfungen nicht gelöscht werden. Bitte erneut prüfen."),
			errCodeStudentDeletionConstraintsChanged,
		)
	}},
}, common.ErrorInternalServer)
