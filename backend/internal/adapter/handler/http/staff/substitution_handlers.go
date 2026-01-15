package staff

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// SubstitutionInfo represents a single substitution with transfer indicator
type SubstitutionInfo struct {
	ID         int64            `json:"id"`
	GroupID    int64            `json:"group_id"`
	GroupName  string           `json:"group_name,omitempty"`
	IsTransfer bool             `json:"is_transfer"`
	StartDate  string           `json:"start_date"`
	EndDate    string           `json:"end_date"`
	Group      *education.Group `json:"group,omitempty"`
}

// StaffWithSubstitutionStatus represents a staff member with their substitution status
type StaffWithSubstitutionStatus struct {
	*StaffResponse
	IsSubstituting    bool               `json:"is_substituting"`
	SubstitutionCount int                `json:"substitution_count"`
	Substitutions     []SubstitutionInfo `json:"substitutions,omitempty"`
	CurrentGroup      *education.Group   `json:"current_group,omitempty"`
	RegularGroup      *education.Group   `json:"regular_group,omitempty"`
	TeacherID         int64              `json:"teacher_id,omitempty"`
	Specialization    string             `json:"specialization,omitempty"`
	Role              string             `json:"role,omitempty"`
	Qualifications    string             `json:"qualifications,omitempty"`
}

// getStaffSubstitutions handles getting substitutions for a staff member
func (rs *Resource) getStaffSubstitutions(w http.ResponseWriter, r *http.Request) {
	staff, ok := rs.parseAndGetStaff(w, r)
	if !ok {
		return
	}

	if rs.EducationService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("education service not available")))
		return
	}

	substitutions, err := rs.EducationService.GetStaffSubstitutions(r.Context(), staff.ID, false)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, substitutions, "Staff substitutions retrieved successfully")
}

// getAvailableForSubstitution handles getting staff available for substitution with their current status
func (rs *Resource) getAvailableForSubstitution(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	searchTerm := r.URL.Query().Get("search")

	date := time.Now()
	if dateStr != "" {
		if parsedDate, err := time.Parse(common.DateFormatISO, dateStr); err == nil {
			date = parsedDate
		}
	}

	staff, err := rs.PersonService.ListStaff(r.Context(), nil)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	substitutingStaffMap := rs.buildSubstitutionMap(r.Context(), date)
	results := rs.filterAndBuildStaffResults(r.Context(), staff, substitutingStaffMap, searchTerm)

	common.Respond(w, r, http.StatusOK, results, "Available staff for substitution retrieved successfully")
}

// buildSubstitutionInfoList creates substitution info from active substitutions
func buildSubstitutionInfoList(subs []*education.GroupSubstitution) []SubstitutionInfo {
	result := make([]SubstitutionInfo, 0, len(subs))
	for _, sub := range subs {
		info := SubstitutionInfo{
			ID:         sub.ID,
			GroupID:    sub.GroupID,
			IsTransfer: sub.Duration() == 1,
			StartDate:  sub.StartDate.Format(common.DateFormatISO),
			EndDate:    sub.EndDate.Format(common.DateFormatISO),
		}
		if sub.Group != nil {
			info.GroupName = sub.Group.Name
			info.Group = sub.Group
		}
		result = append(result, info)
	}
	return result
}

// buildStaffSubstitutionStatus creates a staff status entry with substitution data
func (rs *Resource) buildStaffSubstitutionStatus(
	ctx context.Context,
	staff *users.Staff,
	teacher *users.Teacher,
	subs []*education.GroupSubstitution,
) StaffWithSubstitutionStatus {
	staffResp := newStaffResponse(staff, false)
	result := StaffWithSubstitutionStatus{
		StaffResponse:     &staffResp,
		IsSubstituting:    len(subs) > 0,
		SubstitutionCount: len(subs),
		Substitutions:     []SubstitutionInfo{},
		TeacherID:         teacher.ID,
		Specialization:    teacher.Specialization,
		Role:              teacher.Role,
		Qualifications:    teacher.Qualifications,
	}

	if len(subs) > 0 {
		result.Substitutions = buildSubstitutionInfoList(subs)
		if subs[0].Group != nil {
			result.CurrentGroup = subs[0].Group
		}
	}

	// Find regular group for this teacher
	if rs.EducationService != nil {
		groups, err := rs.EducationService.GetTeacherGroups(ctx, teacher.ID)
		if err == nil && len(groups) > 0 {
			result.RegularGroup = groups[0]
		}
	}

	return result
}

// buildSubstitutionMap creates a map of staff IDs to their active substitutions
func (rs *Resource) buildSubstitutionMap(ctx context.Context, date time.Time) map[int64][]*education.GroupSubstitution {
	result := make(map[int64][]*education.GroupSubstitution)
	if rs.EducationService == nil {
		return result
	}

	activeSubstitutions, _ := rs.EducationService.GetActiveSubstitutions(ctx, date)
	for _, sub := range activeSubstitutions {
		result[sub.SubstituteStaffID] = append(result[sub.SubstituteStaffID], sub)
	}
	return result
}

// filterAndBuildStaffResults filters staff and builds response entries
func (rs *Resource) filterAndBuildStaffResults(
	ctx context.Context,
	staff []*users.Staff,
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) []StaffWithSubstitutionStatus {
	var results []StaffWithSubstitutionStatus

	for _, s := range staff {
		result := rs.processStaffForSubstitution(ctx, s, subsMap, searchTerm)
		if result != nil {
			results = append(results, *result)
		}
	}
	return results
}

// processStaffForSubstitution processes a single staff member for the substitution list
func (rs *Resource) processStaffForSubstitution(
	ctx context.Context,
	s *users.Staff,
	subsMap map[int64][]*education.GroupSubstitution,
	searchTerm string,
) *StaffWithSubstitutionStatus {
	teacher, err := rs.PersonService.GetTeacherByStaffID(ctx, s.ID)
	if err != nil || teacher == nil {
		return nil
	}

	rs.ensurePersonLoaded(ctx, s)

	if s.Person != nil && !matchesSearchTerm(s.Person, searchTerm) {
		return nil
	}

	subs := subsMap[s.ID]
	result := rs.buildStaffSubstitutionStatus(ctx, s, teacher, subs)
	return &result
}

// ensurePersonLoaded loads person data if not already loaded
func (rs *Resource) ensurePersonLoaded(ctx context.Context, s *users.Staff) {
	if s.Person == nil && s.PersonID > 0 {
		if person, err := rs.PersonService.Get(ctx, s.PersonID); err == nil {
			s.Person = person
		}
	}
}

// matchesSearchTerm checks if staff member matches the search filter
func matchesSearchTerm(person *users.Person, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	return containsIgnoreCase(person.FirstName, searchTerm) ||
		containsIgnoreCase(person.LastName, searchTerm)
}

// containsIgnoreCase checks if a string contains another string, ignoring case
func containsIgnoreCase(s, substr string) bool {
	s, substr = strings.ToLower(s), strings.ToLower(substr)
	return strings.Contains(s, substr)
}
