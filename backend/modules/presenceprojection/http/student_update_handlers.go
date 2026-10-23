package presenceprojectionhttp

import (
	http "net/http"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// respondUpdatedStudent re-reads the student and writes the 200 response,
// stamping the write's companion verdict onto it (see
// StudentResponse.CompanionsChanged).
func (rs *Resource) RespondUpdatedStudent(w http.ResponseWriter, r *http.Request, studentID int64, person *peopleModule.Person, hasFullAccess, companionsChanged bool, now time.Time) {
	updatedStudent, err := rs.findStudent(r.Context(), studentID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	group := rs.getStudentGroup(r.Context(), updatedStudent)

	photosEnabled, err := rs.SettingsService.ResolveBool(r.Context(), settingStudentPhotosEnabled)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	response, err := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       updatedStudent,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
	})
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	// Same overlay as the detail read: the caller judges the write by this
	// response, and a status day without the live flag (a parent's Abmeldung,
	// #1735) still counts. A lift that left such a row active must not read as
	// done (#3854).
	if err := rs.applyStatusDaysForDateToResponse(r.Context(), &response, now); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	response.CompanionsChanged = &companionsChanged
	common.Respond(w, r, http.StatusOK, response, "Student updated successfully")
}
