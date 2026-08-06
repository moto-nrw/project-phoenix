package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// dashboardBaseData holds the raw data fetched for dashboard analytics
type dashboardBaseData struct {
	activeVisits             []*active.Visit
	todaysAttendance         []*active.Attendance
	allRooms                 []*facilityModels.Room
	activeGroups             []*active.Group
	allEducationGroups       []*educationModels.Group
	allActivityGroups        []*activitiesModels.Group
	activityGroupsByID       map[int64]*activitiesModels.Group
	activityCategories       int
	supervisorsToday         int
	visitsByGroupID          map[int64][]*active.Visit
	studentsWithActiveVisits map[int64]bool
	studentsWithAttendance   map[int64]bool
	studentsPresent          map[int64]bool
}

// dashboardRoomData holds room-related lookup maps
type dashboardRoomData struct {
	roomByID          map[int64]*facilityModels.Room
	roomCapacityTotal int
	occupiedRooms     map[int64]bool
	roomStudentsMap   map[int64]map[int64]struct{} // roomID -> set of unique student IDs
}

// dashboardGroupData holds group-related mappings
type dashboardGroupData struct {
	educationGroupRooms map[int64]bool  // room IDs that belong to educational groups
	studentHomeRoomMap  map[int64]int64 // studentID -> home room ID
}

// locationMetrics holds calculated location-based metrics
type locationMetrics struct {
	studentsOnPlayground  int
	studentsInGroupRooms  int
	studentsInHomeRoom    int
	studentsInIndoorRooms int
}

// fetchDashboardBaseData retrieves all raw data needed for dashboard analytics
func (s *service) fetchDashboardBaseData(ctx context.Context, today timezone.Date) (*dashboardBaseData, error) {
	data := &dashboardBaseData{
		studentsWithActiveVisits: make(map[int64]bool),
		studentsWithAttendance:   make(map[int64]bool),
		studentsPresent:          make(map[int64]bool),
		visitsByGroupID:          make(map[int64][]*active.Visit),
		activityGroupsByID:       make(map[int64]*activitiesModels.Group),
	}

	// Get active visits
	activeVisits, err := s.VisitRepo.FindActiveVisits(ctx)
	if err != nil {
		return nil, err
	}
	data.activeVisits = activeVisits

	// Build student-visit maps
	for _, visit := range activeVisits {
		data.studentsWithActiveVisits[visit.StudentID] = true
		data.visitsByGroupID[visit.ActiveGroupID] = append(data.visitsByGroupID[visit.ActiveGroupID], visit)
	}

	// Get today's attendance
	todaysAttendance, err := s.AttendanceRepo.FindForDate(ctx, today)
	if err != nil {
		return nil, err
	}
	data.todaysAttendance = todaysAttendance

	// Build attendance maps
	for _, record := range todaysAttendance {
		if record.CheckOutTime == nil {
			data.studentsWithAttendance[record.StudentID] = true
			data.studentsPresent[record.StudentID] = true
		}
	}

	// Add students with active visits to present set
	for studentID := range data.studentsWithActiveVisits {
		data.studentsPresent[studentID] = true
	}

	// Get all rooms
	allRooms, err := s.RoomRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.allRooms = allRooms

	// Get active groups
	activeGroups, err := s.GroupRepo.FindActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	data.activeGroups = activeGroups

	// Get education groups
	allEducationGroups, err := s.EducationGroupRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.allEducationGroups = allEducationGroups

	// Get activity groups (loaded once, used by name resolution, OGS-group
	// classification and current activities).
	// Non-critical: if this fails, dashboard still shows core metrics with
	// fallback names and an OGS-group count of zero — log it, never swallow it.
	allActivityGroups, err := s.ActivityGroupRepo.List(ctx, nil)
	if err != nil {
		s.getLogger().Warn("activity groups load failed, dashboard degrades to fallback names",
			"error", err.Error(),
		)
	} else {
		data.allActivityGroups = allActivityGroups
		for _, ag := range allActivityGroups {
			data.activityGroupsByID[ag.ID] = ag
		}
	}

	// Get activity categories count
	activityCategories, err := s.ActivityCatRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.activityCategories = len(activityCategories)

	// Get supervisors count
	supervisorsCount, err := s.countSupervisorsToday(ctx)
	if err != nil {
		return nil, err
	}
	data.supervisorsToday = supervisorsCount

	return data, nil
}

