package students

import (
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// studentListParams holds all query parameters for student listing
type studentListParams struct {
	schoolClass  string
	guardianName string
	firstName    string
	lastName     string
	location     string
	groupID      int64
	search       string
	page         int
	pageSize     int
}

// studentAccessContext holds access control information for student listing
type studentAccessContext struct {
	isAdmin    bool
	myGroupIDs map[int64]struct{}
}

// parseStudentListParams extracts query parameters from the request
func parseStudentListParams(r *http.Request) *studentListParams {
	params := &studentListParams{
		schoolClass:  r.URL.Query().Get("school_class"),
		guardianName: r.URL.Query().Get("guardian_name"),
		firstName:    r.URL.Query().Get("first_name"),
		lastName:     r.URL.Query().Get("last_name"),
		location:     r.URL.Query().Get("location"),
		search:       r.URL.Query().Get("search"),
	}

	// Parse group ID if provided
	if groupIDStr := r.URL.Query().Get("group_id"); groupIDStr != "" {
		if groupID, err := strconv.ParseInt(groupIDStr, 10, 64); err == nil {
			params.groupID = groupID
		}
	}

	// Parse pagination
	params.page, params.pageSize = common.ParsePagination(r)

	return params
}

// hasPersonFilters returns true if any person-based filters are active
func (p *studentListParams) hasPersonFilters() bool {
	return p.search != "" || p.firstName != "" || p.lastName != "" || p.location != ""
}

// buildQueryOptions creates query options from parameters
func (p *studentListParams) buildQueryOptions() *base.QueryOptions {
	queryOptions := base.NewQueryOptions()
	filter := base.NewFilter()

	if p.schoolClass != "" {
		filter.ILike("school_class", "%"+p.schoolClass+"%")
	}
	if p.guardianName != "" {
		filter.ILike("guardian_name", "%"+p.guardianName+"%")
	}

	// Add pagination only if no person-based filters
	if !p.hasPersonFilters() {
		queryOptions.WithPagination(p.page, p.pageSize)
	}
	queryOptions.Filter = filter

	return queryOptions
}

// buildCountFilter creates a filter for counting records
func (p *studentListParams) buildCountFilter() *base.Filter {
	filter := base.NewFilter()
	if p.schoolClass != "" {
		filter.ILike("school_class", "%"+p.schoolClass+"%")
	}
	if p.guardianName != "" {
		filter.ILike("guardian_name", "%"+p.guardianName+"%")
	}
	return filter
}

// buildCountOptions creates query options for counting records
func (p *studentListParams) buildCountOptions() *base.QueryOptions {
	countOptions := base.NewQueryOptions()
	countOptions.Filter = p.buildCountFilter()
	return countOptions
}

// determineStudentAccess determines access level and group IDs for the current user
// Uses tenant context for GDPR location permission check
func (rs *Resource) determineStudentAccess(r *http.Request) *studentAccessContext {
	ctx := &studentAccessContext{
		isAdmin: tenant.HasLocationPermission(r.Context()), // GDPR: location:read permission determines full access
	}

	if !ctx.isAdmin {
		if staff, err := rs.UserContextService.GetCurrentStaff(r.Context()); err == nil && staff != nil {
			if educationGroups, err := rs.UserContextService.GetMyGroups(r.Context()); err == nil {
				ctx.myGroupIDs = make(map[int64]struct{}, len(educationGroups))
				for _, eduGroup := range educationGroups {
					ctx.myGroupIDs[eduGroup.ID] = struct{}{}
				}
			}
		}
	}

	return ctx
}

// hasFullAccessToStudent checks if user has full access to a specific student
func (ac *studentAccessContext) hasFullAccessToStudent(student *users.Student) bool {
	if ac.isAdmin {
		return true
	}
	if student.GroupID != nil && ac.myGroupIDs != nil {
		_, ok := ac.myGroupIDs[*student.GroupID]
		return ok
	}
	return false
}

// collectIDsFromStudents extracts IDs needed for bulk loading
func collectIDsFromStudents(students []*users.Student) (studentIDs, personIDs, groupIDs []int64) {
	studentIDs = make([]int64, 0, len(students))
	personIDs = make([]int64, 0, len(students))
	groupIDSet := make(map[int64]struct{})

	for _, student := range students {
		studentIDs = append(studentIDs, student.ID)
		personIDs = append(personIDs, student.PersonID)
		if student.GroupID != nil {
			groupIDSet[*student.GroupID] = struct{}{}
		}
	}

	groupIDs = make([]int64, 0, len(groupIDSet))
	for groupID := range groupIDSet {
		groupIDs = append(groupIDs, groupID)
	}

	return studentIDs, personIDs, groupIDs
}

// matchesSearchFilter checks if a student matches the search term
func matchesSearchFilter(person *users.Person, studentID int64, search string) bool {
	if search == "" {
		return true
	}

	studentIDStr := strconv.FormatInt(studentID, 10)
	fullName := person.FirstName + " " + person.LastName

	return containsIgnoreCase(person.FirstName, search) ||
		containsIgnoreCase(person.LastName, search) ||
		containsIgnoreCase(studentIDStr, search) ||
		containsIgnoreCase(fullName, search)
}

// matchesNameFilters checks if a student matches the name filters
func matchesNameFilters(person *users.Person, firstName, lastName string) bool {
	if firstName != "" && !containsIgnoreCase(person.FirstName, firstName) {
		return false
	}
	if lastName != "" && !containsIgnoreCase(person.LastName, lastName) {
		return false
	}
	return true
}

// matchesLocationFilter checks if a student matches the location filter
func matchesLocationFilter(location, studentLocation string, hasFullAccess bool) bool {
	if location == "" {
		return true
	}
	if !hasFullAccess {
		return true
	}
	if location == "Unknown" {
		return true
	}
	return studentLocation == location
}

// applyInMemoryPagination applies pagination to an already-filtered slice
func applyInMemoryPagination(responses []StudentResponse, page, pageSize int) ([]StudentResponse, int) {
	totalCount := len(responses)

	startIndex := (page - 1) * pageSize
	endIndex := startIndex + pageSize

	// Ensure bounds are valid
	if startIndex > len(responses) {
		startIndex = len(responses)
	}
	if endIndex > len(responses) {
		endIndex = len(responses)
	}

	return responses[startIndex:endIndex], totalCount
}
