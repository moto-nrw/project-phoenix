package audit

import (
	"context"
	"fmt"
	"time"
)

// AppendObservation is emitted once for every attempted event append. EventType
// is a stable Go type name; it never contains row data or other PII.
type AppendObservation struct {
	EventType string
	Duration  time.Duration
	Rows      int
	Err       error
}

type appendStore interface {
	Append(context.Context, any) error
}

type appendAllStore interface {
	AppendAll(context.Context, []any) error
}

type appendOnceStore interface {
	AppendOnce(context.Context, any) (bool, error)
}

// Command is the single application entry point for append-only Audit writes.
type Command struct {
	store   appendStore
	observe func(AppendObservation)
}

func NewCommand(store appendStore, observe func(AppendObservation)) (*Command, error) {
	if store == nil || observe == nil {
		return nil, fmt.Errorf("audit command: store and observer are required")
	}
	return &Command{store: store, observe: observe}, nil
}

func (c *Command) Append(ctx context.Context, event any) error {
	started := time.Now()
	err := c.store.Append(ctx, event)
	rows := 1
	if err != nil {
		rows = 0
	}
	c.observe(AppendObservation{EventType: fmt.Sprintf("%T", event), Duration: time.Since(started), Rows: rows, Err: err})
	return err
}

// AppendOnce appends an event subject to its database uniqueness constraint
// and reports whether this call inserted the row.
func (c *Command) AppendOnce(ctx context.Context, event any) (bool, error) {
	started := time.Now()
	store, ok := c.store.(appendOnceStore)
	if !ok {
		err := fmt.Errorf("audit command: store does not support append once")
		c.observe(AppendObservation{EventType: fmt.Sprintf("%T", event), Duration: time.Since(started), Rows: 0, Err: err})
		return false, err
	}
	inserted, err := store.AppendOnce(ctx, event)
	rows := 0
	if inserted {
		rows = 1
	}
	c.observe(AppendObservation{EventType: fmt.Sprintf("%T", event), Duration: time.Since(started), Rows: rows, Err: err})
	return inserted, err
}

// AppendAll appends every event in one statement where the store can, and one
// by one otherwise (#5552, ADR 0067 addendum of 2026-11-08: Audit AppendAll
// is a leaf). It is all or nothing: the store fails the whole call, and the
// caller's transaction or savepoint rolls back what was written.
func (c *Command) AppendAll(ctx context.Context, events []any) error {
	store, ok := c.store.(appendAllStore)
	if !ok {
		for _, event := range events {
			if err := c.Append(ctx, event); err != nil {
				return err
			}
		}
		return nil
	}
	started := time.Now()
	err := store.AppendAll(ctx, events)
	rows := len(events)
	if err != nil {
		rows = 0
	}
	for _, event := range events {
		c.observe(AppendObservation{EventType: fmt.Sprintf("%T", event), Duration: time.Since(started), Rows: min(rows, 1), Err: err})
	}
	return err
}