// countSupervisorsToday counts unique staff members who had any supervision today.
// Uses the repository's optimized query instead of loading all supervisors.
func (s *service) countSupervisorsToday(ctx context.Context) (int, error) {
	staffIDs, err := s.SupervisorRepo.GetStaffIDsWithSupervisionToday(ctx)
	if err != nil {
		return 0, err
	}
	return len(staffIDs), nil
}

// buildRoomLookupMaps creates room-related lookup structures
func (s *service) buildRoomLookupMaps(allRooms []*facilityModels.Room) *dashboardRoomData {
	data := &dashboardRoomData{
		roomByID:        make(map[int64]*facilityModels.Room),
		occupiedRooms:   make(map[int64]bool),
		roomStudentsMap: make(map[int64]map[int64]struct{}),
	}

	for _, room := range allRooms {
		data.roomByID[room.ID] = room
		if room.Capacity != nil && *room.Capacity > 0 {
			data.roomCapacityTotal += *room.Capacity
		}
	}

	return data
}

// loadStudentsWithGroups batch loads students for the given IDs. Consumers
// only build lookup maps from the result, so the order is irrelevant.
func (s *service) loadStudentsWithGroups(ctx context.Context, studentIDs []int64) ([]*userModels.Student, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	studentsByID, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	studentsWithGroups := make([]*userModels.Student, 0, len(studentsByID))
	for _, student := range studentsByID {
		studentsWithGroups = append(studentsWithGroups, student)
	}
	return studentsWithGroups, nil
}

// buildEducationGroupMaps creates group-related lookup structures
func (s *service) buildEducationGroupMaps(allEducationGroups []*educationModels.Group, studentsWithGroups []*userModels.Student) *dashboardGroupData {
	data := &dashboardGroupData{
		educationGroupRooms: make(map[int64]bool),
		studentHomeRoomMap:  make(map[int64]int64),
	}

	// Build education group rooms set
	buildEducationGroupRoomsSet(allEducationGroups, data)

	// Build student home room map
	buildStudentHomeRoomMap(studentsWithGroups, allEducationGroups, data)

	return data
}

// buildEducationGroupRoomsSet populates the set of room IDs belonging to education groups
func buildEducationGroupRoomsSet(allEducationGroups []*educationModels.Group, data *dashboardGroupData) {
	for _, eduGroup := range allEducationGroups {
		if eduGroup.RoomID != nil && *eduGroup.RoomID > 0 {
			data.educationGroupRooms[*eduGroup.RoomID] = true
		}
	}
}

// buildStudentHomeRoomMap creates a mapping of student IDs to their home room IDs
func buildStudentHomeRoomMap(studentsWithGroups []*userModels.Student, allEducationGroups []*educationModels.Group, data *dashboardGroupData) {
	// Pre-build group ID to room ID lookup for O(1) access
	groupToRoom := buildGroupToRoomLookup(allEducationGroups)

	for _, student := range studentsWithGroups {
		if student.GroupID == nil {
			continue
		}
		if roomID, ok := groupToRoom[*student.GroupID]; ok {
			data.studentHomeRoomMap[student.ID] = roomID
		}
	}
}

// buildGroupToRoomLookup creates a map from group ID to room ID
func buildGroupToRoomLookup(allEducationGroups []*educationModels.Group) map[int64]int64 {
	lookup := make(map[int64]int64)
	for _, eduGroup := range allEducationGroups {
		if eduGroup.RoomID != nil {
			lookup[eduGroup.ID] = *eduGroup.RoomID
		}
	}
	return lookup
}

