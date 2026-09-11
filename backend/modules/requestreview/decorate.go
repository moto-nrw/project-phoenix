package requestreview

import (
	"context"
	"fmt"
	"slices"
)

// decorateOpenPage adds everything only the working list needs: the caller's
// review reach (so an empty list can explain itself), the conflict groups,
// the child's group, and the Familienschutz flag. Optional ports stay
// optional — without them the field is simply omitted.
func (s *service) decorateOpenPage(ctx context.Context, page *Page, types []string) error {
	access, err := s.deps.Access.ReviewAccess(ctx)
	if err != nil {
		return err
	}
	page.ReviewAccess = access
	if err := s.decorateConflicts(ctx, page, types); err != nil {
		return err
	}
	if s.deps.Students != nil {
		if err := s.decorateStudentGroups(ctx, page); err != nil {
			return err
		}
	}
	if s.deps.FamilyProtection != nil {
		return s.decorateFamilyProtection(ctx, page)
	}
	return nil
}

func (s *service) decorateStudentGroups(ctx context.Context, page *Page) error {
	names, err := s.deps.Students.GroupNames(ctx, studentIDs(page.Items))
	if err != nil {
		return fmt.Errorf("load students for request queue groups: %w", err)
	}
	for i := range page.Items {
		page.Items[i].GroupName = names[page.Items[i].studentID]
	}
	return nil
}

func (s *service) decorateFamilyProtection(ctx context.Context, page *Page) error {
	protected, err := s.deps.FamilyProtection.Protected(ctx, studentIDs(page.Items))
	if err != nil {
		return fmt.Errorf("load family protection for request queue: %w", err)
	}
	for i := range page.Items {
		page.Items[i].FamilyProtected = protected[page.Items[i].studentID]
	}
	return nil
}

// studentIDs are the children on this page, each once, in page order.
func studentIDs(items []Item) []int64 {
	ids := make([]int64, 0, len(items))
	seen := make(map[int64]struct{}, len(items))
	for _, item := range items {
		if item.studentID <= 0 {
			continue
		}
		if _, ok := seen[item.studentID]; !ok {
			seen[item.studentID] = struct{}{}
			ids = append(ids, item.studentID)
		}
	}
	return ids
}

// Conflict grouping (#2267, stories 6-10). The keys say WHAT a request would
// write, so two requests wanting opposite things for the same weekday, day,
// field or offering land in one group and the client can force a single
// result instead of letting the second decision silently overwrite the
// first. The owner adapters derive the keys with the shared owner rule, so
// the list and the resolve path cannot disagree.

// conflictScopeLimit bounds the conflict scan. A child with more open
// requests than this is far outside anything a school produces; the group
// size is then a lower bound rather than a wrong number, and the grouping
// itself still works.
const conflictScopeLimit = 200

// decorateConflicts fills conflict_key and conflict_group_size.
//
// The scan asks the queues for EVERY open request of the children on this
// page — deliberately not the page itself. A page is a window: two
// contradicting requests can easily land on different pages, and a group
// size counted from the window would tell staff "1" for a request that has a
// contradiction waiting one scroll away. That is the failure this whole
// feature exists to prevent, so it is worth four extra queries per page.
func (s *service) decorateConflicts(ctx context.Context, page *Page, types []string) error {
	ids := studentIDs(page.Items)
	if len(ids) == 0 {
		return nil
	}
	// keyed counts every open request per (student, conflict key).
	keyed := make(map[int64]map[string]int)
	count := func(studentID int64, keys []string) {
		if studentID <= 0 || len(keys) == 0 {
			return
		}
		if keyed[studentID] == nil {
			keyed[studentID] = make(map[string]int, len(keys))
		}
		for _, key := range keys {
			keyed[studentID][key]++
		}
	}
	if err := s.scanOpenConflicts(ctx, ids, types, count); err != nil {
		return err
	}
	for i := range page.Items {
		page.Items[i].ConflictKey, page.Items[i].ConflictGroupSize =
			largestConflictGroup(keyed[page.Items[i].studentID], page.Items[i].ConflictKeys)
	}
	return nil
}

// largestConflictGroup picks the key this item is most contended on. An item
// that shares no key with another request reports no key and a group of one,
// which is what "nothing to resolve" looks like on the wire.
func largestConflictGroup(counts map[string]int, keys []string) (string, int) {
	// Zero, not one: "no group" is absent from the wire, so a client cannot
	// mistake a lone request for a group of one and render a radio list with a
	// single option in it.
	best, bestSize := "", 0
	for _, key := range keys {
		if size := counts[key]; size > 1 && size > bestSize {
			best, bestSize = key, size
		}
	}
	return best, bestSize
}

// scanOpenConflicts walks the open queues the caller is actually served — a
// queue the type filter or the permission scope excluded is NOT queried, so
// the scan can never reach data the list itself refused to show. Nothing is
// lost by that: a conflict key is type-specific, so two requests can only
// share one when they come from the same queue.
func (s *service) scanOpenConflicts(
	ctx context.Context,
	ids []int64,
	types []string,
	report func(studentID int64, keys []string),
) error {
	filter := QueueFilter{StudentIDs: ids, Limit: conflictScopeLimit}
	queues := s.deps.Queues
	for _, scan := range []struct {
		typ   string
		queue Queue
	}{
		{TypeMasterData, queues.MasterData},
		{TypeCareSchedule, queues.CareSchedule},
		{TypeOffering, queues.Offering},
		{TypeExcused, queues.Excused},
	} {
		if !slices.Contains(types, scan.typ) {
			continue
		}
		rows, _, err := scan.queue.Open(ctx, filter)
		if err != nil {
			return fmt.Errorf("scan %s conflicts: %w", scan.typ, err)
		}
		for i := range rows {
			report(rows[i].StudentID, rows[i].ConflictKeys)
		}
	}
	return nil
}
