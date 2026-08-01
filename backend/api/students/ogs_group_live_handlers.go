package students

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// OGS group live projection (#2056): one request returns everything the
// OGS-Gruppen page renders for one group, replacing the former 9-request
// Next.js BFF fan-out. The DTO carries ONLY fields the page displays — no
// guardian contact data, addresses, health info, or other wide student fields.
//
// Error contract: every sub-load is required. Any failure fails the whole
// request with a 5xx instead of degrading to an empty section, so the UI can
// never render partial data as a valid empty state.

// OGSLiveGroupResponse is one supervised group in the group switcher.
type OGSLiveGroupResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	RoomID          *int64 `json:"room_id,omitempty"`
	RoomName        string `json:"room_name,omitempty"`
	ViaSubstitution bool   `json:"via_substitution"`
}

// OGSLiveStudentResponse is the minimal per-student projection for the group
// card grid. Field semantics match the corresponding StudentResponse fields.
type OGSLiveStudentResponse struct {
	ID                 int64      `json:"id"`
	FirstName          string     `json:"first_name"`
	LastName           string     `json:"last_name"`
	SchoolClass        string     `json:"school_class"`
	Location           string     `json:"current_location"`
	LocationSince      *time.Time `json:"location_since,omitempty"`
	RoomColor          *string    `json:"current_room_color,omitempty"`
	Sick               bool       `json:"sick"`
	SickSince          *time.Time `json:"sick_since,omitempty"`
	Excused            bool       `json:"excused"`
	ExcusedSince       *time.Time `json:"excused_since,omitempty"`
	ClassTrip          bool       `json:"class_trip"`
	ClassTripSince     *time.Time `json:"class_trip_since,omitempty"`
	DayPlanningStatus  string     `json:"day_planning_status,omitempty"`
	DayPlanningLabel   string     `json:"day_planning_label,omitempty"`
	PendingExcusedNote *string    `json:"pending_excused_note,omitempty"`
	ArrivalTime        *string    `json:"arrival_time,omitempty"`
	ArrivalIsException bool       `json:"arrival_is_exception,omitempty"`
	ArrivalNotes       string     `json:"arrival_notes,omitempty"`
	ActualArrivalTime  *string    `json:"actual_arrival_time,omitempty"`
	ActualPickupTime   *string    `json:"actual_pickup_time,omitempty"`
	PhotoURL           string     `json:"photo_url,omitempty"`
}

// OGSLiveRoomStatus mirrors the fields the page's location filter reads from
// the former room-status endpoint.
type OGSLiveRoomStatus struct {
	InGroupRoom   bool   `json:"in_group_room"`
	CurrentRoomID *int64 `json:"current_room_id,omitempty"`
}

// OGSLiveTransferResponse is a same-day group transfer (regular_staff_id IS
// NULL substitution) in the shape the transfer badge needs.
type OGSLiveTransferResponse struct {
	ID                int64  `json:"id"`
	GroupID           int64  `json:"group_id"`
	SubstituteStaffID int64  `json:"substitute_staff_id"`
	SubstituteName    string `json:"substitute_name,omitempty"`
	EndDate           string `json:"end_date"`
}

// OGSLiveTrackingIndicators mirrors TrackingIndicatorsResponse.
type OGSLiveTrackingIndicators struct {
	Labels  []string         `json:"labels"`
	Results map[int64][]bool `json:"results"`
}

// OGSGroupLiveResponse aggregates all live data of one OGS group.
type OGSGroupLiveResponse struct {
	Groups             []OGSLiveGroupResponse       `json:"groups"`
	GroupID            *int64                       `json:"group_id,omitempty"`
	Students           []OGSLiveStudentResponse     `json:"students"`
	RoomStatus         map[string]OGSLiveRoomStatus `json:"room_status"`
	PickupTimes        []BulkPickupTimeResponse     `json:"pickup_times"`
	TrackingIndicators OGSLiveTrackingIndicators    `json:"tracking_indicators"`
	Transfers          []OGSLiveTransferResponse    `json:"transfers"`
}

func emptyOGSGroupLiveResponse() *OGSGroupLiveResponse {
	return &OGSGroupLiveResponse{
		Groups:             []OGSLiveGroupResponse{},
		Students:           []OGSLiveStudentResponse{},
		RoomStatus:         map[string]OGSLiveRoomStatus{},
		PickupTimes:        []BulkPickupTimeResponse{},
		TrackingIndicators: OGSLiveTrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}},
		Transfers:          []OGSLiveTransferResponse{},
	}
}

