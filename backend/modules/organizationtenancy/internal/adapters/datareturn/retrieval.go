package datareturn

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// Retrieval serves a released package to the holder of its token. It runs
// as the application role.
type Retrieval struct {
	store  retrievalStore
	crypto organizationtenancy.DataReturnCrypto
	root   string
	now    func() time.Time
	logger *slog.Logger
	// recheckBytes is how much of the archive goes out between two checks
	// that the grant is still neither revoked nor purged.
	recheckBytes int64
}

// recheckEvery bounds what a revoked token still receives: one query per
// 8 MiB is cheap against the transfer itself.
const recheckEvery = 8 << 20

// NewRetrieval binds the retrieval to the application database and store.
func NewRetrieval(db bun.IDB, crypto organizationtenancy.DataReturnCrypto, root string, logger *slog.Logger) *Retrieval {
	if logger == nil {
		logger = slog.Default()
	}
	return &Retrieval{store: retrievalStore{db: db}, crypto: crypto, root: root, now: time.Now, logger: logger, recheckBytes: recheckEvery}
}

// Package answers what the token's package is, without spending a download.
func (s *Retrieval) Package(ctx context.Context, token string) (organizationtenancy.DataReturnPackage, error) {
	row, err := s.usableGrant(ctx, token)
	if err != nil {
		return organizationtenancy.DataReturnPackage{}, err
	}
	return row.toPackage(), nil
}

func (s *Retrieval) usableGrant(ctx context.Context, token string) (grantRow, error) {
	if !strings.HasPrefix(token, "mdr_") || len(token) > 128 {
		return grantRow{}, organizationtenancy.ErrDataReturnDenied
	}
	row, err := s.store.byLookup(ctx, s.crypto.TokenLookup(token))
	if err != nil {
		return row, err
	}
	if !row.usable(s.now()) {
		return row, organizationtenancy.ErrDataReturnDenied
	}
	return row, nil
}

// Download spends one download and streams the decrypted archive. The key
// opens only with the token; the archive is the one the grant names. A
// revocation or purge stops the transfer at the next check.
func (s *Retrieval) Download(ctx context.Context, token string, start func(organizationtenancy.DataReturnPackage) io.Writer) error {
	row, err := s.usableGrant(ctx, token)
	if err != nil {
		return err
	}
	key, err := s.crypto.OpenKey(token, row.WrappedKey)
	if err != nil {
		return organizationtenancy.ErrDataReturnDenied
	}
	file, err := os.Open(filepath.Join(s.root, row.ArchiveName+".mdr"))
	if err != nil {
		s.logger.ErrorContext(ctx, "data return archive missing",
			"grant_id", row.ID,
		)
		return organizationtenancy.ErrDataReturnArchiveMissing
	}
	defer func() { _ = file.Close() }()
	download, downloads, err := s.store.countDownload(ctx, row.ID, s.now())
	if err != nil {
		return err
	}
	row.Downloads = downloads
	guard := &openGrant{ctx: ctx, store: s.store, id: row.ID, every: s.recheckBytes}
	err = s.crypto.Decrypt(key, io.MultiWriter(guard, start(row.toPackage())), file)
	completed := err == nil && guard.written == row.ArchiveBytes
	if finishErr := s.store.finishDownload(context.WithoutCancel(ctx), download, s.now(), completed, guard.written); finishErr != nil {
		s.logger.ErrorContext(ctx, "data return download not recorded",
			"grant_id", row.ID,
			"error", finishErr,
		)
	}
	s.logger.InfoContext(ctx, "data return download",
		"grant_id", row.ID,
		"completed", completed,
		"bytes", guard.written,
	)
	if err == nil && !completed {
		err = errors.New("data return: archive size differs from the release")
	}
	return err
}

// openGrant counts what goes out and asks the database, every few
// megabytes, whether the grant is still neither revoked nor purged. It sits
// before the client's writer, so nothing more leaves once it refuses.
type openGrant struct {
	ctx       context.Context
	store     retrievalStore
	id        int64
	every     int64
	written   int64
	sinceLast int64
}

func (g *openGrant) Write(p []byte) (int, error) {
	if g.sinceLast >= g.every {
		open, err := g.store.stillOpen(g.ctx, g.id)
		if err != nil {
			return 0, err
		}
		if !open {
			return 0, organizationtenancy.ErrDataReturnDenied
		}
		g.sinceLast = 0
	}
	g.written += int64(len(p))
	g.sinceLast += int64(len(p))
	return len(p), nil
}
