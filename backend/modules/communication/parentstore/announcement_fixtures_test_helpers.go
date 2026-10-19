//go:build testhooks

package parentstore

import (
	"context"
	"time"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// ParentAnnouncementFixtures binds native test values without replacing the
// store's validation, transactions, defaults or persisted-field copy-back.
type ParentAnnouncementFixtures struct {
	repo recordports.ParentAnnouncementRepository
}

func NewParentAnnouncementFixtures(repo recordports.ParentAnnouncementRepository) ParentAnnouncementFixtures {
	return ParentAnnouncementFixtures{repo: repo}
}

func (f ParentAnnouncementFixtures) Create(ctx context.Context, value *communication.ParentAnnouncementFixture) error {
	row := announcementFixtureToRow(value)
	if err := f.repo.Create(ctx, row); err != nil {
		return err
	}
	*value = *announcementFixtureFromRow(row)
	return nil
}

func (f ParentAnnouncementFixtures) FindByID(ctx context.Context, id int64) (*communication.ParentAnnouncementFixture, error) {
	row, err := f.repo.FindByID(ctx, id)
	return announcementFixtureFromRow(row), err
}

func (f ParentAnnouncementFixtures) Delete(ctx context.Context, id int64) error {
	return f.repo.Delete(ctx, id)
}
func (f ParentAnnouncementFixtures) SetPublished(ctx context.Context, id int64, at *time.Time) error {
	return f.repo.SetPublished(ctx, id, at)
}

func (f ParentAnnouncementFixtures) ReplaceTargets(ctx context.Context, tenantID, id int64, values []*communication.ParentAnnouncementTargetFixture) error {
	rows := announcementTargetFixtureRows(values)
	err := f.repo.ReplaceTargets(ctx, tenantID, id, rows)
	for i, row := range rows {
		if row != nil {
			*values[i] = *announcementTargetFixture(row)
		}
	}
	return err
}

func (f ParentAnnouncementFixtures) ReplaceOptions(ctx context.Context, tenantID, id int64, values []*communication.ParentAnnouncementOptionFixture) error {
	rows := announcementOptionFixtureRows(values)
	err := f.repo.ReplaceOptions(ctx, tenantID, id, rows)
	for i, row := range rows {
		if row != nil {
			*values[i] = *announcementOptionFixture(row)
		}
	}
	return err
}

func (f ParentAnnouncementFixtures) ListOptions(ctx context.Context, id int64) ([]*communication.ParentAnnouncementOptionFixture, error) {
	rows, err := f.repo.ListOptions(ctx, id)
	if rows == nil {
		return nil, err
	}
	values := make([]*communication.ParentAnnouncementOptionFixture, len(rows))
	for i, row := range rows {
		values[i] = announcementOptionFixture(row)
	}
	return values, err
}

// Declarations use the same stored records and canonical owner encoding.
func (f ParentAnnouncementFixtures) InsertDeclarationVersion(ctx context.Context, value *communication.PortalDeclarationVersion) error {
	row := value
	err := f.repo.InsertDeclarationVersion(ctx, row)
	if value != nil && row != nil {
		*value = *row
	}
	return err
}
func (f ParentAnnouncementFixtures) ListDeclarationSubmissions(ctx context.Context, tenantID, id int64) ([]*communication.PortalDeclarationSubmission, error) {
	rows, err := f.repo.ListDeclarationSubmissions(ctx, tenantID, id)
	return rows, err
}

func announcementFixtureToRow(value *communication.ParentAnnouncementFixture) *communication.PortalParentAnnouncement {
	if value == nil {
		return nil
	}
	row := &communication.PortalParentAnnouncement{
		Title:                   value.Title,
		Body:                    value.Body,
		Priority:                value.Priority,
		LinkURL:                 value.LinkURL,
		RequiresAcknowledgement: value.RequiresAcknowledgement,
		SendEmail:               value.SendEmail,
		PublishedAt:             value.PublishedAt,
		ExpiresAt:               value.ExpiresAt,
		Active:                  value.Active,
		CreatedBy:               value.CreatedBy,
		ResponseType:            value.ResponseType,
		ResponseDeadline:        value.ResponseDeadline,
		DeliveryMode:            value.DeliveryMode,
		EmailAudience:           value.EmailAudience,
		SystemKind:              value.SystemKind,
		ReminderAt:              value.ReminderAt,
		ReminderText:            value.ReminderText,
		ReminderSentAt:          value.ReminderSentAt,
		Targets:                 announcementTargetFixtureRows(value.Targets), Options: announcementOptionFixtureRows(value.Options),
		Declaration: communication.PortalAnnouncementDeclarationSettings(value.Declaration),
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = value.ID, value.TenantID, value.CreatedAt, value.UpdatedAt
	return row
}
func announcementFixtureFromRow(row *communication.PortalParentAnnouncement) *communication.ParentAnnouncementFixture {
	if row == nil {
		return nil
	}
	value := &communication.ParentAnnouncementFixture{
		ID:                      row.ID,
		TenantID:                row.TenantID,
		CreatedAt:               row.CreatedAt,
		UpdatedAt:               row.UpdatedAt,
		Title:                   row.Title,
		Body:                    row.Body,
		Priority:                row.Priority,
		LinkURL:                 row.LinkURL,
		RequiresAcknowledgement: row.RequiresAcknowledgement,
		SendEmail:               row.SendEmail,
		PublishedAt:             row.PublishedAt,
		ExpiresAt:               row.ExpiresAt,
		Active:                  row.Active,
		CreatedBy:               row.CreatedBy,
		ResponseType:            row.ResponseType,
		ResponseDeadline:        row.ResponseDeadline,
		DeliveryMode:            row.DeliveryMode,
		EmailAudience:           row.EmailAudience,
		SystemKind:              row.SystemKind,
		ReminderAt:              row.ReminderAt,
		ReminderText:            row.ReminderText,
		ReminderSentAt:          row.ReminderSentAt,
		Declaration:             communication.ParentDeclarationSettings(row.Declaration),
	}
	if row.Targets != nil {
		value.Targets = make([]*communication.ParentAnnouncementTargetFixture, len(row.Targets))
		for i, r := range row.Targets {
			value.Targets[i] = announcementTargetFixture(r)
		}
	}
	if row.Options != nil {
		value.Options = make([]*communication.ParentAnnouncementOptionFixture, len(row.Options))
		for i, r := range row.Options {
			value.Options[i] = announcementOptionFixture(r)
		}
	}
	return value
}
func announcementTargetFixtureRows(values []*communication.ParentAnnouncementTargetFixture) []*communication.PortalParentAnnouncementTarget {
	if values == nil {
		return nil
	}
	rows := make([]*communication.PortalParentAnnouncementTarget, len(values))
	for i, v := range values {
		if v == nil {
			continue
		}
		r := &communication.PortalParentAnnouncementTarget{
			ID:             v.ID,
			AnnouncementID: v.AnnouncementID,
			TargetType:     v.TargetType,
			TargetRefID:    v.TargetRefID,
			TargetRefText:  v.TargetRefText,
			CreatedAt:      v.CreatedAt,
		}
		r.TenantID = v.TenantID
		rows[i] = r
	}
	return rows
}
func announcementTargetFixture(row *communication.PortalParentAnnouncementTarget) *communication.ParentAnnouncementTargetFixture {
	if row == nil {
		return nil
	}
	return &communication.ParentAnnouncementTargetFixture{
		ID:             row.ID,
		TenantID:       row.TenantID,
		AnnouncementID: row.AnnouncementID,
		TargetType:     row.TargetType,
		TargetRefID:    row.TargetRefID,
		TargetRefText:  row.TargetRefText,
		CreatedAt:      row.CreatedAt,
	}
}
func announcementOptionFixtureRows(values []*communication.ParentAnnouncementOptionFixture) []*communication.PortalParentAnnouncementOption {
	if values == nil {
		return nil
	}
	rows := make([]*communication.PortalParentAnnouncementOption, len(values))
	for i, v := range values {
		if v == nil {
			continue
		}
		r := &communication.PortalParentAnnouncementOption{
			ID:             v.ID,
			AnnouncementID: v.AnnouncementID,
			Label:          v.Label,
			Position:       v.Position,
			CreatedAt:      v.CreatedAt,
		}
		r.TenantID = v.TenantID
		rows[i] = r
	}
	return rows
}
func announcementOptionFixture(row *communication.PortalParentAnnouncementOption) *communication.ParentAnnouncementOptionFixture {
	if row == nil {
		return nil
	}
	return &communication.ParentAnnouncementOptionFixture{
		ID:             row.ID,
		TenantID:       row.TenantID,
		AnnouncementID: row.AnnouncementID,
		Label:          row.Label,
		Position:       row.Position,
		CreatedAt:      row.CreatedAt,
	}
}

func ParentAnnouncementPublishedError() error { return domain.ErrParentAnnouncementPublished }
