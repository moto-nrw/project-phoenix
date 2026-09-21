package application

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Profile returns the caller's own profile: the account, the linked person
// when there is one, and the school-scoped profile fields.
func (c *CallerContext) Profile(ctx context.Context) (domain.CallerProfile, error) {
	account, err := c.Account(ctx)
	if err != nil {
		return domain.CallerProfile{}, &domain.CallerError{Op: "get current profile", Err: err}
	}
	profile := domain.CallerProfile{Account: account}
	if person, err := c.Person(ctx); err == nil {
		profile.Person = &person
	}
	if account.ID > 0 {
		fields, found, err := c.deps.Accounts.FindAccountProfile(ctx, account.ID)
		if err == nil && found {
			profile.Bio, profile.Settings = fields.Bio, fields.Settings
		}
	}
	return profile, nil
}

// UpdateProfile applies the caller's self-edits: names on the linked person
// (creating the person when the account has none), the account username and
// the school-scoped bio. The caller's memo entry is dropped before the
// trailing re-read, so the response shows committed state.
func (c *CallerContext) UpdateProfile(ctx context.Context, update domain.CallerProfileUpdate) (profile domain.CallerProfile, err error) {
	const op = "update current profile"
	err = c.deps.Write(ctx, func(ctx context.Context) error {
		account, err := c.Account(ctx)
		if err != nil {
			return &domain.CallerError{Op: op, Err: err}
		}
		person, personErr := c.Person(ctx)
		if err := c.updatePersonNames(ctx, account.ID, person, personErr, update); err != nil {
			return &domain.CallerError{Op: op, Err: err}
		}
		if update.Username != nil {
			if err := c.deps.Accounts.SetAccountUsername(ctx, account.ID, *update.Username); err != nil {
				return &domain.CallerError{Op: op, Err: err}
			}
		}
		if update.Bio != nil {
			if err := c.deps.Accounts.SetAccountBio(ctx, account.ID, *update.Bio); err != nil {
				return &domain.CallerError{Op: op, Err: err}
			}
		}
		// The writes above may have created a person for an account whose
		// person stage is memoized as "not linked", or changed memoized fields.
		c.invalidate(ctx)
		profile, err = c.Profile(ctx)
		return err
	})
	return profile, err
}

// updatePersonNames renames the linked person, or creates one when the
// account has none; creating requires both names. An empty name leaves the
// stored one.
func (c *CallerContext) updatePersonNames(ctx context.Context, accountID int64, person domain.CallerPerson, personErr error, update domain.CallerProfileUpdate) error {
	if update.FirstName == nil && update.LastName == nil {
		return nil
	}
	if personErr != nil {
		firstName, lastName := valueOf(update.FirstName), valueOf(update.LastName)
		if firstName == "" || lastName == "" {
			return errors.New("first name and last name are required to create profile")
		}
		return c.deps.People.CreatePerson(ctx, accountID, firstName, lastName)
	}
	firstName, lastName := nonEmpty(update.FirstName), nonEmpty(update.LastName)
	if firstName == nil && lastName == nil {
		return nil
	}
	return c.deps.People.RenamePerson(ctx, person.ID, firstName, lastName)
}

func valueOf(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nonEmpty(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

// UpdateAvatar stores the caller's avatar URL, an empty URL removing it, and
// deletes the file of the replaced upload. The memo entry is dropped because
// the memoized account still carries the old URL.
func (c *CallerContext) UpdateAvatar(ctx context.Context, avatarURL string) (profile domain.CallerProfile, err error) {
	const op = "update avatar"
	err = c.deps.Write(ctx, func(ctx context.Context) error {
		account, err := c.Account(ctx)
		if err != nil {
			return &domain.CallerError{Op: op, Err: err}
		}
		oldAvatarPath := uploadedAvatarPath(account.Avatar)
		if err := c.deps.Accounts.SetAccountAvatar(ctx, account.ID, avatarURL); err != nil {
			return &domain.CallerError{Op: op, Err: err}
		}
		if oldAvatarPath != "" {
			if err := c.deps.RemoveFile(oldAvatarPath); err != nil {
				c.logger().WarnContext(ctx, "failed to delete old avatar file",
					slog.String("path", oldAvatarPath),
					slog.String("error", err.Error()))
			}
		}
		c.invalidate(ctx)
		profile, err = c.Profile(ctx)
		return err
	})
	return profile, err
}

// uploadedAvatarPath returns the file behind an uploaded avatar URL, or ""
// for avatars that are not uploads.
func uploadedAvatarPath(avatar string) string {
	if avatar != "" && strings.HasPrefix(avatar, "/uploads/avatars/") {
		return filepath.Join("public", strings.TrimPrefix(avatar, "/"))
	}
	return ""
}