// getOGSGroupLive handles GET /students/ogs-group-live?group_id={id}.
// Without group_id the first supervised group (sorted by name) is selected;
// with group_id the caller must supervise that group (directly or via
// substitution), otherwise 403 — the same access rule the room-status endpoint
// applies to non-admins.
func (rs *Resource) getOGSGroupLive(w http.ResponseWriter, r *http.Request) {
	requestedGroupID, ok := parseOptionalGroupID(w, r)
	if !ok {
		return
	}

	groups, selected, err := rs.resolveOGSLiveGroups(r, requestedGroupID)
	if err != nil {
		common.RenderError(w, r, err)
		return
	}
	if selected == nil {
		common.Respond(w, r, http.StatusOK, emptyOGSGroupLiveResponse(), "No supervised groups")
		return
	}

	response, buildErr := rs.buildOGSGroupLiveResponse(r, groups, selected)
	if buildErr != nil {
		common.RenderError(w, r, buildErr)
		return
	}

	common.Respond(w, r, http.StatusOK, response, "OGS group live data retrieved successfully")
}

func parseOptionalGroupID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("group_id"))
	if raw == "" {
		return 0, true
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid group_id")))
		return 0, false
	}
	return id, true
}

// resolveOGSLiveGroups loads the caller's supervised groups (with room names
// and substitution metadata), sorts them by name, and picks the selected
// group. A nil selected group with nil error means "no groups at all".
func (rs *Resource) resolveOGSLiveGroups(r *http.Request, requestedGroupID int64) ([]OGSLiveGroupResponse, *educationModels.Group, render.Renderer) {
	ctx := r.Context()

	myGroups, err := rs.UserContextService.GetMyGroups(ctx)
	if err != nil {
		return nil, nil, common.ErrorInternalServerWrap("failed to load supervised groups", err)
	}
	if len(myGroups) == 0 {
		if requestedGroupID > 0 {
			return nil, nil, common.ErrorForbidden(errors.New("you do not supervise this group"))
		}
		return nil, nil, nil
	}

	substitutedIDs, err := rs.UserContextService.GetSubstitutedGroupIDs(ctx)
	if err != nil {
		return nil, nil, common.ErrorInternalServerWrap("failed to load substitution metadata", err)
	}

	sort.Slice(myGroups, func(i, j int) bool {
		return strings.ToLower(myGroups[i].Name) < strings.ToLower(myGroups[j].Name)
	})

	selected := selectOGSLiveGroup(myGroups, requestedGroupID)
	if selected == nil {
		return nil, nil, common.ErrorForbidden(errors.New("you do not supervise this group"))
	}

	responses, roomErr := rs.buildOGSLiveGroupList(ctx, myGroups, substitutedIDs)
	if roomErr != nil {
		return nil, nil, common.ErrorInternalServerWrap("failed to load group room", roomErr)
	}

	return responses, selected, nil
}

// selectOGSLiveGroup picks the requested group from the caller's supervised
// groups, or the first (name-sorted) group when none was requested. Returns
// nil when the requested group is not supervised.
func selectOGSLiveGroup(myGroups []*educationModels.Group, requestedGroupID int64) *educationModels.Group {
	if requestedGroupID <= 0 {
		return myGroups[0]
	}
	for _, group := range myGroups {
		if group.ID == requestedGroupID {
			return group
		}
	}
	return nil
}

// buildOGSLiveGroupList maps the supervised groups to the switcher DTO,
// resolving room names per group. Bounded by the caller's group count
// (typically 1–3), not by student count.
func (rs *Resource) buildOGSLiveGroupList(ctx context.Context, myGroups []*educationModels.Group, substitutedIDs map[int64]bool) ([]OGSLiveGroupResponse, error) {
	responses := make([]OGSLiveGroupResponse, 0, len(myGroups))
	for _, group := range myGroups {
		item := OGSLiveGroupResponse{
			ID:              group.ID,
			Name:            group.Name,
			RoomID:          group.RoomID,
			ViaSubstitution: substitutedIDs[group.ID],
		}
		if group.RoomID != nil {
			// GetMyGroups does not preload rooms; resolve the name per group.
			withRoom, err := rs.EducationService.FindGroupWithRoom(ctx, group.ID)
			if err != nil {
				return nil, err
			}
			if withRoom.Room != nil {
				item.RoomName = withRoom.Room.Name
			}
		}
		responses = append(responses, item)
	}
	return responses, nil
}

