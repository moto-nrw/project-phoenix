package application

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
)

// AttachmentDigests reads every live attachment of an announcement through
// the caller's digest function, oldest id first. File Storage only reads; the
// caller owns the hash. It runs in the caller's transaction:
// an Erklärung freezes these digests together with its publication (#3430),
// so a file that cannot be read fails the publication instead of producing a
// version whose attachment nobody can verify.
func (s *Service) AttachmentDigests(ctx context.Context, announcementID int64, digest func(io.Reader) (string, int64, error)) (digests []filestorage.AttachmentDigest, err error) {
	err = s.runRead(ctx, "attachment_digests", func(ctx context.Context, o *op) error {
		if announcementID <= 0 {
			return fmt.Errorf("%w: announcement id is required", filestorage.ErrInvalid)
		}
		if digest == nil {
			return errors.New("attachment digests: digest function is required")
		}
		attachments, stats, err := s.deps.Attachments.ListByOwner(ctx, announcementID)
		o.add(stats)
		if err != nil {
			return err
		}
		tenantID := s.deps.Tx.TenantID(ctx)
		digests = make([]filestorage.AttachmentDigest, 0, len(attachments))
		for i := len(attachments) - 1; i >= 0; i-- { // ListByOwner is newest first
			entry, err := s.digestAttachment(ctx, tenantID, attachments[i], digest)
			if err != nil {
				return err
			}
			digests = append(digests, entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return digests, nil
}

func (s *Service) digestAttachment(ctx context.Context, tenantID int64, attachment domain.Document, digest func(io.Reader) (string, int64, error)) (filestorage.AttachmentDigest, error) {
	object, err := s.openObject(ctx, s.attachments(), tenantID, attachment)
	if err != nil {
		return filestorage.AttachmentDigest{}, err
	}
	defer func() { _ = object.Close() }()
	sum, size, err := digest(object)
	if err != nil {
		return filestorage.AttachmentDigest{}, fmt.Errorf("digest attachment %d: %w", attachment.ID, err)
	}
	return filestorage.AttachmentDigest{
		AttachmentID: attachment.ID, Filename: attachment.FilenameDisplay, ContentType: attachment.ContentType,
		SizeBytes: size, SHA256: sum,
	}, nil
}
