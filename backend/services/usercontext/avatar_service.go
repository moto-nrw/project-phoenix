package usercontext

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Avatar upload constants
const (
	MaxAvatarSize = 5 * 1024 * 1024 // 5MB
	avatarDir     = "public/uploads/avatars"
)

// Allowed image types
var allowedImageTypes = map[string]bool{
	"image/jpeg": true,
	"image/jpg":  true,
	"image/png":  true,
	"image/webp": true,
}

// AvatarUploadInput represents the input for uploading an avatar
type AvatarUploadInput struct {
	File        io.ReadSeeker
	Filename    string
	ContentType string
}

// UploadAvatar handles the complete avatar upload flow
func (s *userContextService) UploadAvatar(ctx context.Context, input AvatarUploadInput) (map[string]interface{}, error) {
	user, err := s.GetCurrentUser(ctx)
	if err != nil {
		return nil, &UserContextError{Op: "upload avatar", Err: err}
	}

	contentType, err := detectAndValidateContentType(input.File)
	if err != nil {
		return nil, &UserContextError{Op: "upload avatar", Err: err}
	}

	filePath, err := saveAvatarFile(input.File, input.Filename, contentType, user.ID)
	if err != nil {
		return nil, &UserContextError{Op: "upload avatar", Err: err}
	}

	avatarURL := fmt.Sprintf("/uploads/avatars/%s", filepath.Base(filePath))
	profile, err := s.UpdateAvatar(ctx, avatarURL)
	if err != nil {
		removeFile(filePath)
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

	if strings.HasPrefix(avatarPath, "/uploads/avatars/") {
		filePath := filepath.Join("public", avatarPath)
		if err := os.Remove(filePath); err != nil {
			log.Printf("Failed to delete avatar file: %v", err)
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

// GetAvatarFilePath validates and returns the full path to an avatar file
func GetAvatarFilePath(filename string) (string, error) {
	filePath := filepath.Join(avatarDir, filename)

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", errors.New("failed to process path")
	}

	absAvatarDir, err := filepath.Abs(avatarDir)
	if err != nil {
		return "", errors.New("failed to process avatar directory")
	}

	if !strings.HasPrefix(absPath, absAvatarDir) {
		return "", errors.New("invalid path")
	}

	return filePath, nil
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

// saveAvatarFile saves the uploaded file and returns the file path
func saveAvatarFile(file io.Reader, originalFilename, contentType string, userID int64) (string, error) {
	fileExt := getFileExtension(originalFilename, contentType)
	randomStr, err := generateRandomString(8)
	if err != nil {
		return "", errors.New("failed to generate filename")
	}

	filename := fmt.Sprintf("%d_%s%s", userID, randomStr, fileExt)
	filePath := filepath.Join(avatarDir, filename)

	if os.MkdirAll(avatarDir, 0755) != nil {
		return "", errors.New("failed to create upload directory")
	}

	dst, err := os.Create(filePath)
	if err != nil {
		return "", errors.New("failed to save file")
	}
	defer func() {
		if err := dst.Close(); err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}()

	if _, err := io.Copy(dst, file); err != nil {
		return "", errors.New("failed to save file")
	}

	return filePath, nil
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

// removeFile attempts to remove a file, logging any error
func removeFile(path string) {
	if err := os.Remove(path); err != nil {
		log.Printf("Error removing file: %v", err)
	}
}
