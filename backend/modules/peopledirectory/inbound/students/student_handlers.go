package students

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// parseAndGetStudent parses the student ID from the URL and fetches the student
// Returns the student and true if successful, or renders an error and returns nil, false
func (rs *Resource) parseAndGetStudent(w http.ResponseWriter, r *http.Request) (*Student, bool) {
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
func (rs *Resource) parseAndGetStudentIncludingAlumni(w http.ResponseWriter, r *http.Request) (*Student, bool) {
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

// getStudent handles getting a student by ID
func (rs *Resource) getStudent(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if student.CareEndedOn(rs.todayDate()) &&
		!securityruntime.HasPermission(permissions.UsersDelete, jwt.PermissionsFromCtx(r.Context())) {
		renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
		return
	}

	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	group := rs.getStudentGroup(r.Context(), student)
	response, err := rs.buildStudentDetail(r, student, person, group)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if err := rs.enrichStudentDetailForToday(r.Context(), &response, student); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Add supervisor contacts for users without full access
	if !response.HasFullAccess && group != nil {
		response.GroupSupervisors = rs.buildSupervisorContacts(r.Context(), group.ID)
	}

	common.Respond(w, r, http.StatusOK, response, "Student retrieved successfully")
}

// buildStudentDetail renders the child's detail for the caller: the access
// flags, the tenant features the page switches on, and the consents.
func (rs *Resource) buildStudentDetail(r *http.Request, student *Student, person *peopleModule.Person, group *SchoolGroup) (StudentDetailResponse, error) {
	hasFullAccess := rs.checkStudentReadAccess(r, student)
	hasWriteAccess := rs.checkStudentFullAccess(r, student)

	attendanceLogEnabled := resolveBoolSetting(r.Context(), rs.SettingsService, settingAttendanceLogEnabled, false, rs.Logger)
	feedbackEnabled := resolveBoolSetting(r.Context(), rs.SettingsService, settingFeedbackEnabled, false, rs.Logger)
	photosEnabled := resolveBoolSetting(r.Context(), rs.SettingsService, settingStudentPhotosEnabled, false, rs.Logger)

	studentResponse, err := newStudentResponseWithOpts(r.Context(), StudentResponseOpts{
		Student:       student,
		Person:        person,
		Group:         group,
		HasFullAccess: hasFullAccess,
		PhotosEnabled: photosEnabled,
	}, StudentResponseServices{
		ActiveService: rs.ActiveService,
	})
	if err != nil {
		return StudentDetailResponse{}, err
	}
	response := StudentDetailResponse{
		StudentResponse:           studentResponse,
		HasFullAccess:             hasFullAccess,
		HasWriteAccess:            hasWriteAccess,
		HasAbsenceWriteAccess:     rs.checkStudentAbsenceWriteAccess(r, student),
		HasSickExcusedWriteAccess: rs.checkStudentSickExcusedWriteAccess(r, student),
		AttendanceLogEnabled:      attendanceLogEnabled,
		FeedbackEnabled:           feedbackEnabled,
	}
	if err := rs.enrichStudentConsents(r.Context(), &response, student, hasFullAccess); err != nil {
		return StudentDetailResponse{}, err
	}
	return response, nil
}

// enrichStudentDetailForToday adds today's status days, the actual arrival and
// pickup times, the day planning and the care-exit flag to the detail.
func (rs *Resource) enrichStudentDetailForToday(ctx context.Context, response *StudentDetailResponse, student *Student) error {
	now := rs.Now()
	rs.applyStatusDaysForDateToResponse(ctx, &response.StudentResponse, now)

	attendances := map[int64]*studentpresence.DailyAttendanceStatus{}
	if response.HasFullAccess {
		attendanceStatus, err := rs.ActiveService.GetStudentAttendanceStatus(ctx, student.ID)
		if err != nil {
			rs.Logger.Warn("failed to resolve actual student arrival/pickup times",
				"student_id", student.ID,
				"error", err.Error(),
			)
		} else {
			applyActualTimesFromAttendance(&response.StudentResponse, attendanceStatus)
		}
		attendances[student.ID] = attendanceStatus
	}

	// Runs for a restricted entry too: the day-planning fields stay behind
	// HasFullAccess inside, but the pending excused-absence note belongs to
	// whoever decides that request — under open care that person supervises no
	// group and sees every child restricted (#2232).
	single := []StudentResponse{response.StudentResponse}
	if _, err := rs.enrichWithDayPlanning(ctx, single, timezone.DateFromTime(now), true, attendances); err != nil {
		return err
	}
	if err := rs.enrichWithCareExitFlag(ctx, single); err != nil {
		return err
	}
	response.StudentResponse = single[0]
	return nil
}

func (rs *Resource) enrichStudentConsents(
	ctx context.Context,
	response *StudentDetailResponse,
	student *Student,
	hasFullAccess bool,
) error {
	if !hasFullAccess {
		return nil
	}
	if rs.StudentConsents == nil {
		return errors.New("student consent service not wired")
	}

	consents, err := rs.StudentConsents.CurrentStudentConsents(ctx, student.consentSnapshot(), false)
	if err != nil {
		return err
	}
	response.Consents = make([]StudentConsentResponse, 0, len(consents))
	for _, consent := range consents {
		response.Consents = append(response.Consents, StudentConsentResponse{
			Key:       consent.Key,
			State:     consent.State,
			ChangedAt: consent.ChangedAt,
		})
	}
	return nil
}
