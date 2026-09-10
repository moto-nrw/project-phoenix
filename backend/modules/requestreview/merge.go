package requestreview

import (
	"context"
	"strconv"
)

const (
	// fetchLimit is the per-queue page size of one internal fetch; larger
	// than the response limit so a page still fills when the per-child scope
	// drops rows.
	fetchLimit = 50
	// maxFetches bounds the DB scanning per type and request. Child name and
	// student filters run in SQL, so what still discards rows here is the
	// status/decided-at filter and the caller's per-child scope. A page may
	// come back underfilled with a next_cursor; the client's "load more"
	// continues from there.
	maxFetches = 4
)

// rowBefore reports whether a is emitted before b (newer first; deterministic
// type-then-id tie-break).
func rowBefore(a, b *Row) bool {
	if !a.SortTime.Equal(b.SortTime) {
		return a.SortTime.After(b.SortTime)
	}
	if a.Type != b.Type {
		return typeRank(a.Type) < typeRank(b.Type)
	}
	return a.ID > b.ID
}

// newItem is the single place a row becomes a wire item, so both views agree
// on what occurred_at means. Past requests (#2267, stories 14/15) change
// nothing if approved: staff still see them — an invisible request is one
// nobody ever closes — but they can only reject or mark them done, and they
// never join a bulk approval.
func newItem(row *Row) Item {
	item := Item{
		RequestType:          row.Type,
		OccurredAt:           row.SortTime,
		StudentID:            strconv.FormatInt(row.StudentID, 10),
		StudentName:          row.StudentName,
		ExpectedVersion:      row.Version,
		UrgentToday:          row.UrgentToday,
		Past:                 row.Past,
		BulkEligible:         row.BulkEligible,
		BulkIneligibleReason: row.BulkIneligibleReason,
		BulkIneligibleText:   row.BulkIneligibleText,
		ConflictKeys:         row.ConflictKeys,
		CurrentValueChanged:  row.CurrentValueChanged,
		CurrentStatusByDate:  row.CurrentStatusByDate,
		CanCorrect:           row.CanCorrect,
		Data:                 row.Data,
	}
	if row.Past {
		item.BulkEligible = false
		item.BulkIneligibleReason = BulkIneligiblePast
		item.BulkIneligibleText = bulkIneligiblePastText
	}
	return item
}

type fetchFunc func(ctx context.Context, filter QueueFilter) ([]Row, *Cursor, error)

// source pulls one type's pages on demand. It keeps the scan frontier (where
// the next DB fetch continues) separate from the consumed position (what the
// response cursor reports): the frontier may run ahead over rows the Go-side
// filters discarded, but the reported cursor must never skip a buffered,
// still-unconsumed row.
type source struct {
	typ      string
	fetch    fetchFunc
	incoming *cursorPosition
	scan     *cursorPosition
	consumed *cursorPosition
	buf      []Row
	done     bool
	fetches  int
}

func (s *source) peek(ctx context.Context, q *ListQuery) (*Row, error) {
	for len(s.buf) == 0 && !s.done && s.fetches < maxFetches {
		s.fetches++
		filter := q.queueFilter()
		filter.Limit = fetchLimit
		if s.scan != nil {
			filter.Before = &Cursor{Instant: s.scan.UpdatedAt, ID: s.scan.ID}
		}
		rows, next, err := s.fetch(ctx, filter)
		if err != nil {
			return nil, err
		}
		for i := range rows {
			if q.matches(&rows[i]) {
				s.buf = append(s.buf, rows[i])
			}
		}
		if next == nil {
			s.done = true
		} else {
			s.scan = &cursorPosition{UpdatedAt: next.Instant, ID: next.ID}
		}
	}
	if len(s.buf) == 0 {
		return nil, nil
	}
	return &s.buf[0], nil
}

func (s *source) pop() Row {
	row := s.buf[0]
	s.buf = s.buf[1:]
	s.consumed = &cursorPosition{UpdatedAt: row.SortTime, ID: row.ID}
	return row
}

// hasMore reports whether this source may still hold unread rows: buffered
// ones, or DB pages the scan budget did not reach.
func (s *source) hasMore() bool {
	return len(s.buf) > 0 || !s.done
}

