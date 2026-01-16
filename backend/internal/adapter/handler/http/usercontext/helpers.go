package usercontext

import (
	"errors"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
)

// validateAvatarPath validates the filename to prevent path traversal.
func validateAvatarPath(filename string) (string, render.Renderer) {
	if filename == "" {
		return "", common.ErrorInvalidRequest(errors.New("filename required"))
	}

	if strings.Contains(filename, "..") || strings.ContainsAny(filename, "/\\") {
		return "", common.ErrorForbidden(errors.New("invalid path"))
	}

	return filename, nil
}
