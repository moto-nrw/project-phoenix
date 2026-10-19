package parentstore

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	parentaudience "github.com/moto-nrw/project-phoenix/modules/parentannouncementaudience/contract"
)

// Erklärungen (#3430): the declaration half of the announcement repository.
// Versions and submissions are the store's rows; children and signers are
// audience reads.

func (r *announcementRepository) LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*communication.PortalDeclarationVersion, error) {
	value, err := r.store.LatestDeclarationVersion(ctx, tenantID, announcementID)
	if err != nil || value == nil {
		return nil, err
	}
	return declarationVersionModel(value), nil
}

func (r *announcementRepository) ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationVersion, error) {
	values, err := r.store.ListDeclarationVersions(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	out := make([]*communication.PortalDeclarationVersion, 0, len(values))
	for _, value := range values {
		out = append(out, declarationVersionModel(value))
	}
	return out, nil
}

func (r *announcementRepository) LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*communication.PortalDeclarationVersion, error) {
	values, err := r.store.LatestDeclarationVersions(ctx, announcementIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*communication.PortalDeclarationVersion, len(values))
	for id, value := range values {
		out[id] = declarationVersionModel(value)
	}
	return out, nil
}

func (r *announcementRepository) InsertDeclarationVersion(ctx context.Context, version *communication.PortalDeclarationVersion) error {
	value := &domain.DeclarationVersion{
		TenantID: version.TenantID, AnnouncementID: version.AnnouncementID, VersionNo: version.VersionNo,
		Title: version.Title, Body: version.Body, Kind: version.Kind, ContentHash: version.ContentHash,
		PublishedAt: version.PublishedAt,
	}
	for _, a := range version.Attachments {
		value.Attachments = append(value.Attachments, domain.DeclarationAttachmentDigest(a))
	}
	if err := r.store.InsertDeclarationVersion(ctx, value); err != nil {
		return err
	}
	version.ID = value.ID
	version.TenantID = value.TenantID
	return nil
}

func (r *announcementRepository) DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]communication.PortalAnnouncementDeclarationSettings, error) {
	values, err := r.store.DeclarationSettings(ctx, announcementIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]communication.PortalAnnouncementDeclarationSettings, len(values))
	for id, value := range values {
		out[id] = communication.PortalAnnouncementDeclarationSettings(value)
	}
	return out, nil
}

func (r *announcementRepository) CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error) {
	return r.store.CountDeclarationSubmissions(ctx, tenantID, announcementID)
}

func (r *announcementRepository) ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationSubmission, error) {
	values, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	return declarationSubmissionModels(values), err
}

func (r *announcementRepository) ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs, studentIDs []int64) ([]*communication.PortalDeclarationSubmission, error) {
	values, err := r.store.ListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
	return declarationSubmissionModels(values), err
}

func (r *announcementRepository) LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error {
	return r.store.LockDeclarationChild(ctx, announcementID, studentID)
}

func (r *announcementRepository) InsertDeclarationSubmission(ctx context.Context, submission *communication.PortalDeclarationSubmission) error {
	value := domain.DeclarationSubmission(*submission)
	if err := r.store.InsertDeclarationSubmission(ctx, &value); err != nil {
		return err
	}
	submission.ID = value.ID
	submission.TenantID = value.TenantID
	return nil
}

func (r *announcementRepository) DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*communication.PortalDeclarationChild, error) {
	submissions, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	values, err := r.audience.DeclarationChildren(ctx, tenantID, announcementID, declarationHistoryLinks(submissions))
	return declarationChildModels(values), err
}

func (r *announcementRepository) DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	values, err := r.audience.DeclarationSigners(ctx, tenantID, studentIDs)
	return declarationSignerModels(values), err
}