// isOGSGroupTemplate reports whether a timetable template represents a
// Betreuungsgruppe (OGS group), i.e. a care block bound to one concrete
// education group.
//
// The binding MUST be read off the activities template. active.groups.group_id
// is a foreign key to activities.groups(id) (fk_active_groups_group), never to
// education.groups(id). Both tables draw from independent BIGSERIAL sequences,
// so looking a session's group_id up in an education-keyed map matches by
// numeric coincidence only — that was issue #2178.
//
// Both conditions are load-bearing:
//   - Type == care excludes an AG whose Zielgruppe happens to be a group
//     ("Fußball für die Bären"); such a template carries EducationGroupID too.
//   - EducationGroupID != nil excludes care blocks that serve no specific
//     group ("Mittagessen 1. Schicht", a Jahrgang-wide Hausaufgabenblock).
//
// Multi-target templates (activities.group_targets) need no extra handling:
// a "gruppe" target list always mirrors its first entry into the scalar
// EducationGroupID (normalizeDynamicTargets), and Group.validateEducationGroupTarget
// makes that scalar mandatory for the type.
func isOGSGroupTemplate(template *activitiesModels.Group) bool {
	return template != nil &&
		template.Type == activitiesModels.GroupTypeCare &&
		template.EducationGroupID != nil
}

// processActiveGroups calculates metrics from active groups.
// Returns total active count, OGS-only count, and unique students in rooms.
func processActiveGroups(activeGroups []*active.Group, visitsByGroupID map[int64][]*active.Visit, activityGroupsByID map[int64]*activitiesModels.Group, roomData *dashboardRoomData) (int, int, map[int64]struct{}) {
	ogsGroupsCount := 0
	uniqueStudentsInRoomsOverall := make(map[int64]struct{})

	for _, group := range activeGroups {
		roomData.occupiedRooms[group.RoomID] = true

		// Initialize room student set if not exists
		if roomData.roomStudentsMap[group.RoomID] == nil {
			roomData.roomStudentsMap[group.RoomID] = make(map[int64]struct{})
		}

		// Count unique students for this group
		if groupVisits, ok := visitsByGroupID[group.ID]; ok {
			for _, visit := range groupVisits {
				roomData.roomStudentsMap[group.RoomID][visit.StudentID] = struct{}{}
				uniqueStudentsInRoomsOverall[visit.StudentID] = struct{}{}
			}
		}

		// Check if this is an OGS/education group. Spontaneous sessions
		// (GroupID == nil, WP-B6) are never OGS-bound — they skip this
		// classification entirely.
		if templateID, ok := group.TemplateID(); ok {
			if isOGSGroupTemplate(activityGroupsByID[templateID]) {
				ogsGroupsCount++
			}
		}
	}

	return len(activeGroups), ogsGroupsCount, uniqueStudentsInRoomsOverall
}

// isPlaygroundRoom checks if a room is a playground/outdoor area
func isPlaygroundRoom(room *facilityModels.Room) bool {
	if room == nil || room.Category == nil {
		return false
	}
	switch *room.Category {
	case "Schulhof", "Playground", "school_yard":
		return true
	}
	return false
}

// calculateLocationMetrics calculates student location-based metrics
func (s *service) calculateLocationMetrics(roomData *dashboardRoomData, groupData *dashboardGroupData, activeVisits []*active.Visit, activeGroups []*active.Group) *locationMetrics {
	metrics := &locationMetrics{}

	// Process each room's student set
	for roomID, studentSet := range roomData.roomStudentsMap {
		s.processRoomForLocationMetrics(roomID, studentSet, roomData, groupData, metrics)
	}

	// Calculate students in indoor rooms (excluding playground)
	metrics.studentsInIndoorRooms = s.countStudentsInIndoorRooms(activeVisits, activeGroups, roomData)

	return metrics
}

// processRoomForLocationMetrics updates metrics based on a single room's students
func (s *service) processRoomForLocationMetrics(roomID int64, studentSet map[int64]struct{}, roomData *dashboardRoomData, groupData *dashboardGroupData, metrics *locationMetrics) {
	room, ok := roomData.roomByID[roomID]
	if !ok {
		return
	}

	uniqueStudentCount := len(studentSet)

	if isPlaygroundRoom(room) {
		metrics.studentsOnPlayground += uniqueStudentCount
	}

	if !groupData.educationGroupRooms[roomID] {
		return
	}

	metrics.studentsInGroupRooms += uniqueStudentCount
	metrics.studentsInHomeRoom += countStudentsInHomeRoom(studentSet, roomID, groupData.studentHomeRoomMap)
}

