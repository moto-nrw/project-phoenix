package students

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// StudentResponseOpts groups parameters for creating a student response to reduce function parameter count
type StudentResponseOpts struct {
	Student          *users.Student
	Person           *users.Person
	Group            *education.Group
	HasFullAccess    bool
	LocationOverride *string
	// Resolve once per request and thread through — populatePhotoFields
	// runs per-student in list paths.
	PhotosEnabled bool
}

// StudentResponseServices groups service dependencies for student response creation
type StudentResponseServices struct {
	ActiveService activeService.Service
	PersonService userService.PersonService
}

// populatePersonAndGuardianData fills the response with person and guardian information
// based on access level permissions
func populatePersonAndGuardianData(response *StudentResponse, person *users.Person, student *users.Student, group *education.Group, hasFullAccess bool) {
	if person != nil {
		response.FirstName = person.FirstName
		response.LastName = person.LastName
		// Format birthday as YYYY-MM-DD string if available
		if person.Birthday != nil {
			response.Birthday = person.Birthday.Format(dateFormatYYYYMMDD)
		}
		// Only include RFID tag for users with full access
		if hasFullAccess && person.TagID != nil {
			response.TagID = *person.TagID
		}
	}

	// Guardian email and phone are visible to all authenticated staff
	if student.GuardianEmail != nil {
		response.GuardianEmail = *student.GuardianEmail
	}

	if student.GuardianPhone != nil {
		response.GuardianPhone = *student.GuardianPhone
	}

	if student.GroupID != nil {
		response.GroupID = *student.GroupID
	}

	if group != nil {
		response.GroupName = group.Name
	}
}

// populatePublicStudentFields sets fields visible to all authenticated staff
func populatePublicStudentFields(response *StudentResponse, student *users.Student) {
	if student.HealthInfo != nil {
		response.HealthInfo = *student.HealthInfo
	}
	allowed := student.AllowedDepartureModes.Normalize()
	if !allowed.HasAny() {
		allowed = users.AllowedDepartureModesFromDeparture(student.DepartureDays)
	}
	departure := allowed.DepartureDays()
	response.AllowedDepartureModes = allowed
	response.DepartureDays = departure
	response.BusDays = allowed.BusDays()
	response.Bus = response.BusDays.HasAny()
	response.PickupDays = allowed.PickupDays()
	// Derive pickup_status from the FULL non-exclusive set, not the exclusive
	// `departure` projection: the projection ranks bus over accompanied, so a day
	// allowing both would drop the accompanied signal and return this child to
	// legacy list/search/admin consumers as a self-goer (#1694).
	response.PickupStatus = responsePickupStatus(student, allowed.LegacyPickupStatus())
}

func responsePickupStatus(student *users.Student, derived string) string {
	if student.PickupStatus == nil {
		return derived
	}
	stored := strings.TrimSpace(*student.PickupStatus)
	if stored == "" ||
		stored == users.PickupStatusPickedUp ||
		stored == users.PickupStatusGoesAlone ||
		stored == users.PickupStatusAccompanied {
		return derived
	}
	return *student.PickupStatus
}

// populateSensitiveStudentFields sets fields visible only to supervisors/admins
func populateSensitiveStudentFields(response *StudentResponse, student *users.Student) {
	if student.ExtraInfo != nil && *student.ExtraInfo != "" {
		response.ExtraInfo = *student.ExtraInfo
	}
	if student.DepartureCompanionNote != nil && *student.DepartureCompanionNote != "" {
		response.DepartureCompanionNote = *student.DepartureCompanionNote
	}
	if student.SupervisorNotes != nil {
		response.SupervisorNotes = *student.SupervisorNotes
	}
	if student.Sick != nil {
		response.Sick = *student.Sick
	}
	if student.SickSince != nil {
		response.SickSince = student.SickSince
	}
	if student.Excused != nil {
		response.Excused = *student.Excused
	}
	if student.ExcusedSince != nil {
		response.ExcusedSince = student.ExcusedSince
	}
}

func populateStudentAddressFields(response *StudentResponse, student *users.Student) {
	if student.AddressStreet != nil {
		response.AddressStreet = *student.AddressStreet
	}
	if student.AddressCity != nil {
		response.AddressCity = *student.AddressCity
	}
	if student.AddressPostalCode != nil {
		response.AddressPostalCode = *student.AddressPostalCode
	}
}

