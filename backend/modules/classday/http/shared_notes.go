package classdayhttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// The notes the OGS released for the school (#4797): the Lehrkraft of the
// child's class reads them in moto schule, and only reads them. The class
// assignment comes from education.class_teachers, never from
// class_day:all_classes; the capability checks it, the child's class and
// writes the access-log row.

var sharedNotesErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: classday.ErrClassNotAssigned, Render: common.ErrorForbidden},
	{Target: classday.ErrSharedNoteStudentNotInClass, Render: common.ErrorNotFound},
}, common.ErrorInternalServer)

// getSharedNotes handles GET /school/class-day/shared-notes/{schoolClass}/students/{studentId}.
func (rs *Resource) getSharedNotes(w http.ResponseWriter, r *http.Request) {
	if rs.SharedNotes == nil {
		common.RenderError(w, r, common.ErrorInternalServer(classday.ErrSharedNotesNotConfigured))
		return
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "studentId", "ungültige Kind-ID")
	if !ok {
		return
	}
	view, err := rs.SharedNotes.StudentSharedNotes(r.Context(), classday.SharedNotesRequest{
		SchoolClass: chi.URLParam(r, "schoolClass"), StudentID: studentID,
		Actor: classday.Actor{AccountID: int64(claims.ID), Roles: strings.Join(claims.Roles, ",")},
	})
	if err != nil {
		common.RenderError(w, r, sharedNotesErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, view, "Shared notes retrieved successfully")
}
