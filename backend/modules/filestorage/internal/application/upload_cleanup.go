package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

type UploadCleanup struct {
	kind   string
	prefix string
	store  ports.CleanupStore
	tx     ports.Transaction
}

func NewUploadCleanup(store ports.CleanupStore, tx ports.Transaction, kind string) *UploadCleanup {
	if kind != "student-photos" && kind != "enrollment-legal-documents" {
		panic("unsupported upload cleanup namespace")
	}
	return &UploadCleanup{store: store, tx: tx, kind: kind, prefix: "/uploads/" + kind + "/"}
}

func (c *UploadCleanup) Queue(ctx context.Context, url string) error {
	if url == "" {
		return nil
	}
	// Invalid legacy URLs name no managed upload. Clearing such a row must not
	// become impossible, and cleanup must never reach another upload family.
	name, ok := strings.CutPrefix(url, c.prefix)
	if !ok || name == "" || strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil
	}
	if c.kind == "enrollment-legal-documents" && !strings.HasPrefix(name, fmt.Sprintf("%d_", c.tx.TenantID(ctx))) {
		return fmt.Errorf("legal document cleanup requires the owning tenant")
	}
	return c.tx.RunWrite(ctx, func(txCtx context.Context) error {
		_, err := c.store.Queue(txCtx, c.tx.TenantID(txCtx), name, time.Now())
		return err
	})
}

func (c *UploadCleanup) Sweep(ctx context.Context, remove func(context.Context, string) error) (int, error) {
	var removed int
	var failures []error
	err := c.tx.RunWrite(ctx, func(txCtx context.Context) error {
		rows, _, err := c.store.ListQueued(txCtx)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := remove(txCtx, c.prefix+row.FilenameStored); err != nil {
				failures = append(failures, fmt.Errorf("remove detached upload: %w", err))
				continue
			}
			if _, err := c.store.MarkComplete(txCtx, row.ID); err != nil {
				return err
			}
			removed++
		}
		// Filesystem errors leave their intents pending. Report them after the
		// successful completions commit, so one bad file does not undo the batch.
		return nil
	})
	return removed, errors.Join(append(failures, err)...)
}
