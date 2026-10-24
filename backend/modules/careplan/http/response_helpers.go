package careplanhttp

import (
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// getPersonForStudent fetches the person data for a student
// Returns the person and true if successful, or renders an error and returns nil, false
func (rs *Resource) getPersonForStudent(w http.ResponseWriter, r *http.Request, student *peopleModule.StudentState) (*peopleModule.Person, bool) {
	person, err := rs.findPerson(r.Context(), student.PersonID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to get person data for student", err))
		return nil, false
	}
	return person, true
}
