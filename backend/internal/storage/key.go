package storage

import (
	"path"
	"strconv"
	"strings"
)

// Key builds a storage key from segments, rejecting anything that could
// escape the backend root. It is the only supported way to construct a key:
// hand-built strings bypass the traversal check that makes the Backend
// contract safe.
func Key(segments ...string) (string, error) {
	if len(segments) == 0 {
		return "", ErrInvalidKey
	}
	clean := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", ErrInvalidKey
		}
		if strings.ContainsAny(segment, `/\`) {
			return "", ErrInvalidKey
		}
		if strings.Contains(segment, "..") {
			return "", ErrInvalidKey
		}
		clean = append(clean, segment)
	}
	key := path.Join(clean...)
	if key == "" || strings.HasPrefix(key, "/") || strings.Contains(key, "..") {
		return "", ErrInvalidKey
	}
	return key, nil
}

// TenantKey builds the canonical per-tenant key for a document-style upload:
// {kind}/{tenantID}/{name}. Tenant isolation on disk mirrors the row-level
// isolation in the database, so a stray filename can never resolve into
// another school's directory.
func TenantKey(kind string, tenantID int64, name string) (string, error) {
	if tenantID <= 0 {
		return "", ErrInvalidKey
	}
	return Key(kind, strconv.FormatInt(tenantID, 10), name)
}
