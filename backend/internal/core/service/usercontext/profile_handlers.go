package usercontext

import (
	"context"
	"errors"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
)

// GetCurrentProfile retrieves the full profile for the current user including person, account, and profile data
func (s *userContextService) GetCurrentProfile(ctx context.Context) (map[string]interface{}, error) {
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "get current profile", Err: err}
	}

	person, _ := s.GetCurrentPerson(ctx)

	response := buildBaseResponse(account)
	addPersonOrAccountData(response, account, person)
	addProfileDataToResponse(ctx, s, response, account.ID)

	return response, nil
}

// buildBaseResponse builds the base response with account data
func buildBaseResponse(account *auth.Account) map[string]interface{} {
	return map[string]interface{}{
		"email":      account.Email,
		"username":   account.Username,
		"last_login": account.LastLogin,
	}
}

// addPersonOrAccountData adds person data if available, otherwise account fallback
func addPersonOrAccountData(response map[string]interface{}, account *auth.Account, person *users.Person) {
	if person != nil {
		addPersonData(response, person)
	} else {
		addAccountFallbackData(response, account)
	}
}

// addPersonData adds person data to response
func addPersonData(response map[string]interface{}, person *users.Person) {
	response["id"] = person.ID
	response["first_name"] = person.FirstName
	response["last_name"] = person.LastName
	response["created_at"] = person.CreatedAt
	response["updated_at"] = person.UpdatedAt

	if person.TagID != nil {
		response["rfid_card"] = *person.TagID
	}
}

// addAccountFallbackData adds account data as fallback when person doesn't exist
func addAccountFallbackData(response map[string]interface{}, account *auth.Account) {
	response["id"] = account.ID
	response["created_at"] = account.CreatedAt
	response["updated_at"] = account.UpdatedAt
	response["first_name"] = ""
	response["last_name"] = ""
}

// addProfileDataToResponse adds profile data if it exists
func addProfileDataToResponse(ctx context.Context, s *userContextService, response map[string]interface{}, accountID int64) {
	if accountID <= 0 {
		return
	}

	profile, err := s.profileRepo.FindByAccountID(ctx, accountID)
	if err != nil || profile == nil {
		return
	}

	addProfileFieldIfNotEmpty(response, "avatar", profile.Avatar)
	addProfileFieldIfNotEmpty(response, "bio", profile.Bio)
	addProfileFieldIfNotEmpty(response, "settings", profile.Settings)
}

// addProfileFieldIfNotEmpty adds a profile field to response if not empty
func addProfileFieldIfNotEmpty(response map[string]interface{}, key, value string) {
	if value != "" {
		response[key] = value
	}
}

// UpdateCurrentProfile updates the current user's profile with the provided data
func (s *userContextService) UpdateCurrentProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	person, personErr := s.GetCurrentPerson(ctx)

	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		if err := s.updatePersonDataInTx(txCtx, account, person, personErr, updates); err != nil {
			return err
		}

		if err := s.updateAccountUsernameInTx(txCtx, account, updates); err != nil {
			return err
		}

		return s.updateProfileBioInTx(txCtx, account.ID, updates)
	})

	if err != nil {
		return nil, &UserContextError{Op: "update current profile", Err: err}
	}

	return s.GetCurrentProfile(ctx)
}

// updatePersonDataInTx handles person creation or update within transaction
func (s *userContextService) updatePersonDataInTx(ctx context.Context, account *auth.Account, person *users.Person, personErr error, updates map[string]interface{}) error {
	firstName, hasFirstName := updates["first_name"].(string)
	lastName, hasLastName := updates["last_name"].(string)

	if !hasFirstName && !hasLastName {
		return nil
	}

	if personErr != nil || person == nil {
		return s.createPersonFromUpdates(ctx, account.ID, firstName, lastName)
	}

	return s.updateExistingPersonFields(ctx, person, firstName, hasFirstName, lastName, hasLastName)
}

// createPersonFromUpdates creates a new person record from update data
func (s *userContextService) createPersonFromUpdates(ctx context.Context, accountID int64, firstName, lastName string) error {
	if firstName == "" || lastName == "" {
		return errors.New("first name and last name are required to create profile")
	}

	person := &users.Person{
		AccountID: &accountID,
		FirstName: firstName,
		LastName:  lastName,
	}

	return s.personRepo.Create(ctx, person)
}

