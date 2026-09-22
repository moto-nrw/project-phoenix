// Package schoolsetupview implements the tenant-safe read projection behind the
// onboarding wizard for new schools (#2832, ADR 0040): whether the school has
// invited staff and created rooms, groups, children and parent invitations.
//
// It is a read-only projection because every answer reads another owner's
// table: Facilities, Identity & Access, School Structure and School
// Membership. It never writes and owns no table. The one statement is a
// compile-time constant so the architecture evaluator can read which tables it
// touches.
package schoolsetupview

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

// Database resolves the caller's ambient tenant transaction.
type Database func(context.Context) (bun.IDB, error)

// Facts are the existence checks the wizard derives its progress from.
type Facts struct {
	// StaffInvited: the school itself invited somebody. Invitations the
	// operator sent while provisioning the school carry no creator and do not
	// count, so the first admin's own invitation never finishes the step.
	StaffInvited bool `bun:"staff_invited"`
	// RoomCreated ignores the system rooms (Schulhof, WC) moto creates itself.
	RoomCreated  bool `bun:"room_created"`
	GroupCreated bool `bun:"group_created"`
	// StudentEnrolled counts active, not deleted school memberships.
	StudentEnrolled bool `bun:"student_enrolled"`
	GuardianInvited bool `bun:"guardian_invited"`
}

// progressQuery answers all five questions in one statement. Every EXISTS
// filters tenant_id explicitly on top of RLS: the predicate is the invariant
// ADR 0040 records for tenant_safe.
const progressQuery = `
SELECT
	EXISTS (
		SELECT 1 FROM auth.invitation_tokens
		WHERE tenant_id = ?0 AND created_by IS NOT NULL
	) AS staff_invited,
	EXISTS (
		SELECT 1 FROM facilities.rooms
		WHERE tenant_id = ?0 AND is_system = FALSE
	) AS room_created,
	EXISTS (
		SELECT 1 FROM education.groups
		WHERE tenant_id = ?0
	) AS group_created,
	EXISTS (
		SELECT 1 FROM users.student_school_memberships
		WHERE tenant_id = ?0 AND status = 'active' AND deleted_at IS NULL
	) AS student_enrolled,
	EXISTS (
		SELECT 1 FROM auth.guardian_invitations
		WHERE tenant_id = ?0
	) AS guardian_invited`

var errTenantRequired = errors.New("school setup view: tenant is required")

// Projection answers the wizard's progress read.
type Projection struct {
	database Database
}

// New builds the projection over the ambient transaction runtime.
func New(database Database) *Projection {
	if database == nil {
		panic("school setup view: database runtime is required")
	}
	return &Projection{database: database}
}

// Progress returns the facts for the school. The caller must run inside the
// school's tenant transaction.
func (p *Projection) Progress(ctx context.Context, tenantID int64) (Facts, error) {
	if tenantID <= 0 {
		return Facts{}, errTenantRequired
	}
	db, err := p.database(ctx)
	if err != nil {
		return Facts{}, fmt.Errorf("school setup view: %w", err)
	}
	var facts Facts
	if err := db.NewRaw(progressQuery, tenantID).Scan(ctx, &facts); err != nil {
		return Facts{}, fmt.Errorf("school setup view: read progress: %w", err)
	}
	return facts, nil
}
