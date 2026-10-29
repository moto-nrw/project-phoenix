package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The access log of the Träger-Portal (#4655, ADR 0109). It reads the access
// trail of the Träger's schools in the projection's administrative unit and,
// after that unit ends, writes each answer to the access log of every school
// it shows, in that school's tenant transaction (ADR 0084 section 7). A
// failed write fails the answer: nothing leaves without its trail.

// The person shown when the trail names no account, or an account without a
// name the Träger may see.
const (
	personWithoutAccount = "Ohne Person (automatisch)"
	personUnknown        = "Unbekannte Person"
	traegerBueroSuffix   = " (Träger-Büro)"
	// withoutSchool stands in the school column of an entry of the
	// Träger's own data, such as its holiday registrations (#5326).
	withoutSchool = "Träger (keine Schule)"
)

// window is the instant range of a filter: the Berlin day From from
// midnight to the midnight after To.
type window struct {
	from, to time.Time
}

func accessLogWindow(from, to calendar.Date) (window, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) || from.DaysUntil(to) >= traegerview.AccessLogMaxDays {
		return window{}, fmt.Errorf("%w: the window must run from a day to a later or the same day, at most %d days", traegerview.ErrInvalidAccessLogFilter, traegerview.AccessLogMaxDays)
	}
	return window{from: from.BerlinMidnight(), to: to.AddDays(1).BerlinMidnight()}, nil
}

func validFilter(viewer traegerview.Viewer, filter traegerview.AccessLogFilter) (window, error) {
	if viewer.OrganizationID <= 0 || viewer.AccountID <= 0 {
		return window{}, traegerview.ErrTraegerRequired
	}
	switch filter.Kind {
	case "", traegerview.AccessKindRead, traegerview.AccessKindChange:
	default:
		return window{}, fmt.Errorf("%w: unknown kind %q", traegerview.ErrInvalidAccessLogFilter, filter.Kind)
	}
	if filter.SchoolID < 0 || filter.AccountID < 0 {
		return window{}, fmt.Errorf("%w: negative ID", traegerview.ErrInvalidAccessLogFilter)
	}
	return accessLogWindow(filter.From, filter.To)
}

// AccessLog lists one page of the access log, newest first.
func (o *Overview) AccessLog(ctx context.Context, viewer traegerview.Viewer, filter traegerview.AccessLogFilter, page int) (traegerview.AccessLogPage, error) {
	span, err := validFilter(viewer, filter)
	if err != nil {
		return traegerview.AccessLogPage{}, err
	}
	if page < 1 {
		return traegerview.AccessLogPage{}, fmt.Errorf("%w: the page starts at 1", traegerview.ErrInvalidAccessLogFilter)
	}
	result := traegerview.AccessLogPage{Entries: []traegerview.AccessLogEntry{}, AsOf: filter.AsOf}
	if result.AsOf.IsZero() {
		result.AsOf = time.Now()
	}
	// The pages of one list end at the same instant, so the reads this list
	// records below never push an earlier entry onto a later page.
	listed := span
	if result.AsOf.Before(listed.to) {
		listed.to = result.AsOf
	}
	err = o.read(ctx, viewer.OrganizationID, func(adminCtx context.Context, schools []int64) error {
		entries, err := o.entries(adminCtx, viewer, schools, filter, listed, traegerview.AccessLogPageSize+1, (page-1)*traegerview.AccessLogPageSize)
		if err != nil {
			return err
		}
		if len(entries) > traegerview.AccessLogPageSize {
			entries, result.HasMore = entries[:traegerview.AccessLogPageSize], true
		}
		result.Entries = entries
		return nil
	})
	if err != nil {
		return traegerview.AccessLogPage{}, err
	}
	if err := o.record(ctx, viewer, traegerview.ResourceTypeTraegerAccessLog, span, entrySchools(result.Entries), "list", page); err != nil {
		return traegerview.AccessLogPage{}, err
	}
	return result, nil
}

