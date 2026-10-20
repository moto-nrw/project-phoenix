package staff

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// employmentFormer is the value of ?employment= that lists former
// employments instead of the current staff (#3403).
const employmentFormer = "former"

// formerStaffResponse is one former employment in the Ehemalige filter of
// the staff list. The period is the membership's time in moto, from the day
// it was created to the day the person left, the same period the invitation
// dialog names (#3402).
type formerStaffResponse struct {
	ID        int64         `json:"id,string"`
	FirstName string        `json:"first_name"`
	LastName  string        `json:"last_name"`
	StartedOn calendar.Date `json:"started_on"`
	EndedOn   calendar.Date `json:"ended_on"`
}

// listFormerStaff answers ?employment=former: former employments stay
// findable where staff is searched (ADR 0021, decision 7). The read takes
// retired memberships along on purpose and stays inside the caller's school
// transaction; the name filters apply as on the current list.
func (rs *Resource) listFormerStaff(w http.ResponseWriter, r *http.Request, filters listStaffFilters) {
	ctx := r.Context()
	members, err := rs.membership.ListStaff(ctx, schoolmembership.StaffFilter{
		IncludeDeleted: true, IncludeInactiveGuests: true, MembershipOnly: true,
	})
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	former := formerEmployments(members)
	persons, err := rs.personIndex(ctx, former)
	if err != nil {
		rs.internal(w, r, err)
		return
	}
	teachers := map[int64]schoolmembership.Teacher{}
	if filters.teachersOnly {
		teachers, err = rs.teacherIndex(ctx, staffIDsOf(former), true)
		if err != nil {
			rs.internal(w, r, err)
			return
		}
	}

	responses := make([]formerStaffResponse, 0, len(former))
	for _, staff := range former {
		person := persons[staff.PersonID]
		if person == nil ||
			schoolmembership.ExternalCaregiver(staff.IsGuest, true, person.AccountID != nil) ||
			!rs.passesRoleFilter(ctx, person, filters.filterByRole) ||
			!matchesNameFilter(person, filters.firstName, filters.lastName) {
			continue
		}
		if filters.teachersOnly {
			if _, isTeacher := teachers[staff.ID]; !isTeacher {
				continue
			}
		}
		responses = append(responses, formerStaffResponse{
			ID: staff.ID, FirstName: person.FirstName, LastName: person.LastName,
			StartedOn: calendar.DateFromTime(staff.CreatedAt), EndedOn: calendar.DateFromTime(*staff.DeletedAt),
		})
	}
	rs.respond(w, r, http.StatusOK, responses, "Former staff members retrieved successfully")
}

// formerEmployments keeps, per person, the latest retired membership the
// automatic deletion of staff has not erased, the one a Wiederaufnahme would
// continue. A person with a live membership is current staff and appears
// there only, so a resumed person is never listed twice. External
// caregivers without an account are no employment and stay out, as they do
// on the current list. The handler applies that check after loading people,
// because a guest profile with a moto account is not an external caregiver.
func formerEmployments(members []schoolmembership.Staff) []schoolmembership.Staff {
	live := make(map[int64]bool, len(members))
	latest := make(map[int64]schoolmembership.Staff, len(members))
	order := make([]int64, 0, len(members))
	for _, member := range members {
		if !member.IsDeleted() {
			live[member.PersonID] = true
			continue
		}
		if member.PersonalDataErasedAt != nil {
			continue
		}
		current, seen := latest[member.PersonID]
		if !seen {
			order = append(order, member.PersonID)
		}
		if !seen || member.ID > current.ID {
			latest[member.PersonID] = member
		}
	}
	result := make([]schoolmembership.Staff, 0, len(order))
	for _, personID := range order {
		if !live[personID] {
			result = append(result, latest[personID])
		}
	}
	return result
}
