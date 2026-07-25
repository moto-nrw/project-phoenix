package students

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/iot"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// checkDeviceAuth verifies device authentication and returns the device
// Returns the device and true if successful, or renders an error and returns nil, false
func (rs *Resource) checkDeviceAuth(w http.ResponseWriter, r *http.Request) (*iot.Device, bool) {
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		renderError(w, r, common.ErrorUnauthorized(errors.New("device authentication required")))
		return nil, false
	}
	return deviceCtx, true
}

// assignRFIDTag handles assigning an RFID tag to a student (device-authenticated endpoint)
func (rs *Resource) assignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx, ok := rs.checkDeviceAuth(w, r)
	if !ok {
		return
	}

	// Parse ID and get student
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	// Parse request
	req := &RFIDAssignmentRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get person details for the student
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// With global PIN authentication, we trust the device to assign tags to any student
	// No need to check teacher supervision rights

	// Store previous tag for response
	var previousTag *string
	if person.TagID != nil {
		previousTag = person.TagID
	}

	// Assign the RFID tag (this handles unlinking old assignments automatically).
	// Routed through the student-aware variant so the alumnus gate above is
	// re-checked under the student row lock: the parseAndGetStudent read happened
	// before this write, and a graduation apply committing in between would
	// otherwise hand a fresh bracelet to a departed child (#405 review).
	if err := rs.PersonService.LinkStudentToRFIDCard(r.Context(), student.ID, req.RFIDTag); err != nil {
		if errors.Is(err, userService.ErrStudentGraduated) || errors.Is(err, userService.ErrStudentNotFound) {
			renderError(w, r, common.ErrorNotFound(errors.New("student not found")))
			return
		}
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Create response
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     req.RFIDTag,
		PreviousTag: previousTag,
		Message:     "RFID tag assigned successfully",
	}

	if previousTag != nil {
		response.Message = "RFID tag assigned successfully (previous tag replaced)"
	}

	// Log assignment for audit trail
	slog.Default().Info("RFID tag assignment",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int64("student_id", student.ID),
		slog.String("tag", req.RFIDTag),
		slog.Any("previous_tag", previousTag))

	common.Respond(w, r, http.StatusOK, response, response.Message)
}

// unassignRFIDTag handles removing an RFID tag from a student (device-authenticated endpoint)
func (rs *Resource) unassignRFIDTag(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx, ok := rs.checkDeviceAuth(w, r)
	if !ok {
		return
	}

	// Parse ID and get student. Alumni pass on purpose: releasing a bracelet is
	// the one action that must still work on a departed child, otherwise a tag
	// left over from a graduation applied before graduation released tags itself
	// can never be freed (#405 review). Assignment stays gated — a soft-deleted
	// child must not be given a new tag.
	student, ok := rs.parseAndGetStudentIncludingAlumni(w, r)
	if !ok {
		return
	}

	// Get person details for the student
	person, ok := rs.getPersonForStudent(w, r, student)
	if !ok {
		return
	}

	// Check if student has an RFID tag assigned
	if person.TagID == nil {
		renderError(w, r, common.ErrorNotFound(errors.New("student has no RFID tag assigned")))
		return
	}

	// Store removed tag for response
	removedTag := *person.TagID

	// Unlink the RFID tag
	if err := rs.PersonService.UnlinkFromRFIDCard(r.Context(), person.ID); err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Create response
	response := RFIDAssignmentResponse{
		Success:     true,
		StudentID:   student.ID,
		StudentName: person.FirstName + " " + person.LastName,
		RFIDTag:     removedTag,
		Message:     "RFID tag unassigned successfully",
	}

	// Log unassignment for audit trail
	slog.Default().Info("RFID tag unassignment",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int64("student_id", student.ID),
		slog.String("tag", removedTag))

	common.Respond(w, r, http.StatusOK, response, response.Message)
}
