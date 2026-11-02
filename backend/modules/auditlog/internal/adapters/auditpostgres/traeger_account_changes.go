package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
)

// TraegerAccountChanges reads the Träger-Büro account ledger (#5474) in the
// caller's transaction. Every statement filters on the Träger.
type TraegerAccountChanges struct {
	runtime Runtime
}

// NewTraegerAccountChanges builds the read over the caller's transaction.
func NewTraegerAccountChanges(runtime Runtime) *TraegerAccountChanges {
	return &TraegerAccountChanges{runtime: requireRuntime(runtime)}
}

// traegerAccountChangesQuery lists the changes of a Träger older than ?1
// (0 starts at the newest). The reads of the member list are no change.
const traegerAccountChangesQuery = `SELECT change.id, change.created_at, change.actor_account_id, change.action,
		change.target_account_id, change.target_email, change.roles_before, change.roles_after
	FROM audit.traeger_account_changes AS change
	WHERE change.organization_id = ?0 AND change.action <> 'members_listed' AND (?1 = 0 OR change.id < ?1)
	ORDER BY change.id DESC
	LIMIT ?2`

// ListTraegerAccountChanges lists the changes of the Träger, newest first.
func (r *TraegerAccountChanges) ListTraegerAccountChanges(ctx context.Context, organizationID, beforeID int64, limit int) ([]auditlog.TraegerAccountChange, error) {
	if organizationID <= 0 {
		return nil, errors.New("audit Träger account changes: the Träger is required")
	}
	result := []auditlog.TraegerAccountChange{}
	if limit <= 0 {
		return result, nil
	}
	db, err := runtimeDB(ctx, r.runtime)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID              int64     `bun:"id"`
		CreatedAt       time.Time `bun:"created_at"`
		ActorAccountID  int64     `bun:"actor_account_id"`
		Action          string    `bun:"action"`
		TargetAccountID *int64    `bun:"target_account_id"`
		TargetEmail     *string   `bun:"target_email"`
		RolesBefore     []string  `bun:"roles_before,array"`
		RolesAfter      []string  `bun:"roles_after,array"`
	}
	if err := db.NewRaw(traegerAccountChangesQuery, organizationID, max(beforeID, 0), limit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("audit Träger account changes: list: %w", err)
	}
	for _, row := range rows {
		result = append(result, auditlog.TraegerAccountChange{
			ID: row.ID, OccurredAt: row.CreatedAt, ActorAccountID: row.ActorAccountID, Action: row.Action,
			TargetAccountID: row.TargetAccountID, TargetEmail: row.TargetEmail,
			RolesBefore: nonNilRoles(row.RolesBefore), RolesAfter: nonNilRoles(row.RolesAfter),
		})
	}
	return result, nil
}

func nonNilRoles(roles []string) []string {
	if roles == nil {
		return []string{}
	}
	return roles
}