func (r *announcementRepository) DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*communication.PortalDeclarationChild, error) {
	submissions, err := r.store.ListDeclarationSubmissionsForAccountAndAnnouncements(ctx, accountID, announcementIDs)
	if err != nil {
		return nil, err
	}
	values, err := r.audience.DeclarationChildrenForAccount(ctx, accountID, announcementIDs, declarationHistoryLinks(submissions))
	return declarationChildModels(values), err
}

func (r *announcementRepository) DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*communication.PortalDeclarationSigner, error) {
	values, err := r.audience.DeclarationSignersForStudents(ctx, studentIDs)
	return declarationSignerModels(values), err
}

func (r *announcementRepository) HoldDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	value, err := r.audience.HoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
	if err != nil || value == nil {
		return nil, err
	}
	signer := communication.PortalDeclarationSignerContext(*value)
	return &signer, nil
}

func (r *announcementRepository) HoldHistoricalDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*communication.PortalDeclarationSignerContext, error) {
	submissions, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	if err != nil || !hasDeclarationSubmission(submissions, accountID, studentID) {
		return nil, err
	}
	value, err := r.audience.HoldHistoricalDeclarationSigner(ctx, tenantID, accountID, studentID)
	if err != nil || value == nil {
		return nil, err
	}
	signer := communication.PortalDeclarationSignerContext(*value)
	return &signer, nil
}

func declarationHistoryLinks(submissions []*domain.DeclarationSubmission) []parentaudience.DeclarationHistoryLink {
	links := make([]parentaudience.DeclarationHistoryLink, 0, len(submissions))
	seen := make(map[parentaudience.DeclarationHistoryLink]struct{}, len(submissions))
	for _, submission := range submissions {
		link := parentaudience.DeclarationHistoryLink{
			TenantID: submission.TenantID, AnnouncementID: submission.AnnouncementID, StudentID: submission.StudentID,
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		links = append(links, link)
	}
	return links
}

func hasDeclarationSubmission(submissions []*domain.DeclarationSubmission, accountID, studentID int64) bool {
	for _, submission := range submissions {
		if submission.StudentID == studentID && submission.AccountID != nil && *submission.AccountID == accountID {
			return true
		}
	}
	return false
}

func (r *announcementRepository) ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error) {
	return r.store.ReadOpenDeclarations(ctx, accountID, tenantIDs)
}

func declarationVersionModel(value *domain.DeclarationVersion) *communication.PortalDeclarationVersion {
	version := &communication.PortalDeclarationVersion{
		ID: value.ID, TenantID: value.TenantID, AnnouncementID: value.AnnouncementID, VersionNo: value.VersionNo,
		Title: value.Title, Body: value.Body, Kind: value.Kind, ContentHash: value.ContentHash,
		PublishedAt: value.PublishedAt, Attachments: make([]communication.PortalDeclarationAttachmentDigest, 0, len(value.Attachments)),
	}
	for _, a := range value.Attachments {
		version.Attachments = append(version.Attachments, communication.PortalDeclarationAttachmentDigest(a))
	}
	return version
}

func declarationSubmissionModels(values []*domain.DeclarationSubmission) []*communication.PortalDeclarationSubmission {
	out := make([]*communication.PortalDeclarationSubmission, 0, len(values))
	for _, value := range values {
		submission := communication.PortalDeclarationSubmission(*value)
		out = append(out, &submission)
	}
	return out
}

func declarationChildModels(values []*parentaudience.DeclarationChild) []*communication.PortalDeclarationChild {
	out := make([]*communication.PortalDeclarationChild, 0, len(values))
	for _, value := range values {
		child := communication.PortalDeclarationChild(*value)
		out = append(out, &child)
	}
	return out
}

func declarationSignerModels(values []*parentaudience.DeclarationSigner) []*communication.PortalDeclarationSigner {
	out := make([]*communication.PortalDeclarationSigner, 0, len(values))
	for _, value := range values {
		signer := communication.PortalDeclarationSigner(*value)
		out = append(out, &signer)
	}
	return out
}