// updateExistingPersonFields updates existing person fields
func (s *userContextService) updateExistingPersonFields(ctx context.Context, person *users.Person, firstName string, hasFirstName bool, lastName string, hasLastName bool) error {
	needsUpdate := false

	if hasFirstName && firstName != "" {
		person.FirstName = firstName
		needsUpdate = true
	}

	if hasLastName && lastName != "" {
		person.LastName = lastName
		needsUpdate = true
	}

	if needsUpdate {
		return s.personRepo.Update(ctx, person)
	}

	return nil
}

// updateAccountUsernameInTx updates account username within transaction
func (s *userContextService) updateAccountUsernameInTx(ctx context.Context, account *auth.Account, updates map[string]interface{}) error {
	username, ok := updates["username"].(string)
	if !ok {
		return nil
	}

	if username == "" {
		account.Username = nil
	} else {
		account.Username = &username
	}

	return s.accountRepo.Update(ctx, account)
}

// updateProfileBioInTx updates or creates profile for bio update
func (s *userContextService) updateProfileBioInTx(ctx context.Context, accountID int64, updates map[string]interface{}) error {
	bio, hasBio := updates["bio"].(string)
	if !hasBio {
		return nil
	}

	profile, _ := s.profileRepo.FindByAccountID(ctx, accountID)
	if profile == nil {
		return s.createProfileWithBio(ctx, accountID, bio)
	}

	return s.updateExistingProfileBio(ctx, profile, bio)
}

// createProfileWithBio creates a new profile with bio
func (s *userContextService) createProfileWithBio(ctx context.Context, accountID int64, bio string) error {
	profile := &users.Profile{
		AccountID: accountID,
		Bio:       bio,
		Settings:  "{}",
	}
	return s.profileRepo.Create(ctx, profile)
}

// updateExistingProfileBio updates existing profile's bio
func (s *userContextService) updateExistingProfileBio(ctx context.Context, profile *users.Profile, bio string) error {
	profile.Bio = bio
	return s.profileRepo.Update(ctx, profile)
}

// UpdateAvatar updates the current user's avatar
func (s *userContextService) UpdateAvatar(ctx context.Context, avatarURL string) (map[string]interface{}, error) {
	account, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	var oldAvatarKey string

	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
		var updateErr error
		oldAvatarKey, updateErr = s.updateAvatarInTx(txCtx, account.ID, avatarURL)
		return updateErr
	})

	if err != nil {
		return nil, &UserContextError{Op: "update avatar", Err: err}
	}

	s.cleanupOldAvatar(ctx, oldAvatarKey)

	return s.GetCurrentProfile(ctx)
}

// updateAvatarInTx updates or creates profile with new avatar, returns old avatar key
func (s *userContextService) updateAvatarInTx(ctx context.Context, accountID int64, avatarURL string) (string, error) {
	profile, _ := s.profileRepo.FindByAccountID(ctx, accountID)
	if profile == nil {
		return "", s.createProfileWithAvatar(ctx, accountID, avatarURL)
	}

	oldKey := getOldAvatarKey(profile.Avatar)
	profile.Avatar = avatarURL

	if err := s.profileRepo.Update(ctx, profile); err != nil {
		return "", err
	}

	return oldKey, nil
}

// createProfileWithAvatar creates a new profile with avatar
func (s *userContextService) createProfileWithAvatar(ctx context.Context, accountID int64, avatarURL string) error {
	profile := &users.Profile{
		AccountID: accountID,
		Avatar:    avatarURL,
		Settings:  "{}",
	}
	return s.profileRepo.Create(ctx, profile)
}

// getOldAvatarKey returns the storage key for the old avatar if it needs cleanup.
func getOldAvatarKey(currentAvatar string) string {
	return extractStorageKey(currentAvatar)
}

// cleanupOldAvatar deletes old avatar from storage if key is provided.
func (s *userContextService) cleanupOldAvatar(ctx context.Context, oldAvatarKey string) {
	if oldAvatarKey == "" || s.avatarStorage == nil {
		return
	}

	if err := s.avatarStorage.Delete(ctx, oldAvatarKey); err != nil {
		logger.Logger.WithError(err).WithField("key", oldAvatarKey).Warn("Failed to delete old avatar file")
	}
}
