package presenceprojectionhttp

import (
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	securityruntime "github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// respondCreatedStudent writes the 201 response for a freshly created student.
func (rs *Resource) RespondCreatedStudent(w http.ResponseWriter, r *http.Request, student *peopleModule.StudentState, person *peopleModule.Person) {
	// Get group data if student has a group
	group := rs.fetchStudentGroup(r.Context(), student.GroupID)

	// Admin users creating students can see full data including detailed location
	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())
	hasFullAccess := securityruntime.HasAdminWildcard(userPermissions)

	photosEnabled, err := rs.SettingsService.ResolveBool(r.Context(), settingStudentPhotosEnabled)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	response, err := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
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
	common.Respond(w, r, http.StatusCreated, response, "Student created successfully")
}
