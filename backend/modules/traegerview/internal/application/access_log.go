package application

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The access log of the Träger-Portal (#4655, ADR 0111). It reads the access
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
		if len(schools) == 0 {
			return nil
		}
		actors, err := o.trail.ListAccessTrailActors(adminCtx, schools, span.from, span.to)
		if err != nil {
			return err
		}
		accounts := make([]int64, 0, len(actors))
		for _, actor := range actors {
			accounts = append(accounts, actor.AccountID)
		}
		names, err := o.accountNames(adminCtx, viewer.OrganizationID, accounts)
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

// distinctPeople keeps one entry per account, named after the first school
// that knows a person for it, ordered by name. shown counts the people each
// school contributes.
func distinctPeople(actors []traegerview.AccessTrailActor, names accountNameIndex) ([]traegerview.AccessLogPerson, map[int64]int) {
	byAccount := map[int64]string{}
	shown := map[int64]int{}
	for _, actor := range actors {
		shown[actor.SchoolID]++
		name := names.person(actor.SchoolID, &actor.AccountID, actor.ActorNameSnapshot)
		if current, found := byAccount[actor.AccountID]; !found || current == personUnknown {
			byAccount[actor.AccountID] = name
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

// entries reads the trail of the filter at the Träger's schools and names
// its schools and people. A school outside the Träger reads nothing.
func (o *Overview) entries(adminCtx context.Context, viewer traegerview.Viewer, schools []int64, filter traegerview.AccessLogFilter, span window, limit, offset int) ([]traegerview.AccessLogEntry, error) {
	if len(schools) == 0 || (filter.SchoolID != 0 && !slices.Contains(schools, filter.SchoolID)) {
		return []traegerview.AccessLogEntry{}, nil
	}
	rows, err := o.trail.ListAccessTrail(adminCtx, traegerview.AccessTrailQuery{
		Schools: schools, From: span.from, To: span.to, SchoolID: filter.SchoolID,
		AccountID: filter.AccountID, Kind: filter.Kind, Limit: limit, Offset: offset,
	})
	if err != nil || len(rows) == 0 {
		return []traegerview.AccessLogEntry{}, err
	}
	schoolRows, err := o.rows.Schools(adminCtx, schools)
	if err != nil {
		return nil, err
	}
	schoolNames := make(map[int64]string, len(schoolRows))
	for _, school := range schoolRows {
		schoolNames[school.ID] = school.Name
	}
	accounts := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.AccountID != nil {
			accounts = append(accounts, *row.AccountID)
		}
	}
	names, err := o.accountNames(adminCtx, viewer.OrganizationID, accounts)
	if err != nil {
		return nil, err
	}
	entries := make([]traegerview.AccessLogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, traegerview.AccessLogEntry{
			ID: fmt.Sprintf("%s-%s-%d", row.Kind, row.Data, row.ID), OccurredAt: row.OccurredAt, SchoolID: row.SchoolID, SchoolName: schoolNames[row.SchoolID],
			AccountID: row.AccountID, Person: names.person(row.SchoolID, row.AccountID, row.ActorNameSnapshot),
			Kind: row.Kind, Data: row.Data, DataLabel: dataLabel(row.Data),
		})
	}
	return entries, nil
}

type accountSchool struct{ account, school int64 }

// accountNameIndex holds the persons of accounts at schools and the
// addresses of the Träger-Büro.
type accountNameIndex struct {
	persons map[accountSchool]string
	bureau  map[int64]string
}

func (o *Overview) accountNames(adminCtx context.Context, organizationID int64, accounts []int64) (accountNameIndex, error) {
	index := accountNameIndex{persons: map[accountSchool]string{}, bureau: map[int64]string{}}
	slices.Sort(accounts)
	accounts = slices.Compact(accounts)
	if len(accounts) == 0 {
		return index, nil
	}
	persons, err := o.persons.PersonNames(adminCtx, accounts)
	if err != nil {
		return index, err
	}
	for _, name := range persons {
		index.persons[accountSchool{account: name.AccountID, school: name.SchoolID}] = name.Name
	}
	addresses, err := o.bureau.BureauAddresses(adminCtx, organizationID, accounts)
	if err != nil {
		return index, err
	}
	for _, name := range addresses {
		index.bureau[name.AccountID] = name.Name
	}
	return index, nil
}

// person names the actor of an entry: the person of the account at the
// entry's school, else the name the ledger stored, else the account's
// address in the Träger-Büro.
func (i accountNameIndex) person(school int64, account *int64, snapshot string) string {
	if account == nil {
		if snapshot != "" {
			return snapshot
		}
		return personWithoutAccount
	}
	if name := i.persons[accountSchool{account: *account, school: school}]; name != "" {
		return name
	}
	if snapshot != "" {
		return snapshot
	}
	if address := i.bureau[*account]; address != "" {
		return address + traegerBueroSuffix
	}
	return personUnknown
}

func entrySchools(entries []traegerview.AccessLogEntry) map[int64]int {
	counts := map[int64]int{}
	for _, entry := range entries {
		counts[entry.SchoolID]++
	}
	return counts
}

// record writes the read to the access log of every school it showed, with
// the number of rows it showed there.
func (o *Overview) record(ctx context.Context, viewer traegerview.Viewer, resourceType string, span window, shown map[int64]int, view string, page int) error {
	schools := make([]int64, 0, len(shown))
	for school := range shown {
		schools = append(schools, school)
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
