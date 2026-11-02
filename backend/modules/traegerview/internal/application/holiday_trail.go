package application

import (
	"context"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/traegerview"
)

// The holiday registrations of a Träger in its access log (#5326): every
// read and change of their personal data by the Träger-Büro. They belong to
// the Träger, not to a school (ADR 0114 point 6), so their entries carry no
// school and no school's access log records that the access log showed them.

// holidayTrail reads the newest limit entries of the Träger's holiday trail
// of the filter in the projection's administrative unit. A school filter
// shows the school's entries only.
func (o *Overview) holidayTrail(adminCtx context.Context, viewer traegerview.Viewer, filter traegerview.AccessLogFilter, span window, limit int) ([]traegerview.AccessTrailEntry, error) {
	if filter.SchoolID != 0 {
		return nil, nil
	}
	rows, err := o.holiday.ListHolidayTrail(adminCtx, viewer.OrganizationID, traegerview.HolidayTrailQuery{
		From: span.from, To: span.to, AccountID: filter.AccountID, Kind: filter.Kind, Limit: limit,
	})
	for i := range rows {
		// The trail stores no name, and a deleted account leaves no
		// account: a person did it, not an automated process.
		if rows[i].AccountID == nil {
			rows[i].ActorNameSnapshot = personUnknown
		}
	}
	return rows, err
}

// mergedPage merges the newest offset+limit entries of both trails in the
// order of the Audit Platform's trail and cuts the page from them.
func mergedPage(schoolRows, holidayRows []traegerview.AccessTrailEntry, limit, offset int) []traegerview.AccessTrailEntry {
	rows := slices.Concat(schoolRows, holidayRows)
	slices.SortFunc(rows, newestFirst)
	if offset >= len(rows) {
		return nil
	}
	return rows[offset:min(offset+limit, len(rows))]
}

// newestFirst orders entries like the Audit Platform's trail: newest first,
// then by school, kind and data, and the newer entry of a tie first.
func newestFirst(a, b traegerview.AccessTrailEntry) int {
	if c := b.OccurredAt.Compare(a.OccurredAt); c != 0 {
		return c
	}
	if a.SchoolID != b.SchoolID {
		return order(a.SchoolID < b.SchoolID)
	}
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	if c := strings.Compare(a.Data, b.Data); c != 0 {
		return c
	}
	return order(a.ID > b.ID)
}

func order(first bool) int {
	if first {
		return -1
	}
	return 1
}

// schoolName names the school of an entry; an entry without one is the
// Träger's own.
func schoolName(names map[int64]string, school int64) string {
	if school == 0 {
		return withoutSchool
	}
	return names[school]
}
