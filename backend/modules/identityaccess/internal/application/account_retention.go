package application

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountRetention tombstones the accounts no school needs any more (#4043)
// and removes the avatar files they uploaded, from the public directory the
// avatar upload writes to. The hourly token cleanup runs it in its
// administrative transaction; it needs no flow dependencies, so every
// composition serves it. The engine embeds it.
type AccountRetention struct {
	store       ports.AccountRetentionStore
	attach      func(context.Context) context.Context
	afterCommit func(context.Context, func())
	removeFile  func(string) error
	logger      *slog.Logger
}

// NewAccountRetention binds the store; attach adds the composed unit of work
// to the cleanup's context, like the engine does for every other flow.
func NewAccountRetention(store ports.AccountRetentionStore, attach func(context.Context) context.Context, afterCommit func(context.Context, func()), logger *slog.Logger) *AccountRetention {
	if store == nil || attach == nil || afterCommit == nil {
		panic("identity access application: account retention requires its store and runtime")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AccountRetention{store: store, attach: attach, afterCommit: afterCommit, removeFile: os.Remove, logger: logger}
}

// EraseUnusedAccounts reports how many accounts it erased. Avatar removal runs
// after the enclosing cleanup transaction commits, so a later transaction
// failure cannot leave a still-valid account referring to a removed file.
func (r *AccountRetention) EraseUnusedAccounts(ctx context.Context) (int, error) {
	txCtx := r.attach(ctx)
	erased, _, err := r.store.EraseUnusedAccounts(txCtx)
	if err != nil {
		return 0, failed("erase unused accounts", err)
	}
	r.afterCommit(txCtx, func() { r.removeAvatars(ctx, erased) })
	if len(erased) > 0 {
		r.logger.InfoContext(ctx, "unused accounts erased",
			slog.Int("records_deleted", len(erased)))
	}
	return len(erased), nil
}

func (r *AccountRetention) removeAvatars(ctx context.Context, erased []domain.ErasedAccount) {
	for _, account := range erased {
		for _, avatar := range account.AvatarFiles {
			path := uploadedAvatarPath(avatar)
			if path == "" {
				continue
			}
			if err := r.removeFile(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
				r.logger.WarnContext(ctx, "failed to delete erased account avatar file",
					slog.Int64("account_id", account.ID),
					slog.Any("error", err))
			}
		}
	}
}