// populatePhotoFields fills the response with photo URL + consent metadata.
// Visible to all authenticated staff so any list view can render the avatar.
//
// When photosEnabled is false (operations.student_photos_enabled off for
// the tenant) we skip every photo-related field. Otherwise an admin who
// turns the feature off would still see photo_url + consent metadata in
// API responses for rows that already had a photo uploaded — the
// frontend would hide the avatar, but the bytes would still be reachable
// through the JSON URL. Caller resolves the flag once per request.
//
// The DB stores the raw `/uploads/student-photos/{filename}` path (the
// serve route keys cleanup off this prefix); the frontend can't fetch
// `/uploads/...` directly because Next.js doesn't serve that path. The
// JSON-facing URL is rewritten to the authenticated proxy URL by
// `common.BuildStudentPhotoServeURL` — same helper the active-group visit
// response uses, so the two endpoints can never drift.
// populatePhotoFields fills the response with photo URL + consent metadata.
//
// hasFullAccess MUST mirror the predicate serveStudentPhoto uses internally
// (authorize.CanReadStudent — i.e. admin OR all_staff scope OR caller
// supervises the student's group): the byte-serving route 403s callers that
// fail it, so emitting photo_url for rows the same session is forbidden to
// fetch would let list/search responses hand out URLs that immediately bounce
// to a broken-image fetch. The boolean PhotoConsentGiven flag is fine to
// surface tenant-wide — it's not GDPR-sensitive on its own — only the URL
// itself is gated.
//
// When photosEnabled is false we still skip every photo field so an admin
// who turns the feature off mid-session no longer sees photo_url + consent
// metadata in API responses for rows that already had a photo uploaded.
func populatePhotoFields(response *StudentResponse, student *users.Student, photosEnabled, hasFullAccess bool) {
	if !photosEnabled {
		// All photo fields stay at their zero values. With
		// PhotoConsentGiven typed as *bool + omitempty, leaving it nil
		// suppresses it from the JSON entirely — the frontend cannot
		// confuse "feature off" with "consent withdrawn". Same goes for
		// PhotoURL (string + omitempty drops the empty string).
		return
	}
	// PhotoURL is only emitted when the caller can actually fetch it.
	// hasFullAccess matches the access gate inside serveStudentPhoto so
	// the URL we hand out is one the same session is allowed to render.
	if hasFullAccess && student.PhotoPath != nil {
		response.PhotoURL = common.BuildStudentPhotoServeURL(student.ID, *student.PhotoPath)
	}
	// Surface the explicit boolean state to the frontend so the consent
	// checkbox can render correctly: true when consent has been recorded,
	// false when photos are enabled but consent is not given (or was
	// withdrawn). Nil is reserved for the feature-off branch above.
	consentGiven := student.PhotoConsentGivenAt != nil
	response.PhotoConsentGiven = &consentGiven
	if student.PhotoConsentGivenAt != nil {
		response.PhotoConsentGivenAt = student.PhotoConsentGivenAt
	}
	if student.PhotoConsentGivenBy != nil {
		response.PhotoConsentGivenBy = student.PhotoConsentGivenBy
	}
}

// populateEnrollmentConsents emits the AGB / Datenschutz / E-Mail
// consent stamps regardless of the photo feature flag — these are
// operational records (DSGVO is the data-processing baseline, not a
// photo gate). Photo consent stays under populatePhotoFields so its
// visibility tracks the photos feature flag.
func populateEnrollmentConsents(response *StudentResponse, student *users.Student) {
	if student.AGBAcceptedAt != nil {
		response.AGBAcceptedAt = student.AGBAcceptedAt
	}
	if student.DataProcessingAcceptedAt != nil {
		response.DataProcessingAcceptedAt = student.DataProcessingAcceptedAt
	}
	if student.EmailContactAcceptedAt != nil {
		response.EmailContactAcceptedAt = student.EmailContactAcceptedAt
	}
}

