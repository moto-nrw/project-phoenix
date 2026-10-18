package application

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountRetention tombstones the accounts no school needs any more (#4043)
// and removes the avatar files they uploaded, from the public directory the
// avatar upload writes to. The hourly token cleanup runs it in its
// administrative transaction; it needs no flow dependencies, so every
// composition serves it. The engine embeds it.
type AccountRetention struct {
	store      ports.AccountRetentionStore
	attach     func(context.Context) context.Context
	removeFile func(string) error
	logger     *slog.Logger
}

// NewAccountRetention binds the store; attach adds the composed unit of work
// to the cleanup's context, like the engine does for every other flow.
func NewAccountRetention(store ports.AccountRetentionStore, attach func(context.Context) context.Context, logger *slog.Logger) *AccountRetention {
	if store == nil || attach == nil {
		panic("identity access application: account retention requires its store and runtime")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountRetention{store: store, attach: attach, removeFile: os.Remove, logger: logger}
}

// EraseUnusedAccounts reports how many accounts it erased. A failed avatar
// removal rolls back the enclosing cleanup transaction, so the account and
// its file are retried together on the next hourly run. A missing file has
// already been removed and therefore succeeds idempotently.
func (r *AccountRetention) EraseUnusedAccounts(ctx context.Context) (int, error) {
	erased, _, err := r.store.EraseUnusedAccounts(r.attach(ctx))
	if err != nil {
		return 0, failed("erase unused accounts", err)
	}
	for _, account := range erased {
		for _, avatar := range account.AvatarFiles {
			path := uploadedAvatarPath(avatar)
			if path == "" {
				continue
			}
			if err := r.removeFile(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return 0, failed("delete avatar file", err)
			}
		}
	}
	if len(erased) > 0 {
		r.logger.InfoContext(ctx, "unused accounts erased",
			slog.Int("records_deleted", len(erased)))
	}
	return len(erased), nil
}
