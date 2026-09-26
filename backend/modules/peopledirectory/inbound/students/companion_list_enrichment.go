package students

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// enrichWithCompanions fills CompanionStudentIDs for the given day with the
// child's whole Laufgemeinschaft (the connected component, see
// CompanionIDsForWeekday) — the page cannot derive it from direct links alone
// when a bridging child is filtered out.
//
// Only the ids travel, not the names: the Kindersuche already knows every child
// on the page, so it can resolve a companion that is visible and quietly ignore
// one that is filtered out. Shipping names here would leak children the caller
// filtered away.
//
// A failure is NOT swallowed: the caller asked for the grouping, and empty
// companion ids are indistinguishable from "this child walks alone". The
// Kindersuche would file every linked child under "Ohne Laufgemeinschaft" and
// present a transient query failure as a real departure arrangement, so the
// request fails instead.
func (rs *Resource) enrichWithCompanions(ctx context.Context, responses []StudentResponse, params *studentListParams, day time.Time) error {
	if !params.includeCompanions {
		return nil
	}

	studentIDs := collectFullAccessStudentIDs(responses)
	if len(studentIDs) == 0 {
		return nil
	}

	weekday := companionWeekdayForDate(day)
	byStudent, err := rs.CompanionService.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	if err != nil {
		rs.Logger.Error("failed to load departure companions for list",
			slog.String("error", err.Error()),
			slog.Int("weekday", weekday),
		)
		return err
	}

	for i := range responses {
		if companions, ok := byStudent[responses[i].ID]; ok && len(companions) > 0 {
			responses[i].CompanionStudentIDs = companions
		}
	}
	return nil
}

// enrichWithCompanionLinks fills DepartureCompanions with each child's own
// "läuft mit" links, for the offline lists that have to print WHO a child walks
// home with.
//
// Unlike enrichWithCompanions (ids of the whole Laufgemeinschaft, for the
// Kindersuche grouping) this carries NAMES, so it is restricted to the children
// the caller has full access to — the same gate the grouping uses. That gate
// covers only the SUBJECT of each link: for a caller without full access
// (guest/guardian, #2329) the far end's name must not ride into the exported
// document either — the same per-companion redaction getStudentCompanions
// applies runs here before any link is attached to a response.
//
// A failure is returned, not swallowed. The caller decides: an export whose
// columns include the departure plan MUST abort, because a child whose
// accompanied days are backed only by links legitimately has no free-text note,
// so departureExportCell would print a bare "Mit anderem Kind" and hand staff a
// paper pickup list without a single name. Exports that do not carry the
// departure column lose nothing and may continue.
func (rs *Resource) enrichWithCompanionLinks(ctx context.Context, responses []StudentResponse, accessCtx *studentAccessContext) error {
	studentIDs := collectFullAccessStudentIDs(responses)
	if len(studentIDs) == 0 {
		return nil
	}

	storedByStudent, err := rs.CompanionService.ListCompanionsForStudents(ctx, studentIDs)
	if err != nil {
		rs.Logger.Error("failed to load departure companions for export",
			slog.String("error", err.Error()),
		)
		return err
	}

	byStudent := make(map[int64][]departure.CompanionLink, len(storedByStudent))
	for studentID, links := range storedByStudent {
		byStudent[studentID] = peopleCompanionLinks(links)
	}

	// Redaction failing must NOT fall through to the unredacted names — the
	// column is optional, the far-end names are another child's personal data.
	linkSets := make([][]departure.CompanionLink, 0, len(byStudent))
	for _, links := range byStudent {
		linkSets = append(linkSets, links)
	}
	if err := rs.redactCompanionNames(ctx, accessCtx, linkSets...); err != nil {
		rs.Logger.Error("failed to authorize departure companions for export",
			slog.String("error", err.Error()),
		)
		return err
	}

	for i := range responses {
		if links, ok := byStudent[responses[i].ID]; ok && len(links) > 0 {
			responses[i].DepartureCompanions = links
		}
	}
	return nil
}

// companionWeekdayForDate maps a date onto the stored 1..5 weekday.
//
// Saturday and Sunday fall back to Monday rather than resolving to "no
// weekday": OGS care does not run on the weekend, so a staff member opening
// the Kindersuche on a Sunday is looking ahead at the coming week. Returning
// nothing there would make the Laufgemeinschaft grouping look broken (every
// child in "Ohne Laufgemeinschaft") for a reason no one can see.
//
// The instant is moved to Berlin first: it comes from rs.Now, which is
// time.Now in production (a container clock that is normally UTC) and an
// arbitrary injected clock in tests. Reading Weekday() off that location would
// spend the late Berlin evening — already the next calendar day here — querying
// yesterday's links and grouping the children by the wrong day's
// Laufgemeinschaft.
func companionWeekdayForDate(day time.Time) int {
	switch day.In(timezone.Berlin).Weekday() {
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 3
	case time.Thursday:
		return 4
	case time.Friday:
		return 5
	default:
		return 1
	}
}

// toCompanionLinks converts the wire entries into the service's link shape.
func toCompanionLinks(entries []CompanionEntry) []departure.CompanionLink {
	links := make([]departure.CompanionLink, 0, len(entries))
	for _, entry := range entries {
		links = append(links, departure.CompanionLink{
			CompanionStudentID: entry.CompanionStudentID,
			Weekdays:           entry.Weekdays,
		})
	}
	return links
}

func toCompanionResponses(links []departure.CompanionLink) []CompanionResponse {
	out := make([]CompanionResponse, 0, len(links))
	for _, link := range links {
		weekdays := link.Weekdays
		if weekdays == nil {
			weekdays = []string{}
		}
		out = append(out, CompanionResponse{
			CompanionStudentID: link.CompanionStudentID,
			FirstName:          link.FirstName,
			LastName:           link.LastName,
			Weekdays:           weekdays,
		})
	}
	return out
}

// carePlanCompanionLinks and peopleCompanionLinks translate the "läuft mit"
// links at the seam between Care Plan, which owns them, and the People
// Directory row whose departure plan they answer for. A nil list stays nil.
func carePlanCompanionLinks(links []departure.CompanionLink) []careplan.CompanionLink {
	if links == nil {
		return nil
	}
	result := make([]careplan.CompanionLink, 0, len(links))
	for _, link := range links {
		result = append(result, careplan.CompanionLink(link))
	}
	return result
}

func peopleCompanionLinks(links []careplan.CompanionLink) []departure.CompanionLink {
	if links == nil {
		return nil
	}
	result := make([]departure.CompanionLink, 0, len(links))
	for _, link := range links {
		result = append(result, departure.CompanionLink(link))
	}
	return result
}
