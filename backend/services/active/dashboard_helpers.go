package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// dashboardBaseData holds the raw data fetched for dashboard analytics
type dashboardBaseData struct {
	activeVisits             []*active.Visit
	todaysAttendance         []*active.Attendance
	allRooms                 []*facilityModels.Room
	activeGroups             []*active.Group
	allEducationGroups       []*educationModels.Group
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
	educationGroupsMap  map[int64]bool
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
func (s *service) fetchDashboardBaseData(ctx context.Context, today time.Time) (*dashboardBaseData, error) {
	data := &dashboardBaseData{
		studentsWithActiveVisits: make(map[int64]bool),
		studentsWithAttendance:   make(map[int64]bool),
		studentsPresent:          make(map[int64]bool),
		visitsByGroupID:          make(map[int64][]*active.Visit),
	}

	// Get active visits
	activeVisits, err := s.visitRepo.FindActiveVisits(ctx)
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
	todaysAttendance, err := s.attendanceRepo.FindForDate(ctx, today)
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
	allRooms, err := s.roomRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.allRooms = allRooms

	// Get active groups
	activeGroups, err := s.groupRepo.FindActiveGroups(ctx)
	if err != nil {
		return nil, err
	}
	data.activeGroups = activeGroups

	// Get education groups
	allEducationGroups, err := s.educationGroupRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.allEducationGroups = allEducationGroups

	// Get activity categories count
	activityCategories, err := s.activityCatRepo.List(ctx, nil)
	if err != nil {
		return nil, err
	}
	data.activityCategories = len(activityCategories)

	// Get supervisors count
	supervisorsCount, err := s.countSupervisorsToday(ctx, today)
	if err != nil {
		return nil, err
	}
	data.supervisorsToday = supervisorsCount

	return data, nil
}

// countSupervisorsToday counts unique supervisors active today
func (s *service) countSupervisorsToday(ctx context.Context, today time.Time) (int, error) {
	supervisors, err := s.supervisorRepo.List(ctx, nil)
	if err != nil {
		return 0, err
	}

	supervisorMap := make(map[int64]bool)
	now := time.Now()
	for _, supervisor := range supervisors {
		if supervisor.IsActive() || (supervisor.StartDate.After(today) && supervisor.StartDate.Before(now)) {
			supervisorMap[supervisor.StaffID] = true
		}
	}
	return len(supervisorMap), nil
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

// loadStudentsWithGroups batch loads students for the given IDs
func (s *service) loadStudentsWithGroups(ctx context.Context, studentIDs []int64) ([]*userModels.Student, error) {
	if len(studentIDs) == 0 {
		return nil, nil
	}

	var studentsWithGroups []*userModels.Student
	err := s.db.NewSelect().
		Model(&studentsWithGroups).
		ModelTableExpr(`users.students AS "student"`).
		Where(`"student".id IN (?)`, bun.In(studentIDs)).
		Scan(ctx)

	return studentsWithGroups, err
}

// buildEducationGroupMaps creates group-related lookup structures
func (s *service) buildEducationGroupMaps(ctx context.Context, activeGroups []*active.Group, allEducationGroups []*educationModels.Group, studentsWithGroups []*userModels.Student) (*dashboardGroupData, error) {
	data := &dashboardGroupData{
		educationGroupsMap:  make(map[int64]bool),
		educationGroupRooms: make(map[int64]bool),
		studentHomeRoomMap:  make(map[int64]int64),
	}

	// Load education groups for active groups
	s.loadEducationGroupsForActive(ctx, activeGroups, data)

	// Build education group rooms set
	buildEducationGroupRoomsSet(allEducationGroups, data)

	// Build student home room map
	buildStudentHomeRoomMap(studentsWithGroups, allEducationGroups, data)

	return data, nil
}

// loadEducationGroupsForActive loads and marks education groups that have active sessions
func (s *service) loadEducationGroupsForActive(ctx context.Context, activeGroups []*active.Group, data *dashboardGroupData) {
	groupIDs := collectGroupIDs(activeGroups)
	if len(groupIDs) == 0 {
		return
	}

	var eduGroups []*educationModels.Group
	err := s.db.NewSelect().
		Model(&eduGroups).
		ModelTableExpr(`education.groups AS "group"`).
		Where(`"group".id IN (?)`, bun.In(groupIDs)).
		Scan(ctx)
	if err != nil {
		return
	}

	for _, eg := range eduGroups {
		data.educationGroupsMap[eg.ID] = true
	}
}

// collectGroupIDs extracts group IDs from active groups
func collectGroupIDs(activeGroups []*active.Group) []int64 {
	groupIDs := make([]int64, 0, len(activeGroups))
	for _, group := range activeGroups {
		groupIDs = append(groupIDs, group.GroupID)
	}
	return groupIDs
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

// processActiveGroups calculates metrics from active groups
func (s *service) processActiveGroups(activeGroups []*active.Group, visitsByGroupID map[int64][]*active.Visit, groupData *dashboardGroupData, roomData *dashboardRoomData) (int, int, map[int64]struct{}) {
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

		// Check if this is an OGS group
		if groupData.educationGroupsMap[group.GroupID] {
			ogsGroupsCount++
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
func (s *service) buildRecentActivity(ctx context.Context, activeGroups []*active.Group, roomData *dashboardRoomData) []RecentActivity {
	recentActivity := []RecentActivity{}

	for i, group := range activeGroups {
		if i >= 3 { // Limit to 3 recent activities
			break
		}

		if time.Since(group.StartTime) >= 30*time.Minute || !group.IsActive() {
			continue
		}

		groupName := s.resolveGroupName(ctx, group.GroupID)
		roomName := s.resolveRoomName(group.RoomID, roomData.roomByID)

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
func (s *service) buildCurrentActivities(ctx context.Context, activeGroups []*active.Group, roomData *dashboardRoomData) []CurrentActivity {
	currentActivities := []CurrentActivity{}

	activityGroups, err := s.activityGroupRepo.List(ctx, nil)
	if err != nil {
		return currentActivities
	}

	for i, actGroup := range activityGroups {
		if i >= 2 { // Limit to 2 current activities
			break
		}

		hasActiveSession, participantCount := s.findActiveSessionForActivity(actGroup.ID, activeGroups, roomData)
		if !hasActiveSession {
			continue
		}

		categoryName := "Sonstiges"
		if actGroup.Category != nil {
			categoryName = actGroup.Category.Name
		}

		status := s.determineActivityStatus(participantCount, actGroup.MaxParticipants)

		activity := CurrentActivity{
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

// findActiveSessionForActivity checks if an activity has an active session and returns participant count
func (s *service) findActiveSessionForActivity(activityID int64, activeGroups []*active.Group, roomData *dashboardRoomData) (bool, int) {
	for _, group := range activeGroups {
		if group.IsActive() && group.GroupID == activityID {
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
func (s *service) determineActivityStatus(participants, maxCapacity int) string {
	if participants >= maxCapacity {
		return "full"
	}
	if participants > int(float64(maxCapacity)*0.8) {
		return "ending_soon"
	}
	return "active"
}

// buildActiveGroupsSummary builds the active groups summary list
func (s *service) buildActiveGroupsSummary(ctx context.Context, activeGroups []*active.Group, roomData *dashboardRoomData) []ActiveGroupInfo {
	summary := []ActiveGroupInfo{}

	for i, group := range activeGroups {
		if i >= 2 || !group.IsActive() { // Limit to 2 groups
			break
		}

		groupName, groupType := s.resolveGroupNameAndType(ctx, group.GroupID)
		location := s.resolveRoomName(group.RoomID, roomData.roomByID)

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

// resolveGroupName gets the display name for a group
func (s *service) resolveGroupName(ctx context.Context, groupID int64) string {
	if actGroup, err := s.activityGroupRepo.FindByID(ctx, groupID); err == nil && actGroup != nil {
		return actGroup.Name
	}
	if eduGroup, err := s.educationGroupRepo.FindByID(ctx, groupID); err == nil && eduGroup != nil {
		return eduGroup.Name
	}
	return fmt.Sprintf("Gruppe %d", groupID)
}

// resolveGroupNameAndType gets the display name and type for a group
func (s *service) resolveGroupNameAndType(ctx context.Context, groupID int64) (string, string) {
	if eduGroup, err := s.educationGroupRepo.FindByID(ctx, groupID); err == nil && eduGroup != nil {
		return eduGroup.Name, "ogs_group"
	}
	return fmt.Sprintf("Gruppe %d", groupID), "activity"
}

// resolveRoomName gets the display name for a room
func (s *service) resolveRoomName(roomID int64, roomByID map[int64]*facilityModels.Room) string {
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