// populateSnapshotSensitiveFields sets sensitive fields for the snapshot version
// Note: This differs from populateSensitiveStudentFields by including HealthInfo
func populateSnapshotSensitiveFields(response *StudentResponse, student *users.Student) {
	if student.ExtraInfo != nil && *student.ExtraInfo != "" {
		response.ExtraInfo = *student.ExtraInfo
	}
	if student.DepartureCompanionNote != nil && *student.DepartureCompanionNote != "" {
		response.DepartureCompanionNote = *student.DepartureCompanionNote
	}
	if student.HealthInfo != nil {
		response.HealthInfo = *student.HealthInfo
	}
	if student.SupervisorNotes != nil {
		response.SupervisorNotes = *student.SupervisorNotes
	}
	if student.Sick != nil {
		response.Sick = *student.Sick
	}
	if student.SickSince != nil {
		response.SickSince = student.SickSince
	}
	if student.Excused != nil {
		response.Excused = *student.Excused
	}
	if student.ExcusedSince != nil {
		response.ExcusedSince = student.ExcusedSince
	}
}

// populateSnapshotPublicFields sets fields visible to all staff in snapshot version
func populateSnapshotPublicFields(response *StudentResponse, student *users.Student) {
	allowed := student.AllowedDepartureModes.Normalize()
	if !allowed.HasAny() {
		allowed = users.AllowedDepartureModesFromDeparture(student.DepartureDays)
	}
	departure := allowed.DepartureDays()
	response.AllowedDepartureModes = allowed
	response.DepartureDays = departure
	response.BusDays = allowed.BusDays()
	response.Bus = response.BusDays.HasAny()
	response.PickupDays = allowed.PickupDays()
	// Full set, not the exclusive projection: see populatePublicStudentFields (#1694).
	response.PickupStatus = responsePickupStatus(student, allowed.LegacyPickupStatus())
}

// presentOrTransit returns the appropriate location for a checked-in student
// without a specific room assignment, based on access level.
func presentOrTransit(hasFullAccess bool) common.StudentLocationInfo {
	if hasFullAccess {
		return common.StudentLocationInfo{Location: "Unterwegs"}
	}
	return common.StudentLocationInfo{Location: "Anwesend"}
}

// absentInfo returns the "Abwesend" location, optionally with checkout time for full access users.
func absentInfo(hasFullAccess bool, checkOutTime *time.Time) common.StudentLocationInfo {
	if hasFullAccess && checkOutTime != nil {
		return common.StudentLocationInfo{Location: "Abwesend", Since: checkOutTime}
	}
	return common.StudentLocationInfo{Location: "Abwesend"}
}

// resolveStudentLocationWithTime determines a student's current location with timestamp.
//
// Binary-mode tenants short-circuit to ResolveBinaryLocation — web check-ins
// write only attendance (no room visit), so falling through to
// presentOrTransit() would always yield "Unterwegs", contradicting the
// simplified Anwesend/Schulhof/Abwesend UX binary mode promises.
func resolveStudentLocationWithTime(ctx context.Context, studentID int64, hasFullAccess bool, activeService activeService.Service) common.StudentLocationInfo {
	attendanceStatus, err := activeService.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil || attendanceStatus == nil {
		return common.StudentLocationInfo{Location: "Abwesend"}
	}

	if activeService.GetPresenceMode(ctx) == common.PresenceModeBinary {
		return common.ResolveBinaryLocation(attendanceStatus, hasFullAccess)
	}

	// Handle non-checked-in states (checked_out or other)
	if attendanceStatus.Status != "checked_in" {
		return absentInfo(hasFullAccess, attendanceStatus.CheckOutTime)
	}

	// Student is checked in - get current visit to check room assignment
	currentVisit, err := activeService.GetStudentCurrentVisit(ctx, studentID)
	if err != nil || currentVisit == nil || currentVisit.ActiveGroupID <= 0 {
		return presentOrTransit(hasFullAccess)
	}

	activeGroup, err := activeService.GetActiveGroup(ctx, currentVisit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return presentOrTransit(hasFullAccess)
	}

	// Include room name for all authenticated staff (needed for supervised room checkout)
	if activeGroup.Room != nil && activeGroup.Room.Name != "" {
		return common.StudentLocationInfo{
			Location:  fmt.Sprintf("Anwesend - %s", activeGroup.Room.Name),
			Since:     &currentVisit.EntryTime,
			RoomColor: activeGroup.Room.Color,
		}
	}

	return presentOrTransit(hasFullAccess)
}

