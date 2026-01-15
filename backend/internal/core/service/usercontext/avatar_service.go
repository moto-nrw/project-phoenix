package usercontext

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// Avatar upload constants
const (
	MaxAvatarSize = 5 * 1024 * 1024 // 5MB
	// avatarSubdir is the subdirectory within storage for avatars
	avatarSubdir = "avatars"
)

// Allowed image types
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
}

// avatarStorage provides file storage for avatar operations.
// This is set via SetAvatarStorage during service initialization.
// If nil, avatar operations will return an error.
var avatarStorage port.FileStorage

// SetAvatarStorage configures the file storage backend for avatar operations.
// This should be called during application initialization.
func SetAvatarStorage(storage port.FileStorage) {
	avatarStorage = storage
}

// AvatarUploadInput represents the input for uploading an avatar
type AvatarUploadInput struct {
	File        io.ReadSeeker
	Filename    string
	ContentType string
}

// UploadAvatar handles the complete avatar upload flow
func (s *userContextService) UploadAvatar(ctx context.Context, input AvatarUploadInput) (map[string]interface{}, error) {
	if avatarStorage == nil {
		return nil, &UserContextError{Op: "upload avatar", Err: errors.New("avatar storage not configured")}
	}

	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "upload avatar", Err: err}
	}

	contentType, err := detectAndValidateContentType(input.File)
	if err != nil {
		return nil, &UserContextError{Op: "upload avatar", Err: err}
	}

	avatarURL, err := saveAvatarFile(ctx, input.File, input.Filename, contentType, user.ID)
	if err != nil {
		return nil, &UserContextError{Op: "upload avatar", Err: err}
	}

	profile, err := s.UpdateAvatar(ctx, avatarURL)
	if err != nil {
		// Attempt to clean up the saved file on error
		key := extractStorageKey(avatarURL)
		if key != "" {
			_ = avatarStorage.Delete(ctx, key)
		}
		return nil, err
	}

	return profile, nil
}

// DeleteAvatar removes the current user's avatar
func (s *userContextService) DeleteAvatar(ctx context.Context) (map[string]interface{}, error) {
	profile, err := s.GetCurrentProfile(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "delete avatar", Err: err}
	}

	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		return nil, &UserContextError{Op: "delete avatar", Err: errors.New("no avatar to delete")}
	}

	updatedProfile, err := s.UpdateAvatar(ctx, "")
	if err != nil {
		return nil, err
	}

	// Delete the file from storage
	if avatarStorage != nil {
		if key := extractStorageKey(avatarPath); key != "" {
			if err := avatarStorage.Delete(ctx, key); err != nil {
				logger.Logger.WithError(err).WithField("key", key).Warn("Failed to delete avatar file from storage")
			}
		}
	}

	return updatedProfile, nil
}

// ValidateAvatarAccess checks if the current user can access the requested avatar
func (s *userContextService) ValidateAvatarAccess(ctx context.Context, filename string) error {
	profile, err := s.GetCurrentProfile(ctx)
	if err != nil {
		return err
	}

	avatarPath, ok := profile["avatar"].(string)
	if !ok || avatarPath == "" {
		return errors.New("no avatar found")
	}

	if filepath.Base(avatarPath) != filename {
		return errors.New("access denied")
	}

	return nil
}

// GetAvatarFile validates and returns an avatar file for streaming.
func GetAvatarFile(ctx context.Context, filename string) (port.StoredFile, error) {
	if avatarStorage == nil {
		return port.StoredFile{}, errors.New("avatar storage not configured")
	}

	key := filepath.Join(avatarSubdir, filename)
	return avatarStorage.Open(ctx, key)
}

// detectAndValidateContentType reads file header and validates content type
func detectAndValidateContentType(file io.ReadSeeker) (string, error) {
	buffer := make([]byte, 512)
	if _, err := file.Read(buffer); err != nil {
		return "", errors.New("cannot read file")
	}

	contentType := http.DetectContentType(buffer)
	if !allowedImageTypes[contentType] {
		return "", errors.New("invalid file type. Only JPEG, PNG, and WebP images are allowed")
	}

	if _, err := file.Seek(0, 0); err != nil {
		return "", errors.New("failed to process file")
	}

	return contentType, nil
}

// saveAvatarFile saves the uploaded file using the storage backend and returns the public URL
func saveAvatarFile(ctx context.Context, file io.Reader, originalFilename, contentType string, userID int64) (string, error) {
	if avatarStorage == nil {
		return "", errors.New("avatar storage not configured")
	}

	fileExt := getFileExtension(originalFilename, contentType)
	randomStr, err := generateRandomString(8)
	if err != nil {
		return "", errors.New("failed to generate filename")
	}

	filename := fmt.Sprintf("%d_%s%s", userID, randomStr, fileExt)
	key := filepath.Join(avatarSubdir, filename)

	publicURL, err := avatarStorage.Save(ctx, key, file, contentType)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return publicURL, nil
}

// getFileExtension returns the file extension, inferring from content type if needed
func getFileExtension(filename, contentType string) string {
	ext := filepath.Ext(filename)
	if ext != "" {
		return ext
	}

	switch contentType {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

// generateRandomString generates a cryptographically secure random string of specified length
func generateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	for i := range b {
		b[i] = charset[b[i]%byte(len(charset))]
	}
	return string(b), nil
}

// extractStorageKey extracts the storage key from a public URL.
// For example, "/uploads/avatars/123_abc.jpg" -> "avatars/123_abc.jpg"
func extractStorageKey(publicURL string) string {
	// Locate the "/uploads/" marker in relative or absolute URLs.
	const marker = "/uploads/"
	idx := strings.Index(publicURL, marker)
	if idx == -1 {
		return ""
	}
	return strings.TrimPrefix(publicURL[idx:], marker)
}
