package studentdeletionhttp

import (
	errors "errors"
	http "net/http"

	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

func (rs *Resource) previewWithdrawalDeletion(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "completionId", "invalid completion ID")
	if !ok {
		return
	}
	if _, ok := rs.authorizeWithdrawalDeletion(w, r, id); !ok {
		return
	}
	if rs.StudentDeletion == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student deletion workflow not configured")))
		return
	}
	impact, err := rs.StudentDeletion.PreviewWithdrawal(r.Context(), id)
	if err != nil {
		renderError(w, r, rs.withdrawalDeletionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toStudentDeleteImpactResponse(impact), "Auswirkungen der Löschung geladen")
}

func (rs *Resource) deleteWithdrawalStudent(w http.ResponseWriter, r *http.Request) {
	if rs.CareLifecycleService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care lifecycle service not configured")))
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "completionId", "invalid completion ID")
	if !ok {
		return
	}
	body := new(studentDeleteRequest)
	if err := render.Bind(r, body); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if _, ok := rs.authorizeWithdrawalDeletion(w, r, id); !ok {
		return
	}
	if rs.StudentDeletion == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student deletion workflow not configured")))
		return
	}
	if _, err := rs.StudentDeletion.ExecuteWithdrawal(r.Context(), id, body.confirmation()); err != nil {
		// The route middleware owns the ambient transaction. The workflow
		// cannot roll it back itself when the delete re-check fails.
		tenant.MarkRollback(r.Context())
		renderError(w, r, rs.withdrawalDeletionErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Kind und verknüpfte Daten gelöscht")
}

func (rs *Resource) authorizeWithdrawalDeletion(
	w http.ResponseWriter,
	r *http.Request,
	completionID int64,
) (careplan.WithdrawalCompletion, bool) {
	completion, err := rs.CareLifecycleService.GetPendingWithdrawal(r.Context(), completionID)
	if err != nil {
		renderError(w, r, rs.withdrawalDeletionErrorRenderer(err))
		return careplan.WithdrawalCompletion{}, false
	}
	students, err := rs.studentsByIDs(r.Context(), []int64{*completion.StudentID})
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return careplan.WithdrawalCompletion{}, false
	}
	student := students[*completion.StudentID]
	if student == nil {
		renderError(w, r, common.ErrorNotFound(careplan.ErrCareWithdrawalNotFound))
		return careplan.WithdrawalCompletion{}, false
	}
	authorized, authErr := canDeleteStudent(
		r.Context(), sessiontoken.PermissionsFromCtx(r.Context()), student, rs.UserContextService,
	)
	if !authorized {
		renderError(w, r, common.ErrorForbidden(authErr))
		return careplan.WithdrawalCompletion{}, false
	}
	return completion, true
}

func (rs *Resource) withdrawalDeletionErrorRenderer(err error) render.Renderer {
	if errors.Is(err, careplan.ErrCareWithdrawalNotFound) {
		return common.ErrorNotFoundWithCode(err, common.CodeStudentsCareWithdrawalNotFound)
	}
	if errors.Is(err, careplan.ErrCareWithdrawalAlreadyResolved) {
		return common.ErrorConflictWithCode(err, common.CodeStudentsCareWithdrawalAlreadyResolved)
	}
	return rs.studentDeletionErrorRenderer(err)
}
