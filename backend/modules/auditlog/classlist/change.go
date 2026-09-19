// Package classlist defines Audit's class-list change event contract.
package classlist

import "time"

const ClassListEntryActionCreated = "created"

// ClassListEntryChange is appended in the same transaction as the entry change.
type ClassListEntryChange struct {
	TenantID, EntryID          int64
	Action, OldValue, NewValue string
	MatchedStudentID           *int64
	ChangedBy                  int64
	OccurredAt                 time.Time
}

func (c *ClassListEntryChange) GetTenantID() int64   { return c.TenantID }
func (c *ClassListEntryChange) SetTenantID(id int64) { c.TenantID = id }
