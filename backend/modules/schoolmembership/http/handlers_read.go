package staff

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// listStaffFilters holds the parsed query parameters of the directory read.
type listStaffFilters struct {
	firstName    string
	lastName     string
	teachersOnly bool
	filterByRole string
}

func parseListStaffFilters(r *http.Request) listStaffFilters {
	return listStaffFilters{
		firstName:    r.URL.Query().Get("first_name"),
		lastName:     r.URL.Query().Get("last_name"),
		teachersOnly: r.URL.Query().Get("teachers_only") == "true",
		filterByRole: r.URL.Query().Get("role"),
	}
}

func matchesNameFilter(person *Person, firstName, lastName string) bool {
	if firstName != "" && !containsFold(person.FirstName, firstName) {
		return false
	}
	if lastName != "" && !containsFold(person.LastName, lastName) {
		return false
	}
	return true
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// passesRoleFilter checks the optional ?role= filter against the person's
// account. A person without an account never matches a role filter.
func (rs *Resource) passesRoleFilter(ctx context.Context, person *Person, role string) bool {
	if role == "" {
		return true
	}
	if person.AccountID == nil {
		return false
	}
	return rs.runtime.AccountHasRole(ctx, *person.AccountID, role)
}

// listStaff lists the staff directory. Every foreign lookup is batched, so
// the response costs a fixed number of round trips regardless of staff count.
func (rs *Resource) listStaff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filters := parseListStaffFilters(r)

	members, err := rs.membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	teachers, err := rs.teacherIndex(ctx, staffIDsOf(members))
	if err != nil {
		rs.internal(w, r, err)
		return
	}

	persons, err := rs.personIndex(ctx, members)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	data := rs.loadDirectory(ctx, accountIDsOf(persons))
	access := rs.fieldAccess(ctx)

	responses := make([]any, 0, len(members))
	for _, staff := range members {
		person := persons[staff.PersonID]
		if person == nil {
			continue
		}
		if !rs.passesRoleFilter(ctx, person, filters.filterByRole) {
			continue
		}
		if !matchesNameFilter(person, filters.firstName, filters.lastName) {
			continue
		}
		teacher, isTeacher := teachers[staff.ID]
		if filters.teachersOnly && !isTeacher {
			continue
		}
		var profile *schoolmembership.Teacher
		if isTeacher {
			profile = &teacher
		}
		responses = append(responses, buildResponse(access, staff, person, profile, data.enrich(staff, person, access.personnel)))
	}

	rs.respond(w, r, http.StatusOK, responses, "Staff members retrieved successfully")
}

type documentDirectoryEntry struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

// listDocumentDirectory exposes only staff identities to document-capable
// roles. It intentionally omits directory, attendance, and account data.
func (rs *Resource) listDocumentDirectory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	members, err := rs.membership.ListStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	persons, err := rs.personIndex(ctx, members)
	if err != nil {
		rs.internal(w, r, err)
		return
	}

	entries := make([]documentDirectoryEntry, 0, len(members))
	for _, staff := range members {
		person := persons[staff.PersonID]
		if person == nil {
			continue
		}
		entries = append(entries, documentDirectoryEntry{
			ID:        staff.ID,
			Name:      person.FirstName + " " + person.LastName,
			FirstName: person.FirstName,
			LastName:  person.LastName,
		})
	}
	rs.runtime.RetryQueuedDocumentCleanups(ctx, "directory")
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })

	rs.respond(w, r, http.StatusOK, entries, "Document staff directory retrieved successfully")
}

// getStaff returns one staff member, as a teacher response when a teacher
// profile exists.
func (rs *Resource) getStaff(w http.ResponseWriter, r *http.Request) {
	staff, ok := rs.parseAndFindStaff(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	person, err := rs.personFor(ctx, staff)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	access := rs.fieldAccess(ctx)

	accountIDs := []int64{}
	if person != nil && person.AccountID != nil {
		accountIDs = append(accountIDs, *person.AccountID)
	}
	data := rs.loadDirectory(ctx, accountIDs).enrich(staff, person, access.personnel)

	if teacher := rs.teacherByStaff(ctx, staff.ID); teacher != nil {
		rs.respond(w, r, http.StatusOK, buildTeacherResponse(access, staff, person, *teacher, data), "Teacher retrieved successfully")
		return
	}
	rs.respond(w, r, http.StatusOK, buildStaffResponse(access, staff, person, false, data), "Staff member retrieved successfully")
}

// getFinancialProfile returns only the identity needed to operate the
// staff:financial Stammdaten section. It deliberately excludes the generic
// profile's notes, RFID, account, presence, and absence data.
func (rs *Resource) getFinancialProfile(w http.ResponseWriter, r *http.Request) {
	rs.getMinimalStaffProfile(w, r, "Financial staff profile retrieved successfully")
}

// getDocumentProfile returns only the staff identity needed to operate the
// documents tab. Dedicated health-document users must not need unrelated
// directory permissions merely to identify the profile they may access.
func (rs *Resource) getDocumentProfile(w http.ResponseWriter, r *http.Request) {
	rs.getMinimalStaffProfile(w, r, "Document staff profile retrieved successfully")
}

func (rs *Resource) getMinimalStaffProfile(w http.ResponseWriter, r *http.Request, message string) {
	staff, ok := rs.parseAndFindStaff(w, r)
	if !ok {
		return
	}
	person, err := rs.personFor(r.Context(), staff)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	if person == nil {
		rs.failure(w, r, FailureNotFound, errors.New(msgStaffNotFound), "not_found")
		return
	}
	rs.respond(w, r, http.StatusOK, map[string]any{
		"id":        staff.ID,
		"name":      person.FirstName + " " + person.LastName,
		"firstName": person.FirstName,
		"lastName":  person.LastName,
	}, message)
}

// serveStaffAvatar streams the avatar of the account behind a staff member.
// Every miss along the chain is a plain 404, as before — the image tag on the
// staff screens has no other error handling.
func (rs *Resource) serveStaffAvatar(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	staff, findErr := rs.membership.FindStaff(ctx, id)
	if findErr != nil {
		http.NotFound(w, r)
		return
	}
	person, personErr := rs.personFor(ctx, staff)
	if personErr != nil {
		http.NotFound(w, r)
		return
	}
	if person == nil || person.AccountID == nil {
		http.NotFound(w, r)
		return
	}
	avatars, avatarErr := rs.runtime.AccountAvatars(ctx, []int64{*person.AccountID})
	if avatarErr != nil || avatars[*person.AccountID] == "" {
		http.NotFound(w, r)
		return
	}
	rs.runtime.ServeAvatar(w, r, avatars[*person.AccountID])
}