// position is the keyset position the response cursor reports for this
// type. The bool distinguishes an omitted source from an explicit nil start
// position: the latter preserves a source with prefetched but unconsumed
// rows. Preference order: last consumed row (exact resume point); the
// incoming position while unconsumed rows sit in the buffer (never skip
// them); otherwise the scan frontier, so a page whose rows were ALL filtered
// still makes progress instead of looping the client forever.
func (s *source) position() (*cursorPosition, bool) {
	if s.consumed != nil {
		return s.consumed, true
	}
	if len(s.buf) > 0 {
		return s.incoming, true
	}
	if !s.done && s.scan != nil {
		return s.scan, true
	}
	if s.incoming != nil {
		return s.incoming, true
	}
	return nil, false
}

func (s *service) sourceFor(typ string, history bool) *source {
	src := &source{typ: typ}
	queues := s.deps.Queues
	switch typ {
	case TypeMasterData:
		src.fetch = queueFetch(queues.MasterData, history)
	case TypeCareSchedule:
		src.fetch = queueFetch(queues.CareSchedule, history)
	case TypeOffering:
		src.fetch = queueFetch(queues.Offering, history)
	case TypeExcused:
		src.fetch = queueFetch(queues.Excused, history)
	case TypeDirectCorrection:
		// History only — a correction has no open state. validate drops the
		// type from the working list before it gets here.
		src.fetch = queues.DirectCorrections.History
	}
	return src
}

func queueFetch(queue Queue, history bool) fetchFunc {
	if history {
		return queue.History
	}
	return queue.Open
}

// page merges the requested types into one keyset page. Open list and
// history walk the identical path — they differ only in which queue method
// each source pulls from and which instant the rows sort on. The open view
// additionally serves the urgent requests first: the urgent phase drains
// completely before the normal phase begins, and the cursor remembers which
// phase the client is in.
func (s *service) page(ctx context.Context, q *ListQuery) (Page, error) {
	if q.History {
		return s.priorityPage(ctx, q)
	}
	if _, normalPhase := q.cursor[normalPhaseCursorKey]; normalPhase {
		normal := false
		q.urgentOnly = &normal
		page, err := s.priorityPage(ctx, q)
		page.NextCursor = withNormalPhaseCursor(page.NextCursor)
		return page, err
	}
	urgent := true
	q.urgentOnly = &urgent
	page, err := s.priorityPage(ctx, q)
	if err != nil || page.NextCursor != "" {
		return page, err
	}
	if len(page.Items) == q.Limit {
		page.NextCursor = encodeCursor(pageCursor{normalPhaseCursorKey: nil})
		return page, nil
	}
	normal := *q
	normal.cursor = nil
	normal.Limit = q.Limit - len(page.Items)
	normalUrgent := false
	normal.urgentOnly = &normalUrgent
	normalPage, err := s.priorityPage(ctx, &normal)
	if err != nil {
		return Page{}, err
	}
	page.Items = append(page.Items, normalPage.Items...)
	page.NextCursor = withNormalPhaseCursor(normalPage.NextCursor)
	return page, nil
}

func (s *service) priorityPage(ctx context.Context, q *ListQuery) (Page, error) {
	sources := make([]*source, 0, len(q.Types))
	for _, typ := range q.Types {
		src := s.sourceFor(typ, q.History)
		if pos, ok := q.cursor[typ]; ok && pos != nil {
			pin := *pos
			src.incoming = &pin
			src.scan = &pin
		}
		sources = append(sources, src)
	}

	page := Page{Items: make([]Item, 0, q.Limit)}
	for len(page.Items) < q.Limit {
		best, err := newestSource(ctx, q, sources)
		if err != nil {
			return Page{}, err
		}
		if best == nil {
			break
		}
		row := best.pop()
		page.Items = append(page.Items, newItem(&row))
	}
	page.NextCursor = nextCursor(sources)
	return page, nil
}

// newestSource returns the source whose buffered head row sorts first across
// all types, or nil when every source is drained.
func newestSource(ctx context.Context, q *ListQuery, sources []*source) (*source, error) {
	var best *source
	var bestRow *Row
	for _, src := range sources {
		row, err := src.peek(ctx, q)
		if err != nil {
			return nil, err
		}
		if row != nil && (bestRow == nil || rowBefore(row, bestRow)) {
			best, bestRow = src, row
		}
	}
	return best, nil
}

// nextCursor assembles the follow-up cursor, or "" when every source is
// exhausted.
func nextCursor(sources []*source) string {
	next := make(pageCursor)
	hasMore := false
	for _, src := range sources {
		if src.hasMore() {
			hasMore = true
		}
		if pos, ok := src.position(); ok {
			next[src.typ] = pos
		}
	}
	if !hasMore {
		return ""
	}
	return encodeCursor(next)
}