// AccessLogExport lists every entry of the filter for the CSV export.
func (o *Overview) AccessLogExport(ctx context.Context, viewer traegerview.Viewer, filter traegerview.AccessLogFilter) ([]traegerview.AccessLogEntry, error) {
	span, err := validFilter(viewer, filter)
	if err != nil {
		return nil, err
	}
	entries := []traegerview.AccessLogEntry{}
	err = o.read(ctx, viewer.OrganizationID, func(adminCtx context.Context, schools []int64) error {
		var err error
		entries, err = o.entries(adminCtx, viewer, schools, filter, span, traegerview.AccessLogExportLimit+1, 0)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(entries) > traegerview.AccessLogExportLimit {
		return nil, traegerview.ErrAccessLogExportTooLarge
	}
	if err := o.record(ctx, viewer, traegerview.ResourceTypeTraegerAccessLogExport, span, entrySchools(entries), "export", 0); err != nil {
		return nil, err
	}
	return entries, nil
}

// AccessLogPeople lists the people with an entry in the window, by name.
func (o *Overview) AccessLogPeople(ctx context.Context, viewer traegerview.Viewer, from, to calendar.Date) ([]traegerview.AccessLogPerson, error) {
	if viewer.OrganizationID <= 0 || viewer.AccountID <= 0 {
		return nil, traegerview.ErrTraegerRequired
	}
	span, err := accessLogWindow(from, to)
	if err != nil {
		return nil, err
	}
	people := []traegerview.AccessLogPerson{}
	shown := map[int64]int{}
	err = o.read(ctx, viewer.OrganizationID, func(adminCtx context.Context, schools []int64) error {
		actors, err := o.trailActors(adminCtx, viewer, schools, span)
		if err != nil || len(actors) == 0 {
			return err
		}
		refs := make([]actorRef, 0, len(actors))
		for _, trailActor := range actors {
			refs = append(refs, trailActorRef(trailActor))
		}
		names, err := o.actorIndex(adminCtx, viewer.OrganizationID, schools, refs)
		if err != nil {
			return err
		}
		people, shown = distinctPeople(actors, names)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := o.record(ctx, viewer, traegerview.ResourceTypeTraegerAccessLog, span, shown, "people", 0); err != nil {
		return nil, err
	}
	return people, nil
}

// trailActors lists who acted in the window: at the schools shown, then in
// the Träger's holiday trail.
func (o *Overview) trailActors(adminCtx context.Context, viewer traegerview.Viewer, schools []int64, span window) ([]traegerview.AccessTrailActor, error) {
	actors, err := o.holiday.ListHolidayTrailActors(adminCtx, viewer.OrganizationID, span.from, span.to)
	if err != nil || len(schools) == 0 {
		return actors, err
	}
	schoolActors, err := o.trail.ListAccessTrailActors(adminCtx, schools, span.from, span.to)
	if err != nil {
		return nil, err
	}
	return append(schoolActors, actors...), nil
}

// distinctPeople keeps one entry per account, named after the first school
// that knows a person for it, ordered by name. A staff member is listed under
// the account of its person; one without an account cannot be chosen as a
// filter and is left out. shown counts the people each school contributes.
func distinctPeople(actors []traegerview.AccessTrailActor, names actorIndex) ([]traegerview.AccessLogPerson, map[int64]int) {
	byAccount := map[int64]string{}
	shown := map[int64]int{}
	for _, trailActor := range actors {
		shown[trailActor.SchoolID]++
		resolved := names.resolve(trailActorRef(trailActor))
		if resolved.account == nil {
			continue
		}
		if current, found := byAccount[*resolved.account]; !found || current == personUnknown {
			byAccount[*resolved.account] = resolved.name
		}
	}
	people := make([]traegerview.AccessLogPerson, 0, len(byAccount))
	for accountID, name := range byAccount {
		people = append(people, traegerview.AccessLogPerson{AccountID: accountID, Name: name})
	}
	slices.SortFunc(people, func(a, b traegerview.AccessLogPerson) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		case a.AccountID < b.AccountID:
			return -1
		case a.AccountID > b.AccountID:
			return 1
		}
		return 0
	})
	return people, shown
}

// entries reads the trail of the filter at the Träger's schools and the
// Träger's holiday trail, merges them, and names their schools and people.
// A school outside the Träger reads nothing.
func (o *Overview) entries(adminCtx context.Context, viewer traegerview.Viewer, schools []int64, filter traegerview.AccessLogFilter, span window, limit, offset int) ([]traegerview.AccessLogEntry, error) {
	holiday, err := o.holidayTrail(adminCtx, viewer, filter, span, offset+limit)
	if err != nil {
		return nil, err
	}
	rows, err := o.schoolTrail(adminCtx, schools, filter, span, limit, offset, len(holiday) > 0)
	if err != nil {
		return nil, err
	}
	if len(holiday) > 0 {
		rows = mergedPage(rows, holiday, limit, offset)
	}
	if len(rows) == 0 {
		return []traegerview.AccessLogEntry{}, nil
	}
	schoolNames, err := o.schoolNames(adminCtx, schools)
	if err != nil {
		return nil, err
	}
	refs := make([]actorRef, 0, len(rows))
	for _, row := range rows {
		refs = append(refs, actorRef{school: row.SchoolID, account: row.AccountID, staff: row.StaffID, snapshot: row.ActorNameSnapshot})
	}
	names, err := o.actorIndex(adminCtx, viewer.OrganizationID, schools, refs)
	if err != nil {
		return nil, err
	}
	entries := make([]traegerview.AccessLogEntry, 0, len(rows))
	for i, row := range rows {
		resolved := names.resolve(refs[i])
		entries = append(entries, traegerview.AccessLogEntry{
			ID: fmt.Sprintf("%s-%s-%d", row.Kind, row.Data, row.ID), OccurredAt: row.OccurredAt, SchoolID: row.SchoolID, SchoolName: schoolName(schoolNames, row.SchoolID),
			AccountID: resolved.account, Person: resolved.name, Kind: row.Kind, Data: row.Data, DataLabel: dataLabel(row.Data),
		})
	}
	return entries, nil
}

// schoolTrail reads the trail of the filter at the Träger's schools. A
// school outside the Träger reads nothing. To be merged with the holiday
// trail, it reads the newest offset+limit entries instead of the page.
func (o *Overview) schoolTrail(adminCtx context.Context, schools []int64, filter traegerview.AccessLogFilter, span window, limit, offset int, merged bool) ([]traegerview.AccessTrailEntry, error) {
	if len(schools) == 0 || (filter.SchoolID != 0 && !slices.Contains(schools, filter.SchoolID)) {
		return nil, nil
	}
	var staffIDs []int64
	if filter.AccountID != 0 {
		var err error
		if staffIDs, err = o.staffOfAccount(adminCtx, schools, filter.AccountID); err != nil {
			return nil, err
		}
	}
	if merged {
		limit, offset = offset+limit, 0
	}
	return o.trail.ListAccessTrail(adminCtx, traegerview.AccessTrailQuery{
		Schools: schools, From: span.from, To: span.to, SchoolID: filter.SchoolID,
		AccountID: filter.AccountID, StaffIDs: staffIDs, Kind: filter.Kind, Limit: limit, Offset: offset,
	})
}

// schoolNames names the schools of the set.
func (o *Overview) schoolNames(adminCtx context.Context, schools []int64) (map[int64]string, error) {
	if len(schools) == 0 {
		return map[int64]string{}, nil
	}
	rows, err := o.rows.Schools(adminCtx, schools)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for _, school := range rows {
		names[school.ID] = school.Name
	}
	return names, nil
}

func entrySchools(entries []traegerview.AccessLogEntry) map[int64]int {
	counts := map[int64]int{}
	for _, entry := range entries {
		counts[entry.SchoolID]++
	}
	return counts
}

// record writes the read to the access log of every school it showed, with
// the number of rows it showed there. Entries without a school are the
// Träger's own data and have no school access log (ADR 0114 point 6).
func (o *Overview) record(ctx context.Context, viewer traegerview.Viewer, resourceType string, span window, shown map[int64]int, view string, page int) error {
	schools := make([]int64, 0, len(shown))
	for school := range shown {
		if school > 0 {
			schools = append(schools, school)
		}
	}
	slices.Sort(schools)
	for _, school := range schools {
		if err := o.recorder.RecordTraegerAccess(ctx, traegerview.TraegerAccess{
			SchoolID: school, AccountID: viewer.AccountID, OrganizationID: viewer.OrganizationID, ResourceType: resourceType,
			RangeStart: span.from, RangeEnd: span.to, View: view, Page: page, Rows: shown[school],
		}); err != nil {
			return fmt.Errorf("traeger view: record the access at school %d: %w", school, err)
		}
	}
	return nil
}