// newStudentResponseWithOpts creates a student response using options structs
func newStudentResponseWithOpts(ctx context.Context, opts StudentResponseOpts, services StudentResponseServices) StudentResponse {
	student := opts.Student
	person := opts.Person
	group := opts.Group
	hasFullAccess := opts.HasFullAccess
	locationOverride := opts.LocationOverride
	response := StudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		SchoolClass: student.SchoolClass,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	}

	// Include legacy guardian name if available
	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}

	// Guardian contact info is visible to all authenticated staff
	if student.GuardianContact != nil {
		response.GuardianContact = *student.GuardianContact
	}

	response.HasFullAccess = hasFullAccess

	// Resolve location
	if locationOverride != nil {
		response.Location = *locationOverride
	} else {
		locationInfo := resolveStudentLocationWithTime(ctx, student.ID, hasFullAccess, services.ActiveService)
		response.Location = locationInfo.Location
		response.LocationSince = locationInfo.Since
		response.RoomColor = locationInfo.RoomColor
	}

	populatePersonAndGuardianData(&response, person, student, group, hasFullAccess)
	populatePublicStudentFields(&response, student)

	// Sensitive student fields (notes, sickness) are now visible to all authenticated staff
	populateSensitiveStudentFields(&response, student)
	if hasFullAccess {
		populateStudentAddressFields(&response, student)
	}

	// Photo + consent metadata. Suppressed entirely when the feature is
	// off; PhotoURL additionally requires hasFullAccess so we never hand
	// out a URL the same session would 403 against in serveStudentPhoto.
	populatePhotoFields(&response, student, opts.PhotosEnabled, hasFullAccess)

	// AGB / Datenschutz / E-Mail consents flow through independently
	// of the photo feature flag — see populateEnrollmentConsents.
	populateEnrollmentConsents(&response, student)

	return response
}

// newStudentResponseFromSnapshot creates a student response using pre-loaded snapshot data
// This eliminates N+1 queries by using cached person, group, and location data
func newStudentResponseFromSnapshot(_ context.Context, student *users.Student, person *users.Person, group *education.Group, hasFullAccess bool, snapshot *common.StudentDataSnapshot, photosEnabled bool) StudentResponse {
	response := StudentResponse{
		ID:          student.ID,
		PersonID:    student.PersonID,
		SchoolClass: student.SchoolClass,
		CreatedAt:   student.CreatedAt,
		UpdatedAt:   student.UpdatedAt,
	}

	if student.GuardianName != nil {
		response.GuardianName = *student.GuardianName
	}

	// Guardian contact info is visible to all authenticated staff
	if student.GuardianContact != nil {
		response.GuardianContact = *student.GuardianContact
	}

	response.HasFullAccess = hasFullAccess

	locationInfo := snapshot.ResolveLocationWithTime(student.ID, hasFullAccess)
	response.Location = locationInfo.Location
	response.LocationSince = locationInfo.Since
	response.RoomColor = locationInfo.RoomColor

	populatePersonAndGuardianData(&response, person, student, group, hasFullAccess)
	populateSnapshotPublicFields(&response, student)

	// Sensitive student fields (notes, sickness) are now visible to all authenticated staff
	populateSnapshotSensitiveFields(&response, student)
	if hasFullAccess {
		populateStudentAddressFields(&response, student)
	}

	// Photo + consent metadata — same rationale as in newStudentResponseWithOpts.
	populatePhotoFields(&response, student, photosEnabled, hasFullAccess)
	populateEnrollmentConsents(&response, student)

	return response
}

// newPrivacyConsentResponse converts a privacy consent model to a response
func newPrivacyConsentResponse(consent *users.PrivacyConsent) PrivacyConsentResponse {
	return PrivacyConsentResponse{
		ID:                consent.ID,
		StudentID:         consent.StudentID,
		PolicyVersion:     consent.PolicyVersion,
		Accepted:          consent.Accepted,
		AcceptedAt:        consent.AcceptedAt,
		ExpiresAt:         consent.ExpiresAt,
		DurationDays:      consent.DurationDays,
		RenewalRequired:   consent.RenewalRequired,
		DataRetentionDays: consent.DataRetentionDays,
		Details:           consent.Details,
		CreatedAt:         consent.CreatedAt,
		UpdatedAt:         consent.UpdatedAt,
	}
}

