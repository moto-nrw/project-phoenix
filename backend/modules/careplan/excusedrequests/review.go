package excusedrequests

import (
	"context"
	"slices"
)

type ReviewQuery interface {
	ListPending(context.Context, QueueFilter) ([]*ReviewItem, *Cursor, error)
	ListHistory(context.Context, QueueFilter) ([]*HistoryItem, *Cursor, error)
}

func (r Request) UrgentOn(today Date) bool {
	return slices.Contains(r.Dates, today)
}

func (r Request) PastOn(today Date) bool {
	var last Date
	for _, date := range r.Dates {
		if last.IsZero() || date.After(last) {
			last = date
		}
	}
	return !last.IsZero() && last.Before(today)
}

func (r Request) ConflictKeys() []string {
	var keys []string
	for _, date := range r.Dates {
		if !date.IsZero() {
			keys = append(keys, "absence:"+date.String())
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys)
}

func (r Request) CanCorrect() bool {
	return r.Status == "approved" || r.Status == "rejected"
}