// countStudentsInHomeRoom counts how many students in the set are in their home room
func countStudentsInHomeRoom(studentSet map[int64]struct{}, roomID int64, studentHomeRoomMap map[int64]int64) int {
	count := 0
	for studentID := range studentSet {
		if homeRoomID, ok := studentHomeRoomMap[studentID]; ok && homeRoomID == roomID {
			count++
		}
	}
	return count
}

// countStudentsInIndoorRooms counts unique students in rooms excluding playground areas
func (s *service) countStudentsInIndoorRooms(activeVisits []*active.Visit, activeGroups []*active.Group, roomData *dashboardRoomData) int {
	// Build group ID to room lookup for O(1) access
	groupToRoom := buildActiveGroupRoomLookup(activeGroups, roomData)

	uniqueStudentsInRooms := make(map[int64]struct{})
	for _, visit := range activeVisits {
		if !visit.IsActive() {
			continue
		}
		if _, isIndoor := groupToRoom[visit.ActiveGroupID]; isIndoor {
			uniqueStudentsInRooms[visit.StudentID] = struct{}{}
		}
	}

	return len(uniqueStudentsInRooms)
}

// buildActiveGroupRoomLookup creates a lookup of active group IDs to non-playground room status
func buildActiveGroupRoomLookup(activeGroups []*active.Group, roomData *dashboardRoomData) map[int64]bool {
	lookup := make(map[int64]bool)
	for _, group := range activeGroups {
		if !group.IsActive() {
			continue
		}
		room, ok := roomData.roomByID[group.RoomID]
		if ok && !isPlaygroundRoom(room) {
			lookup[group.ID] = true
		}
	}
	return lookup
}

// buildRecentActivity builds the recent activity list
func buildRecentActivity(activeGroups []*active.Group, activityGroupsByID map[int64]*activitiesModels.Group, roomData *dashboardRoomData) []RecentActivity {
	recentActivity := []RecentActivity{}

	for _, group := range activeGroups {
		if len(recentActivity) >= 5 {
			break
		}

		if time.Since(group.StartTime) >= 30*time.Minute || !group.IsActive() {
			continue
		}

		groupName := resolveGroupName(group.GroupID, activityGroupsByID)
		roomName := resolveRoomName(group.RoomID, roomData.roomByID)

		// Count unique students
		visitCount := 0
		if studentSet, ok := roomData.roomStudentsMap[group.RoomID]; ok {
			visitCount = len(studentSet)
		}

		activity := RecentActivity{
			Type:      "group_start",
			GroupName: groupName,
			RoomName:  roomName,
			Count:     visitCount,
			Timestamp: group.StartTime,
		}
		recentActivity = append(recentActivity, activity)
	}

	return recentActivity
}

// buildCurrentActivities builds the current activities list
func buildCurrentActivities(allActivityGroups []*activitiesModels.Group, activeGroups []*active.Group, roomData *dashboardRoomData) []CurrentActivity {
	currentActivities := []CurrentActivity{}

	for _, actGroup := range allActivityGroups {
		if len(currentActivities) >= 5 {
			break
		}

		hasActiveSession, participantCount := findActiveSessionForActivity(actGroup.ID, activeGroups, roomData)
		if !hasActiveSession {
			continue
		}

		categoryName := "Sonstiges"
		if actGroup.Category != nil {
			categoryName = actGroup.Category.Name
		}

		status := determineActivityStatus(participantCount, actGroup.MaxParticipants)

		activity := CurrentActivity{
			ID:           actGroup.ID,
			Name:         actGroup.Name,
			Category:     categoryName,
			Participants: participantCount,
			MaxCapacity:  actGroup.MaxParticipants,
			Status:       status,
		}
		currentActivities = append(currentActivities, activity)
	}

	return currentActivities
}