// teacherToSupervisorContact converts a teacher to a supervisor contact if valid
func teacherToSupervisorContact(teacher *users.Teacher) *SupervisorContact {
	if teacher == nil || teacher.Staff == nil || teacher.Staff.Person == nil {
		return nil
	}

	supervisor := &SupervisorContact{
		ID:        teacher.ID,
		FirstName: teacher.Staff.Person.FirstName,
		LastName:  teacher.Staff.Person.LastName,
		Role:      "teacher",
	}

	if teacher.Staff.Person.Account != nil {
		supervisor.Email = teacher.Staff.Person.Account.Email
	}

	return supervisor
}

// enrichWithPickupTimes adds today's effective pickup time to each student response.
// Uses a single bulk query via PickupScheduleService (handles schedule + exception merging).
// Only students with HasFullAccess=true receive pickup times (GDPR).
func (rs *Resource) enrichWithPickupTimes(ctx context.Context, responses []StudentResponse, studentIDs []int64, now time.Time) {
	if len(studentIDs) == 0 || rs.PickupScheduleService == nil {
		return
	}

	pickupTimes, err := rs.PickupScheduleService.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, timezone.DateFromTime(now))
	if err != nil {
		rs.Logger.Warn("failed to bulk-fetch pickup times", "error", err.Error())
		return
	}

	applyPickupTimesFromMap(responses, pickupTimes)
}

// applyPickupTimesFromMap writes already-loaded effective pickup times onto the
// responses without touching the database, so a pipeline stage that has the
// bulk map in hand does not re-run the three pickup SELECTs (#2098).
func applyPickupTimesFromMap(responses []StudentResponse, pickupTimes map[int64]*schedule.EffectivePickupTime) {
	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		if ept, ok := pickupTimes[responses[i].ID]; ok {
			if ept.PickupTime != nil {
				formatted := ept.PickupTime.Format("15:04")
				responses[i].PickupTime = &formatted
			}
			responses[i].PickupIsException = ept.IsException
			responses[i].PickupNotes = buildPickupNotes(ept)
		}
	}
}

// enrichWithArrivalTimes adds today's effective arrival time to each student response.
// It mirrors pickup enrichment so student list consumers can render arrival badges
// from their primary SWR cache instead of maintaining a second cache.
func (rs *Resource) enrichWithArrivalTimes(ctx context.Context, responses []StudentResponse, studentIDs []int64, now time.Time) {
	if len(studentIDs) == 0 || rs.ArrivalScheduleService == nil {
		return
	}

	arrivalTimes, err := rs.ArrivalScheduleService.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, timezone.DateFromTime(now))
	if err != nil {
		rs.Logger.Warn("failed to bulk-fetch arrival times", "error", err.Error())
		return
	}

	applyArrivalTimesFromMap(responses, arrivalTimes)
}

// applyArrivalTimesFromMap writes already-loaded effective arrival times onto
// the responses without touching the database, so a pipeline stage that has the
// bulk map in hand does not re-run the three arrival SELECTs (#2098).
func applyArrivalTimesFromMap(responses []StudentResponse, arrivalTimes map[int64]*schedule.EffectiveArrivalTime) {
	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		if eat, ok := arrivalTimes[responses[i].ID]; ok {
			if eat.ArrivalTime != nil {
				formatted := eat.ArrivalTime.Format("15:04")
				responses[i].ArrivalTime = &formatted
			}
			responses[i].ArrivalIsException = eat.IsException
			responses[i].ArrivalNotes = buildArrivalNotes(eat)
		}
	}
}

// buildPickupNotes combines exception reason and day notes into a single string.
func buildPickupNotes(ept *schedule.EffectivePickupTime) string {
	var parts []string
	if ept.Notes != "" {
		parts = append(parts, ept.Notes)
	}
	for _, n := range ept.DayNotes {
		if n.Content != "" {
			parts = append(parts, n.Content)
		}
	}
	return strings.Join(parts, ", ")
}

// buildArrivalNotes combines exception reason and day notes into a single string.
func buildArrivalNotes(eat *schedule.EffectiveArrivalTime) string {
	var parts []string
	if eat.Notes != "" {
		parts = append(parts, eat.Notes)
	}
	for _, n := range eat.DayNotes {
		if n.Content != "" {
			parts = append(parts, n.Content)
		}
	}
	return strings.Join(parts, ", ")
}

func applyActualTimesFromAttendance(response *StudentResponse, status *activeService.AttendanceStatus) {
	if response == nil || status == nil {
		return
	}

	response.ActualArrivalTime = timezone.FormatBerlinClock(status.CheckInTime)
	response.ActualPickupTime = timezone.FormatBerlinClock(status.CheckOutTime)
}

