package me_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// publicDir finds the public directory the upload storage resolves relative
// paths against: the first public/ or backend/public/ walking up from the
// working directory.
func publicDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	for current := wd; ; current = filepath.Dir(current) {
		for _, candidate := range []string{filepath.Join(current, "public"), filepath.Join(current, "backend", "public")} {
			if info, statErr := os.Stat(candidate); statErr == nil && info.IsDir() {
				return candidate
			}
		}
		if filepath.Dir(current) == current {
			return "public"
		}
	}
}

// storedFile is the file behind a stored upload URL such as
// /uploads/avatars/global/1_x.png.
func storedFile(t *testing.T, urlPath string) string {
	t.Helper()
	require.True(t, strings.HasPrefix(urlPath, "/uploads/"), "stored upload URL %q", urlPath)
	return filepath.Join(publicDir(t), strings.TrimPrefix(urlPath, "/"))
}
