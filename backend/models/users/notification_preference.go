package users

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// NotificationPreference records that one account agreed (or explicitly
// declined) to receive one type of notification at one school.
//
// Absence of a row means "not agreed". A row with Enabled=false is a deliberate
// opt-out and is kept as such, so a later change of defaults cannot silently
// overrule a decision someone already made.
type NotificationPreference struct {
	base.Model `bun:"schema:users,table:notification_preferences"`
	base.TenantModel
	AccountID        int64  `bun:"account_id,notnull" json:"account_id"`
	NotificationType string `bun:"notification_type,notnull" json:"notification_type"`
	Enabled          bool   `bun:"enabled,notnull" json:"enabled"`
}

// Validate ensures the preference identifies an account and a type.
func (p *NotificationPreference) Validate() error {
	if p.AccountID <= 0 {
		return errors.New("account ID is required")
	}
	if p.NotificationType == "" {
		return errors.New("notification type is required")
	}
	return nil
}

// NotificationPreferenceRepository stores per-account notification opt-ins.
type NotificationPreferenceRepository interface {
	base.CRUDRepository[*NotificationPreference]

	// Upsert records one decision, overwriting any previous one for the same
	// (tenant, account, type).
	Upsert(ctx context.Context, pref *NotificationPreference) error

	// ListByAccount returns every stored decision of one account in the current
	// tenant. Types the account never touched are simply absent.
	ListByAccount(ctx context.Context, accountID int64) ([]*NotificationPreference, error)

	// FilterOptedIn narrows a producer's candidate recipients to those who
	// agreed to the given type. This is the enforcement point for "nothing is
	// delivered without consent": an empty input, or nobody having agreed,
	// returns an empty result and never the full list.
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)

	// FilterNotOptedOut is the mirror of FilterOptedIn: it returns the candidates
	// MINUS those who explicitly declined the type (a row with Enabled=false).
	// A missing row does not filter anybody out. This is for channels that were
	// delivered before consent existed and must not be silenced by the absence
	// of a decision; consent-first channels use FilterOptedIn.
	FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)

	// DisableAllForAccount switches the named stored decisions of one account
	// off. Types with no row stay absent — they are already off.
	DisableAllForAccount(ctx context.Context, accountID int64, notificationTypes []string) error
}
