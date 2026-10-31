package careplanhttp

import (
	errors "errors"
	http "net/http"
	strconv "strconv"
	strings "strings"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	careplan "github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CourseWaitlistEntryResponse is one child waiting for an AG (#4767).
type CourseWaitlistEntryResponse struct {
	RequestID   string    `json:"request_id"`
	StudentID   string    `json:"student_id"`
	StudentName string    `json:"student_name"`
	Rank        int       `json:"rank"`
	SubmittedAt time.Time `json:"submitted_at"`
}

// CourseWaitlistResponse is one AG with its places and its ranked queue.
// ParticipantLimit and FreeSlots are null for an AG without a limit.
// SlotFreeForNext marks the moment the team confirms the first child.
type CourseWaitlistResponse struct {
	ActivityGroupID  string                        `json:"activity_group_id"`
	Name             string                        `json:"name"`
	ParticipantLimit *int                          `json:"participant_limit"`
	Participants     int                           `json:"participants"`
	FreeSlots        *int                          `json:"free_slots"`
	SlotFreeForNext  bool                          `json:"slot_free_for_next"`
	Entries          []CourseWaitlistEntryResponse `json:"entries"`
}

// listCourseWaitlists answers GET .../offering-change-requests/course-waitlists,
// optionally narrowed to one AG with ?activity_group_id=.
func (rs *Resource) listCourseWaitlists(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingChangeService == nil || rs.PeopleDirectory == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("course waitlist services not configured")))
		return
	}
	activityGroupID, err := optionalPositiveQueryID(r, "activity_group_id")
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	waitlists, err := rs.OfferingChangeService.CourseWaitlists(r.Context(), activityGroupID, calendar.DateFromTime(rs.Now()))
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	names, err := rs.courseWaitlistStudentNames(r, waitlists)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, courseWaitlistResponses(waitlists, names), "Course waitlists loaded")
}

func (rs *Resource) courseWaitlistStudentNames(r *http.Request, waitlists []careplan.CourseWaitlist) (map[int64]string, error) {
	ids := make([]int64, 0)
	for _, waitlist := range waitlists {
		for _, entry := range waitlist.Entries {
			ids = append(ids, entry.StudentID)
		}
	}
	names := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return names, nil
	}
	rows, err := rs.PeopleDirectory.ListStudentNamesByID(r.Context(), ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.StudentID] = strings.TrimSpace(row.FirstName + " " + row.LastName)
	}
	return names, nil
}

func courseWaitlistResponses(waitlists []careplan.CourseWaitlist, names map[int64]string) []CourseWaitlistResponse {
	out := make([]CourseWaitlistResponse, 0, len(waitlists))
	for _, waitlist := range waitlists {
		entries := make([]CourseWaitlistEntryResponse, 0, len(waitlist.Entries))
		for _, entry := range waitlist.Entries {
			entries = append(entries, CourseWaitlistEntryResponse{
				RequestID: strconv.FormatInt(entry.RequestID, 10), StudentID: strconv.FormatInt(entry.StudentID, 10),
				StudentName: names[entry.StudentID], Rank: entry.Rank, SubmittedAt: entry.SubmittedAt,
			})
		}
		out = append(out, CourseWaitlistResponse{
			ActivityGroupID: strconv.FormatInt(waitlist.ActivityGroupID, 10), Name: waitlist.Name,
			ParticipantLimit: waitlist.ParticipantLimit, Participants: waitlist.Participants, FreeSlots: waitlist.FreeSlots,
			SlotFreeForNext: waitlist.HasFreeSlotForNext(), Entries: entries,
		})
	}
	return out
}