// buildOGSGroupLiveResponse assembles the full projection for the selected
// group. Every sub-load error is returned (→ 5xx), never downgraded to an
// empty section.
func (rs *Resource) buildOGSGroupLiveResponse(r *http.Request, groups []OGSLiveGroupResponse, selected *educationModels.Group) (*OGSGroupLiveResponse, render.Renderer) {
	ctx := common.PrefetchSettings(r.Context(), rs.SettingsService,
		configModel.KeyStudentPhotosEnabled,
		configModel.KeyTrackingIndicatorsEnabled,
		configModel.KeyTrackingIndicator1,
		configModel.KeyTrackingIndicator2,
		configModel.KeyTrackingIndicator3,
	)
	today := timezone.DateFromTime(rs.Now())

	students, err := rs.PersonService.GetStudentsByGroupIDs(ctx, []int64{selected.ID})
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load group students", err)
	}

	params := &studentListParams{groupID: selected.ID, includeArrivalTimes: true}
	accessCtx := rs.determineStudentAccess(r)
	studentIDs, personIDs, groupIDs := collectIDsFromStudents(students)
	dataSnapshot, err := common.LoadStudentDataSnapshot(ctx, rs.PersonService, rs.EducationService, rs.ActiveService, studentIDs, personIDs, groupIDs)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load student data snapshot", err)
	}

	photosEnabled := configService.ResolveBoolOrDefault(ctx, rs.SettingsService, configModel.KeyStudentPhotosEnabled, false, rs.Logger)
	responses := rs.buildStudentResponses(ctx, students, params, accessCtx, dataSnapshot, photosEnabled)

	if err := rs.applyStatusDaysForDate(ctx, responses, today.BerlinMidnight()); err != nil {
		return nil, common.ErrorInternalServerWrap("failed to apply student status days", err)
	}

	fullAccessIDs := collectFullAccessStudentIDs(responses)
	arrivals, err := rs.loadDayPlanningArrivals(ctx, fullAccessIDs, today)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load arrival times", err)
	}
	pickups, err := rs.loadDayPlanningPickups(ctx, fullAccessIDs, today)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load pickup times", err)
	}
	timetableIDs, err := rs.loadDayPlanningTimetableIDs(ctx, fullAccessIDs, today)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load timetable planning", err)
	}
	pendingExcused, err := rs.loadPendingExcusedForDayPlanning(ctx, today)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load pending excused requests", err)
	}
	applyDayPlanning(responses, arrivals, pickups, attendanceMapFromSnapshot(dataSnapshot), timetableIDs, pendingExcused, true)

	for i := range responses {
		if responses[i].HasFullAccess {
			applyActualTimesFromSnapshot(&responses[i], dataSnapshot)
		}
	}
	applyArrivalTimesFromMap(responses, arrivals)

	trackingIndicators, err := rs.loadOGSLiveTrackingIndicators(ctx, studentIDs)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load tracking indicators", err)
	}

	transfers, err := rs.loadOGSLiveTransfers(ctx, selected.ID, today)
	if err != nil {
		return nil, common.ErrorInternalServerWrap("failed to load group transfers", err)
	}

	selectedID := selected.ID
	return &OGSGroupLiveResponse{
		Groups:             groups,
		GroupID:            &selectedID,
		Students:           projectOGSLiveStudents(responses),
		RoomStatus:         buildOGSLiveRoomStatus(responses, dataSnapshot, selected.RoomID),
		PickupTimes:        buildOGSLivePickupTimes(responses, pickups),
		TrackingIndicators: trackingIndicators,
		Transfers:          transfers,
	}, nil
}

// applyArrivalTimesFromMap sets the arrival fields from an already-loaded bulk
// map — same semantics as enrichWithArrivalTimes, minus the second query.
func applyArrivalTimesFromMap(responses []StudentResponse, arrivals map[int64]*scheduleService.EffectiveArrivalTime) {
	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		eat, ok := arrivals[responses[i].ID]
		if !ok {
			continue
		}
		if eat.ArrivalTime != nil {
			formatted := eat.ArrivalTime.Format("15:04")
			responses[i].ArrivalTime = &formatted
		}
		responses[i].ArrivalIsException = eat.IsException
		responses[i].ArrivalNotes = buildArrivalNotes(eat)
	}
}

func projectOGSLiveStudents(responses []StudentResponse) []OGSLiveStudentResponse {
	students := make([]OGSLiveStudentResponse, 0, len(responses))
	for i := range responses {
		r := &responses[i]
		students = append(students, OGSLiveStudentResponse{
			ID:                 r.ID,
			FirstName:          r.FirstName,
			LastName:           r.LastName,
			SchoolClass:        r.SchoolClass,
			Location:           r.Location,
			LocationSince:      r.LocationSince,
			RoomColor:          r.RoomColor,
			Sick:               r.Sick,
			SickSince:          r.SickSince,
			Excused:            r.Excused,
			ExcusedSince:       r.ExcusedSince,
			ClassTrip:          r.ClassTrip,
			ClassTripSince:     r.ClassTripSince,
			DayPlanningStatus:  r.DayPlanningStatus,
			DayPlanningLabel:   r.DayPlanningLabel,
			PendingExcusedNote: r.PendingExcusedNote,
			ArrivalTime:        r.ArrivalTime,
			ArrivalIsException: r.ArrivalIsException,
			ArrivalNotes:       r.ArrivalNotes,
			ActualArrivalTime:  r.ActualArrivalTime,
			ActualPickupTime:   r.ActualPickupTime,
			PhotoURL:           r.PhotoURL,
		})
	}
	return students
}

