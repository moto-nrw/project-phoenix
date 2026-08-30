package migrations

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/argon2"
)

// hashBootstrapPassword preserves the Argon2id format used when the historical
// bootstrap-account migrations were written. PostgreSQL supplies the random
// 16-byte salt because uuid-ossp is installed before either migration runs.
func hashBootstrapPassword(ctx context.Context, db bun.IDB, password string) (string, error) {
	var salt []byte
	if err := db.NewRaw(`SELECT uuid_send(uuid_generate_v4())`).Scan(ctx, &salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	const (
		memory      = 64 * 1024
		iterations  = 3
		parallelism = 2
		keyLength   = 32
	)
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Migration represents a database migration with metadata
type Migration struct {
	Version     string   // Semantic version of the migration
	Description string   // Human-readable description
	DependsOn   []string // Versions this migration depends on
	Up          func(ctx context.Context, db *bun.DB) error
	Down        func(ctx context.Context, db *bun.DB) error
}
