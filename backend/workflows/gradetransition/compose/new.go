// Package compose wires the grade transition workflow over the owner
// capabilities and the shared tenant runtime (#2711).
package compose

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/uptrace/bun"
)

// Dependencies name the owner capabilities the root binds: School Structure,
// Student Presence, the bound People Directory and School Membership of the
// running graph, the Timetable roster reconciliation, the recurrence gate and
// the offering-roster resync, plus the runtime hooks. The composition builds
// no other owner (#4004, ADR 0059 point 6).
type Dependencies struct {
	DB         *bun.DB
	Structure  gradetransition.Structure
	Presence   gradetransition.Presence
	Directory  gradetransition.Directory
	Membership gradetransition.Membership
	Rosters    gradetransition.Rosters
	// LockRecurrenceWrites is the tenant-wide recurrence gate every timetable
	// writer takes; apply and revert hold it after the class-writes gate.
	LockRecurrenceWrites func(context.Context) error
	// ResyncOfferingRosters re-reconciles the offering-sourced timetable
	// templates from the given calendar day after the class rewrite.
	ResyncOfferingRosters func(context.Context, calendar.Date) error
	Logger                *slog.Logger
	// Audit appends the class-list rewrites in the caller's transaction, as
	// the records ClassListEntryRecord builds; the root binds both.
	Audit                AuditLog
	ClassListEntryRecord func(gradetransition.Actor, gradetransition.ClassListEntryAudit) any
	Clock                func() time.Time
}

// AuditLog is the Audit owner's append command.
type AuditLog interface {
	Append(context.Context, any) error
}

// New constructs the workflow over the ports already bound by the root.
func New(deps gradetransition.Dependencies) (*gradetransition.Workflow, error) {
	return gradetransition.New(deps)
}