// buildOGSLiveRoomStatus derives each student's room status from the location
// snapshot already loaded for the student projection — no extra queries. In
// binary presence mode the snapshot carries no visits, so every student
// reports in_group_room=false, matching a mode whose UI hides room state.
func buildOGSLiveRoomStatus(responses []StudentResponse, dataSnapshot *common.StudentDataSnapshot, groupRoomID *int64) map[string]OGSLiveRoomStatus {
	result := make(map[string]OGSLiveRoomStatus, len(responses))
	var snapshot *common.StudentLocationSnapshot
	if dataSnapshot != nil {
		snapshot = dataSnapshot.LocationSnapshot
	}
	for i := range responses {
		status := OGSLiveRoomStatus{}
		if snapshot != nil {
			if visit := snapshot.Visits[responses[i].ID]; visit != nil {
				if activeGroup := snapshot.Groups[visit.ActiveGroupID]; activeGroup != nil {
					roomID := activeGroup.RoomID
					status.CurrentRoomID = &roomID
					status.InGroupRoom = groupRoomID != nil && roomID == *groupRoomID
				}
			}
		}
		result[strconv.FormatInt(responses[i].ID, 10)] = status
	}
	return result
}

// buildOGSLivePickupTimes reuses the bulk pickup map already loaded for day
// planning; only full-access students are included (GDPR parity with the bulk
// pickup endpoint).
func buildOGSLivePickupTimes(responses []StudentResponse, pickups map[int64]*scheduleService.EffectivePickupTime) []BulkPickupTimeResponse {
	result := make([]BulkPickupTimeResponse, 0, len(responses))
	for i := range responses {
		if !responses[i].HasFullAccess {
			continue
		}
		ept, ok := pickups[responses[i].ID]
		if !ok || ept == nil {
			continue
		}
		result = append(result, mapBulkPickupTimeResponse(responses[i].ID, ept))
	}
	return result
}

// loadOGSLiveTrackingIndicators mirrors the tracking-indicators endpoint:
// disabled feature or no configured labels yield an empty result, but a real
// resolution or service failure fails the request (strict error contract).
func (rs *Resource) loadOGSLiveTrackingIndicators(ctx context.Context, studentIDs []int64) (OGSLiveTrackingIndicators, error) {
	empty := OGSLiveTrackingIndicators{Labels: []string{}, Results: map[int64][]bool{}}
	if len(studentIDs) == 0 || rs.SettingsService == nil || rs.ActiveService == nil {
		return empty, nil
	}

	enabled, err := rs.SettingsService.ResolveBool(ctx, configModel.KeyTrackingIndicatorsEnabled)
	if err != nil {
		return empty, err
	}
	if !enabled {
		return empty, nil
	}

	var labels []string
	for _, key := range []string{configModel.KeyTrackingIndicator1, configModel.KeyTrackingIndicator2, configModel.KeyTrackingIndicator3} {
		val, resolveErr := rs.SettingsService.ResolveString(ctx, key)
		if resolveErr != nil {
			return empty, resolveErr
		}
		if trimmed := strings.TrimSpace(val); trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	if len(labels) == 0 {
		return empty, nil
	}

	results, err := rs.ActiveService.GetTrackingIndicators(ctx, studentIDs, labels)
	if err != nil {
		return empty, err
	}
	return OGSLiveTrackingIndicators{Labels: labels, Results: results}, nil
}

// loadOGSLiveTransfers returns today's same-day transfers (substitutions with
// an unassigned regular staff slot) for the group.
func (rs *Resource) loadOGSLiveTransfers(ctx context.Context, groupID int64, today timezone.Date) ([]OGSLiveTransferResponse, error) {
	substitutions, err := rs.EducationService.GetActiveGroupSubstitutions(ctx, groupID, today)
	if err != nil {
		return nil, err
	}

	transfers := make([]OGSLiveTransferResponse, 0, len(substitutions))
	for _, sub := range substitutions {
		if sub.RegularStaffID != nil {
			continue
		}
		transfer := OGSLiveTransferResponse{
			ID:                sub.ID,
			GroupID:           sub.GroupID,
			SubstituteStaffID: sub.SubstituteStaffID,
			EndDate:           sub.EndDate.String(),
		}
		if sub.SubstituteStaff != nil && sub.SubstituteStaff.Person != nil {
			transfer.SubstituteName = strings.TrimSpace(sub.SubstituteStaff.Person.FirstName + " " + sub.SubstituteStaff.Person.LastName)
		}
		transfers = append(transfers, transfer)
	}
	return transfers, nil
}
