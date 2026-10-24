package careplanhttp

import (
	errors "errors"
	http "net/http"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// parseAndGetStudent parses the student ID from the URL and fetches the student
// Returns the student and true if successful, or renders an error and returns nil, false
func (rs *Resource) parseAndGetStudent(w http.ResponseWriter, r *http.Request) (*peopleModule.StudentState, bool) {
	student, ok := rs.parseAndGetStudentIncludingAlumni(w, r)
	if !ok {
		return nil, false
	}

	// A graduated (alumnus) student is soft-deleted: invisible to every staff
	// list and export. GetStudentByID is unfiltered, so this shared per-student
	// gate is where a bookmarked ID or a direct API call to any update /
	// status-day / schedule / RFID / privacy / delete route is rejected — the
	// same 404 those routes returned back when graduates were hard-deleted (#405).
	if student.Status == studentStatusAlumnus {
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	return student, true
}

// parseAndGetStudentIncludingAlumni is parseAndGetStudent without the alumnus
// gate. It exists for the ONE operation that must still work on a departed
// child: releasing the RFID bracelet they are still holding. Graduation now
// clears the tag itself, so this only covers children graduated before that
// existed — for those, the gate would otherwise create a state the kiosk can
// detect but never resolve (#405 review). Every other route uses
// parseAndGetStudent; do not widen this one.
func (rs *Resource) parseAndGetStudentIncludingAlumni(w http.ResponseWriter, r *http.Request) (*peopleModule.StudentState, bool) {
	id, err := common.ParseID(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStudentID)))
		return nil, false
	}

	student, err := rs.findStudent(r.Context(), id)
	if err != nil {
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return nil, false
	}

	return student, true
}
