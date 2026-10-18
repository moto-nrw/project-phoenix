package settings

import (
	"context"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Existing setting identifiers exposed for platform-contract consumers.
const (
	KeyParentMessageStaffNameVisible = configModel.KeyParentMessageStaffNameVisible
	KeyGDPRStaffMessageRetentionDays = configModel.KeyGDPRStaffMessageRetentionDays
	KeyNotificationsDispatchEnabled  = configModel.KeyNotificationsDispatchEnabled
)

// ParentMessagingValueSource is the resolver slice used by ParentMessagingValues.
type ParentMessagingValueSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// ParentMessagingValues exposes canonical settings without moving their keys to consumers.
type ParentMessagingValues interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ParentMessageStaffNameVisible(ctx context.Context) (bool, error)
}

type parentMessagingValues struct{ source ParentMessagingValueSource }

// NewParentMessagingValues binds the same resolver, preserving a missing dependency as nil.
func NewParentMessagingValues(source ParentMessagingValueSource) ParentMessagingValues {
	if source == nil {
		return nil
	}
	return parentMessagingValues{source: source}
}

func (v parentMessagingValues) ResolveBool(ctx context.Context, key string) (bool, error) {
	return v.source.ResolveBool(ctx, key)
}

func (v parentMessagingValues) ParentMessageStaffNameVisible(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyParentMessageStaffNameVisible)
}

// ParentAnnouncementValueSource is the resolver slice used by ParentAnnouncementValues.
type ParentAnnouncementValueSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)
}

// ParentAnnouncementValues exposes canonical settings without moving their keys to consumers.
type ParentAnnouncementValues interface {
	ParentNewsEnabled(ctx context.Context) (bool, error)
	GetLoginImageURL(ctx context.Context, tenantID int64) (string, error)
}

type parentAnnouncementValues struct{ source ParentAnnouncementValueSource }

// NewParentAnnouncementValues binds the same resolver, preserving a missing dependency as nil.
func NewParentAnnouncementValues(source ParentAnnouncementValueSource) ParentAnnouncementValues {
	if source == nil {
		return nil
	}
	return parentAnnouncementValues{source: source}
}

func (v parentAnnouncementValues) ParentNewsEnabled(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyParentNewsEnabled)
}

func (v parentAnnouncementValues) GetLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	return v.source.GetLoginImageURL(ctx, tenantID)
}

// StaffMessagingValueSource is the resolver slice used by StaffMessagingValues.
type StaffMessagingValueSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveInt(ctx context.Context, key string) (int, error)
}

// StaffMessagingValues exposes canonical settings without moving their keys to consumers.
type StaffMessagingValues interface {
	StaffMessagingEnabled(ctx context.Context) (bool, error)
	StaffMessageRetentionDays(ctx context.Context) (int, error)
	StaffMessagingEnabledKey() string
	StaffMessageRetentionDaysKey() string
}

type staffMessagingValues struct{ source StaffMessagingValueSource }

// NewStaffMessagingValues binds the same resolver, preserving a missing dependency as nil.
func NewStaffMessagingValues(source StaffMessagingValueSource) StaffMessagingValues {
	if source == nil {
		return nil
	}
	return staffMessagingValues{source: source}
}

func (v staffMessagingValues) StaffMessagingEnabled(ctx context.Context) (bool, error) {
	return v.source.ResolveBool(ctx, configModel.KeyStaffMessagingEnabled)
}

func (v staffMessagingValues) StaffMessageRetentionDays(ctx context.Context) (int, error) {
	return v.source.ResolveInt(ctx, configModel.KeyGDPRStaffMessageRetentionDays)
}

func (v staffMessagingValues) StaffMessagingEnabledKey() string {
	return configModel.KeyStaffMessagingEnabled
}

func (v staffMessagingValues) StaffMessageRetentionDaysKey() string {
	return configModel.KeyGDPRStaffMessageRetentionDays
}

// ParentEventValueSource is the resolver slice used by ParentEventValues.
type ParentEventValueSource interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
}

// ParentEventValues exposes canonical settings without moving their keys to consumers.
type ParentEventValues interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ParentNotesEnabledForTenant(ctx context.Context, tenantID int64) (bool, error)
}

type parentEventValues struct{ source ParentEventValueSource }

// NewParentEventValues binds the same resolver, preserving a missing dependency as nil.
func NewParentEventValues(source ParentEventValueSource) ParentEventValues {
	if source == nil {
		return nil
	}
	return parentEventValues{source: source}
}

func (v parentEventValues) ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error) {
	return v.source.ResolveBoolForTenant(ctx, tenantID, key)
}

func (v parentEventValues) ParentNotesEnabledForTenant(ctx context.Context, tenantID int64) (bool, error) {
	return v.source.ResolveBoolForTenant(ctx, tenantID, configModel.KeyParentNotesEnabled)
}

// ParentMessageRetentionValueSource is the resolver slice used by ParentMessageRetentionValues.
type ParentMessageRetentionValueSource interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// ParentMessageRetentionValues exposes canonical settings without moving their keys to consumers.
type ParentMessageRetentionValues interface {
	ParentMessageRetentionDays(ctx context.Context) (int, error)
}

type parentMessageRetentionValues struct {
	source ParentMessageRetentionValueSource
}

// NewParentMessageRetentionValues binds the same resolver, preserving a missing dependency as nil.
func NewParentMessageRetentionValues(source ParentMessageRetentionValueSource) ParentMessageRetentionValues {
	if source == nil {
		return nil
	}
	return parentMessageRetentionValues{source: source}
}

func (v parentMessageRetentionValues) ParentMessageRetentionDays(ctx context.Context) (int, error) {
	return v.source.ResolveInt(ctx, configModel.KeyGDPRParentMessageRetentionDays)
}
