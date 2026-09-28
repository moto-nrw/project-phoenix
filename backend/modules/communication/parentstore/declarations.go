package parentstore

import (
	"context"
	"errors"
	"time"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/parentpostgres"
	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
)

// Erklärungen (#3430): the declaration half of the announcement repository.
// Versions and submissions are the store's rows; children and signers are
// audience reads.

func (r *announcementRepository) LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*usersModels.DeclarationVersion, error) {
	value, err := r.store.LatestDeclarationVersion(ctx, tenantID, announcementID)
	if err != nil || value == nil {
		return nil, err
	}
	return declarationVersionModel(value), nil
}

func (r *announcementRepository) ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.DeclarationVersion, error) {
	values, err := r.store.ListDeclarationVersions(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	out := make([]*usersModels.DeclarationVersion, 0, len(values))
	for _, value := range values {
		out = append(out, declarationVersionModel(value))
	}
	return out, nil
}

func (r *announcementRepository) LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*usersModels.DeclarationVersion, error) {
	values, err := r.store.LatestDeclarationVersions(ctx, announcementIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*usersModels.DeclarationVersion, len(values))
	for id, value := range values {
		out[id] = declarationVersionModel(value)
	}
	return out, nil
}

func (r *announcementRepository) InsertDeclarationVersion(ctx context.Context, version *usersModels.DeclarationVersion) error {
	value := &domain.DeclarationVersion{
		TenantID: version.TenantID, AnnouncementID: version.AnnouncementID, VersionNo: version.VersionNo,
		Title: version.Title, Body: version.Body, Kind: version.Kind, ContentHash: version.ContentHash,
		PublishedAt: version.PublishedAt,
	}
	for _, a := range version.Attachments {
		value.Attachments = append(value.Attachments, domain.DeclarationAttachmentDigest(a))
	}
	if err := r.store.InsertDeclarationVersion(ctx, value); err != nil {
		if errors.Is(err, parentpostgres.ErrDeclarationVersionConflict) {
			return usersModels.ErrDeclarationVersionConflict
		}
		return err
	}
	version.ID = value.ID
	version.TenantID = value.TenantID
	return nil
}

func (r *announcementRepository) DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]usersModels.AnnouncementDeclarationSettings, error) {
	values, err := r.store.DeclarationSettings(ctx, announcementIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]usersModels.AnnouncementDeclarationSettings, len(values))
	for id, value := range values {
		out[id] = usersModels.AnnouncementDeclarationSettings(value)
	}
	return out, nil
}

func (r *announcementRepository) CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error) {
	return r.store.CountDeclarationSubmissions(ctx, tenantID, announcementID)
}

func (r *announcementRepository) ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.DeclarationSubmission, error) {
	values, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	return declarationSubmissionModels(values), err
}

func (r *announcementRepository) ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs, studentIDs []int64) ([]*usersModels.DeclarationSubmission, error) {
	values, err := r.store.ListDeclarationSubmissionsForStudents(ctx, announcementIDs, studentIDs)
	return declarationSubmissionModels(values), err
}

func (r *announcementRepository) LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error {
	return r.store.LockDeclarationChild(ctx, announcementID, studentID)
}

func (r *announcementRepository) InsertDeclarationSubmission(ctx context.Context, submission *usersModels.DeclarationSubmission) error {
	value := domain.DeclarationSubmission(*submission)
	if err := r.store.InsertDeclarationSubmission(ctx, &value); err != nil {
		return err
	}
	submission.ID = value.ID
	submission.TenantID = value.TenantID
	return nil
}

func (r *announcementRepository) DeclarationChildren(ctx context.Context, tenantID, announcementID int64) ([]*usersModels.DeclarationChild, error) {
	submissions, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	values, err := r.audience.DeclarationChildren(ctx, tenantID, announcementID, declarationHistoryLinks(submissions))
	return declarationChildModels(values), err
}

