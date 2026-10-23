package application

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// AccountRetention anonymizes unused global accounts and drains the durable
// avatar deletion intents written by the same database transaction.
type AccountRetention struct {
	store      ports.AccountRetentionStore
	attach     func(context.Context) context.Context
	runAdmin   func(context.Context, func(context.Context) error) error
	removeFile func(string) error
	logger     *slog.Logger
}

func NewAccountRetention(store ports.AccountRetentionStore, attach func(context.Context) context.Context, runAdmin func(context.Context, func(context.Context) error) error, removeFile func(string) error, logger *slog.Logger) *AccountRetention {
	if store == nil || attach == nil || runAdmin == nil {
		panic("identity access application: account retention requires its store and runtime")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if removeFile == nil {
		removeFile = removeErasedAvatarFile
	}
	return &AccountRetention{store: store, attach: attach, runAdmin: runAdmin, removeFile: removeFile, logger: logger}
}

// EraseUnusedAccounts joins the hourly cleanup's administrative transaction.
// No file is touched while anonymization can still roll back.
func (r *AccountRetention) EraseUnusedAccounts(ctx context.Context) (int, error) {
	erased, _, err := r.store.EraseUnusedAccounts(r.attach(ctx))
	if err != nil {
		return 0, fmt.Errorf("identity access: erase unused accounts: %w", failed("erase unused accounts", err))
	}
	if len(erased) > 0 {
		r.logger.InfoContext(ctx, "unused accounts erased", slog.Int("records_deleted", len(erased)))
	}
	return len(erased), nil
}

// CleanupErasedAccountAvatars owns its administrative transaction. File failures
// are returned after commit so successful acknowledgements and retry ordering
// survive a partial failure. A crash before acknowledgement is safe: a missing
// file counts as success on the next attempt.
func (r *AccountRetention) CleanupErasedAccountAvatars(ctx context.Context) (int, error) {
	removed := 0
	var fileErrors []error
	// Strengthen the existing administrative boundary, never start a second one.
	ctx = context.WithValue(ctx, standaloneAdminScopeKey{}, true)
	err := r.runAdmin(ctx, func(txCtx context.Context) error {
		pending, err := r.store.PendingAvatarCleanups(txCtx)
		if err != nil {
			return fmt.Errorf("list account avatar cleanups: %w", err)
		}
		for _, entry := range pending {
			err := r.removeFile(entry.Avatar)
			success := err == nil || errors.Is(err, fs.ErrNotExist)
			if !success {
				// Filesystem errors may contain identifying paths. Return only
				// the queue ID, never the path or the underlying error text.
				fileErrors = append(fileErrors, fmt.Errorf("account avatar cleanup %d: file removal failed", entry.ID))
			}
			if err := r.store.FinishAvatarCleanup(txCtx, entry.ID, success); err != nil {
				return fmt.Errorf("finish account avatar cleanup: %w", err)
			}
			if success {
				removed++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return removed, errors.Join(fileErrors...)
}

// The rooted filesystem accepts both global/ and legacy <school>/ paths but
// cannot follow traversal or symlinks outside the avatar directory.
func removeErasedAvatarFile(avatar string) error {
	return removeErasedAvatarFileFrom("public/uploads/avatars", avatar)
}

func removeErasedAvatarFileFrom(directory, avatar string) (err error) {
	const prefix = "/uploads/avatars/"
	relative, ok := strings.CutPrefix(avatar, prefix)
	if !ok || relative == "." || !fs.ValidPath(relative) {
		return errors.New("invalid avatar path")
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		// An unavailable mount is not proof that the file is absent. Do not
		// expose ErrNotExist here: the caller would acknowledge every intent.
		return errors.New("avatar storage root unavailable")
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	info, err := root.Lstat(relative)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("avatar path is not a regular file")
	}
	return root.Remove(relative)
}