// findActiveSessionForActivity checks if an activity has an active session and returns participant count.
// Spontaneous sessions (TemplateID ok == false) can never match since they
// have no parent template id to compare against.
func findActiveSessionForActivity(activityID int64, activeGroups []*active.Group, roomData *dashboardRoomData) (bool, int) {
	for _, group := range activeGroups {
		templateID, ok := group.TemplateID()
		if group.IsActive() && ok && templateID == activityID {
			participantCount := 0
			if studentSet, ok := roomData.roomStudentsMap[group.RoomID]; ok {
				participantCount = len(studentSet)
			}
			return true, participantCount
		}
	}
	return false, 0
}

// determineActivityStatus returns the status string based on capacity
func determineActivityStatus(participants, maxCapacity int) string {
	if participants >= maxCapacity {
		return "full"
	}
	if participants > int(float64(maxCapacity)*0.8) {
		return "ending_soon"
	}
	return "active"
}

// buildActiveGroupsSummary builds the active groups summary list
func buildActiveGroupsSummary(activeGroups []*active.Group, activityGroupsByID map[int64]*activitiesModels.Group, roomData *dashboardRoomData) []ActiveGroupInfo {
	summary := []ActiveGroupInfo{}

	for _, group := range activeGroups {
		if len(summary) >= 5 {
			break
		}
		if !group.IsActive() {
			continue
		}

		groupName, groupType := resolveGroupNameAndType(group.GroupID, activityGroupsByID)
		location := resolveRoomName(group.RoomID, roomData.roomByID)

		studentCount := 0
		if studentSet, ok := roomData.roomStudentsMap[group.RoomID]; ok {
			studentCount = len(studentSet)
		}

		groupInfo := ActiveGroupInfo{
			Name:         groupName,
			Type:         groupType,
			StudentCount: studentCount,
			Location:     location,
			Status:       "active",
		}
		summary = append(summary, groupInfo)
	}

	return summary
}

// resolveGroupName gets the display name for a group via the pre-loaded
// template map. Spontaneous sessions (groupID == nil, WP-B6) return a generic
// label since no template row exists to derive a name from.
//
// groupID is an activities.groups.id — never look it up in an education-keyed
// map (#2178).
func resolveGroupName(groupID *int64, activityGroupsByID map[int64]*activitiesModels.Group) string {
	if groupID == nil {
		return "Spontane Aktivität"
	}
	if ag, ok := activityGroupsByID[*groupID]; ok {
		return ag.Name
	}
	return fmt.Sprintf("Gruppe %d", *groupID)
}

// resolveGroupNameAndType gets the display name and type for a group.
// Both come from the session's activities template: care blocks bound to an
// education group are typed "ogs_group", everything else "activity".
// Spontaneous sessions (groupID == nil) are classified as "spontaneous".
func resolveGroupNameAndType(groupID *int64, activityGroupsByID map[int64]*activitiesModels.Group) (string, string) {
	if groupID == nil {
		return "Spontane Aktivität", "spontaneous"
	}
	ag, ok := activityGroupsByID[*groupID]
	if !ok {
		return fmt.Sprintf("Gruppe %d", *groupID), "activity"
	}
	if isOGSGroupTemplate(ag) {
		return ag.Name, "ogs_group"
	}
	return ag.Name, "activity"
}

// resolveRoomName gets the display name for a room
func resolveRoomName(roomID int64, roomByID map[int64]*facilityModels.Room) string {
	if room, ok := roomByID[roomID]; ok {
		return room.Name
	}
	return fmt.Sprintf("Raum %d", roomID)
}

// extractUniqueStudentIDs extracts unique student IDs from visits
func extractUniqueStudentIDs(visits []*active.Visit) []int64 {
	studentIDSet := make(map[int64]struct{})
	studentIDs := make([]int64, 0, len(visits))

	for _, visit := range visits {
		if _, exists := studentIDSet[visit.StudentID]; !exists {
			studentIDs = append(studentIDs, visit.StudentID)
			studentIDSet[visit.StudentID] = struct{}{}
		}
	}

	return studentIDs
}

// calculateStudentsInTransit counts students with attendance but no active visit
func calculateStudentsInTransit(studentsWithAttendance, studentsWithActiveVisits map[int64]bool) int {
	count := 0
	for studentID := range studentsWithAttendance {
		if !studentsWithActiveVisits[studentID] {
			count++
		}
	}
	return count
}