func (r *announcementRepository) DeclarationSigners(ctx context.Context, tenantID int64, studentIDs []int64) ([]*usersModels.DeclarationSigner, error) {
	values, err := r.audience.DeclarationSigners(ctx, tenantID, studentIDs)
	return declarationSignerModels(values), err
}

func (r *announcementRepository) DeclarationChildrenForAccount(ctx context.Context, accountID int64, announcementIDs []int64) ([]*usersModels.DeclarationChild, error) {
	submissions, err := r.store.ListDeclarationSubmissionsForAccountAndAnnouncements(ctx, accountID, announcementIDs)
	if err != nil {
		return nil, err
	}
	values, err := r.audience.DeclarationChildrenForAccount(ctx, accountID, announcementIDs, declarationHistoryLinks(submissions))
	return declarationChildModels(values), err
}

func (r *announcementRepository) DeclarationSignersForStudents(ctx context.Context, studentIDs []int64) ([]*usersModels.DeclarationSigner, error) {
	values, err := r.audience.DeclarationSignersForStudents(ctx, studentIDs)
	return declarationSignerModels(values), err
}

func (r *announcementRepository) HoldDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*usersModels.DeclarationSignerContext, error) {
	value, err := r.audience.HoldDeclarationSigner(ctx, tenantID, announcementID, accountID, studentID)
	if err != nil || value == nil {
		return nil, err
	}
	signer := usersModels.DeclarationSignerContext(*value)
	return &signer, nil
}

func (r *announcementRepository) HoldHistoricalDeclarationSigner(ctx context.Context, tenantID, announcementID, accountID, studentID int64) (*usersModels.DeclarationSignerContext, error) {
	submissions, err := r.store.ListDeclarationSubmissions(ctx, tenantID, announcementID)
	if err != nil || !hasDeclarationSubmission(submissions, accountID, studentID) {
		return nil, err
	}
	value, err := r.audience.HoldHistoricalDeclarationSigner(ctx, tenantID, accountID, studentID)
	if err != nil || value == nil {
		return nil, err
	}
	signer := usersModels.DeclarationSignerContext(*value)
	return &signer, nil
}

func declarationHistoryLinks(submissions []*domain.DeclarationSubmission) []domain.DeclarationHistoryLink {
	links := make([]domain.DeclarationHistoryLink, 0, len(submissions))
	seen := make(map[domain.DeclarationHistoryLink]struct{}, len(submissions))
	for _, submission := range submissions {
		link := domain.DeclarationHistoryLink{
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

func declarationVersionModel(value *domain.DeclarationVersion) *usersModels.DeclarationVersion {
	version := &usersModels.DeclarationVersion{
		ID: value.ID, TenantID: value.TenantID, AnnouncementID: value.AnnouncementID, VersionNo: value.VersionNo,
		Title: value.Title, Body: value.Body, Kind: value.Kind, ContentHash: value.ContentHash,
		PublishedAt: value.PublishedAt, Attachments: make([]usersModels.DeclarationAttachmentDigest, 0, len(value.Attachments)),
	}
	for _, a := range value.Attachments {
		version.Attachments = append(version.Attachments, usersModels.DeclarationAttachmentDigest(a))
	}
	return version
}

func declarationSubmissionModels(values []*domain.DeclarationSubmission) []*usersModels.DeclarationSubmission {
	out := make([]*usersModels.DeclarationSubmission, 0, len(values))
	for _, value := range values {
		submission := usersModels.DeclarationSubmission(*value)
		out = append(out, &submission)
	}
	return out
}

func declarationChildModels(values []*domain.DeclarationChild) []*usersModels.DeclarationChild {
	out := make([]*usersModels.DeclarationChild, 0, len(values))
	for _, value := range values {
		child := usersModels.DeclarationChild(*value)
		out = append(out, &child)
	}
	return out
}

func declarationSignerModels(values []*domain.DeclarationSigner) []*usersModels.DeclarationSigner {
	out := make([]*usersModels.DeclarationSigner, 0, len(values))
	for _, value := range values {
		signer := usersModels.DeclarationSigner(*value)
		out = append(out, &signer)
	}
	return out
}