func applyActualTimesFromSnapshot(response *StudentResponse, snapshot *common.StudentDataSnapshot) {
	if response == nil || snapshot == nil || snapshot.LocationSnapshot == nil {
		return
	}

	status, ok := snapshot.LocationSnapshot.Attendances[response.ID]
	if !ok || status == nil {
		return
	}

	applyActualTimesFromAttendance(response, status)
}

// getPersonForStudent fetches the person data for a student
// Returns the person and true if successful, or renders an error and returns nil, false
func (rs *Resource) getPersonForStudent(w http.ResponseWriter, r *http.Request, student *users.Student) (*users.Person, bool) {
	person, err := rs.PersonService.Get(r.Context(), student.PersonID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to get person data for student", err))
		return nil, false
	}
	return person, true
}

// getStudentGroup fetches the group for a student if they have one assigned
func (rs *Resource) getStudentGroup(ctx context.Context, student *users.Student) *education.Group {
	if student.GroupID == nil {
		return nil
	}
	group, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil {
		return nil
	}
	return group
}

// fetchStudentGroup retrieves group data if the student has an assigned group
func (rs *Resource) fetchStudentGroup(ctx context.Context, groupID *int64) *education.Group {
	if groupID == nil {
		return nil
	}
	group, err := rs.EducationService.GetGroup(ctx, *groupID)
	if err != nil {
		return nil
	}
	return group
}

// filterStudentIDsByGroups keeps the student ids whose group is among groupIDs.
// An empty groupIDs slice means "no group restriction" and returns the input
// unchanged — the same meaning an absent group_id has everywhere else (#2218).
func (rs *Resource) filterStudentIDsByGroups(ctx context.Context, studentIDs []int64, groupIDs []int64) ([]int64, error) {
	if len(groupIDs) == 0 {
		return studentIDs, nil
	}
	wanted := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		wanted[groupID] = struct{}{}
	}
	studentMap, err := rs.PersonService.GetStudentsByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	filtered := make([]int64, 0, len(studentMap))
	for _, sid := range studentIDs {
		student, ok := studentMap[sid]
		if !ok || student.GroupID == nil {
			continue
		}
		if _, wantedGroup := wanted[*student.GroupID]; !wantedGroup {
			continue
		}
		filtered = append(filtered, sid)
	}
	return filtered, nil
}

// buildStudentResponses builds filtered student responses
func (rs *Resource) buildStudentResponses(ctx context.Context, students []*users.Student, params *studentListParams, accessCtx *studentAccessContext, dataSnapshot *common.StudentDataSnapshot, photosEnabled bool) []StudentResponse {
	responses := make([]StudentResponse, 0, len(students))

	for _, student := range students {
		response := rs.buildSingleStudentResponse(ctx, student, params, accessCtx, dataSnapshot, photosEnabled)
		if response != nil {
			responses = append(responses, *response)
		}
	}

	return responses
}

// buildSingleStudentResponse builds a response for a single student, returning nil if filtered out
func (rs *Resource) buildSingleStudentResponse(ctx context.Context, student *users.Student, params *studentListParams, accessCtx *studentAccessContext, dataSnapshot *common.StudentDataSnapshot, photosEnabled bool) *StudentResponse {
	hasFullAccess := accessCtx.HasFullAccessToStudent(student)

	// Get person data from snapshot
	person := dataSnapshot.GetPerson(student.PersonID)
	if person == nil {
		return nil
	}

	// Apply filters
	if !matchesSearchFilter(person, student.ID, params.search) {
		return nil
	}
	if !matchesNameFilters(person, params.firstName, params.lastName) {
		return nil
	}
	if !matchesGradeLevel(student.SchoolClass, params.gradeLevels) {
		return nil
	}

	// Get group data from snapshot
	var group *education.Group
	if student.GroupID != nil {
		group = dataSnapshot.GetGroup(*student.GroupID)
	}

	// Build response
	studentResponse := newStudentResponseFromSnapshot(ctx, student, person, group, hasFullAccess, dataSnapshot, photosEnabled)

	// Apply location filter
	if !matchesLocationFilter(params.location, studentResponse.Location, hasFullAccess) {
		return nil
	}

	return &studentResponse
}
