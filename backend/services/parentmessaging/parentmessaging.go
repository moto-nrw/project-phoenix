// Package parentmessaging is the consumer-facing contract of the parent-OGS
// event stream: the pill vocabulary the request domains emit, the settings
// gate every read path shares, and the Emitter shell those domains hold.
//
// The persistence, transaction, and realtime work behind the shell belongs to
// Communication (modules/communication/internal/parentmessages) and reaches
// this package only through the Events port below. Keeping the shell free of
// models, ORM, tenant, and realtime imports is what lets the request domains
// keep their existing emitter dependency while Communication owns the tables.
package parentmessaging

import (
	"context"
	"errors"
	"log/slog"
)

// parentNotesSettingKey is the tenant setting that switches parent-OGS
// messaging on. It mirrors the settings registry's key; a Communication test
// pins the two together.
const parentNotesSettingKey = "operations.parent_notes_enabled"

// Pill vocabulary. The values are the users.parent_messages column values and
// mirror the People Directory model constants; a Communication test pins the
// two together so the request domains and the owner cannot drift.
const (
	// EventRequestCreated marks a request submission pill.
	EventRequestCreated = "request_created"
	// EventRequestStatus marks a request decision, withdrawal, or close pill.
	EventRequestStatus = "request_status"

	// ActorGuardian attributes a pill to the parent side.
	ActorGuardian = "guardian"
	// ActorStaff attributes a pill to the OGS side.
	ActorStaff = "staff"

	// RequestStatusOpen through RequestStatusWithdrawn are the request
	// outcome vocabulary of the pills.
	RequestStatusOpen      = "offen"
	RequestStatusDone      = "erledigt"
	RequestStatusRejected  = "abgelehnt"
	RequestStatusWithdrawn = "zurueckgezogen"
)

// ErrEmitterNotConfigured reports an emitter without its Communication side.
var ErrEmitterNotConfigured = errors.New("parentmessaging: emitter not configured")

// SettingsResolver resolves a boolean setting inside the current tenant
// transaction. Satisfied by services/config.SettingsService; declared here as a
// narrow interface so this shared core does not depend on the whole settings
// service (and to keep the import graph acyclic).
type SettingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// TenantSettingsResolver resolves a boolean setting for an explicit tenant,
// opening its own tenant transaction. Used by callers OUTSIDE tenant middleware
// (the cross-tenant parent badge, the unauthenticated tenant-resolve handler).
type TenantSettingsResolver interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
}

// MessagingEnabled reports whether parent messaging is on for the CURRENT tenant
// transaction. It is the single home for BOTH the setting key and the fail-OPEN
// direction, so every read- and write-path across the staff and parent services
// agrees: a transient settings-resolve error counts as ENABLED.
//
// Failing open is deliberate and uniform. The unread badge, the inbox/row pills,
// the compose-button visibility, and the reply path all gate on this one flag; if
// they disagreed during a config-DB blip the staffer would see and read unread
// messages while the "Neue Nachricht" button vanished and every reply 500'd — a
// half-disabled UI that contradicts the still-rendering inbox. Over-permitting on
// a rare blip beats that split brain: messaging is a soft, non-destructive feature
// flag, not a security boundary. (Unlike the photos/NFC flags, which fail closed
// for opt-out safety because enabling them surfaces data a school opted out of.)
func MessagingEnabled(ctx context.Context, settings SettingsResolver, logger *slog.Logger) bool {
	enabled, err := settings.ResolveBool(ctx, parentNotesSettingKey)
	if err != nil {
		loggerOr(logger).Warn("parent messaging: resolve enabled failed, failing open (counting as enabled)",
			slog.String("error", err.Error()),
		)
		return true
	}
	return enabled
}

// MessagingEnabledForTenant is MessagingEnabled for an explicit tenant, for
// callers running outside tenant middleware. Same key, same fail-OPEN contract.
func MessagingEnabledForTenant(ctx context.Context, settings TenantSettingsResolver, tenantID int64, logger *slog.Logger) bool {
	enabled, err := settings.ResolveBoolForTenant(ctx, tenantID, parentNotesSettingKey)
	if err != nil {
		loggerOr(logger).Warn("parent messaging: resolve enabled for tenant failed, failing open (counting as enabled)",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return enabled
}

func loggerOr(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
