package testdb

import (
	"context"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // clone names only need collision resistance for identifiers, not security
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// RunIDEnv carries the run identifier that stamps every clone of one test
// run. The wrapper scripts export it so all package binaries share one run;
// a naked `go test` gets a random per-process ID and relies on the next
// run's GC for cleanup (ADR 0004).
const RunIDEnv = "PHX_TEST_RUN_ID"

var (
	runIDOnce sync.Once
	runID     string

	validRunID = regexp.MustCompile(`^[a-z0-9]{1,16}$`)
)

// RunID returns this process's run identifier: PHX_TEST_RUN_ID if set (and
// sanitized), otherwise a random per-process ID.
func RunID() string {
	runIDOnce.Do(func() {
		runID = SanitizeRunID(os.Getenv(RunIDEnv))
	})
	return runID
}

// SanitizeRunID normalizes raw into a postgres-identifier-safe run ID.
// Empty input yields a fresh random ID; anything not matching
// ^[a-z0-9]{1,16}$ is hashed down to one.
func SanitizeRunID(raw string) string {
	if raw == "" {
		return randomRunID()
	}
	if validRunID.MatchString(raw) {
		return raw
	}
	sum := sha1.Sum([]byte(raw)) //nolint:gosec // identifier derivation, not security
	return hex.EncodeToString(sum[:])[:12]
}

func randomRunID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failing is effectively fatal elsewhere; fall back to
		// the PID so clone names stay unique per process.
		return fmt.Sprintf("pid%d", os.Getpid())
	}
	return hex.EncodeToString(b[:])
}

// CloneName derives the database name for this run's clone of the package
// rooted at workdir: phx_test_pkg_<runID>_<sha1(workdir)[:12]>.
func CloneName(runID, workdir string) string {
	normalized := filepath.ToSlash(workdir)
	sum := sha1.Sum([]byte(normalized)) //nolint:gosec // identifier derivation, not security
	return ClonePrefix + SanitizeRunID(runID) + "_" + hex.EncodeToString(sum[:])[:12]
}

// CloneHandle is a live package clone. The keeper connection pins the clone
// in pg_stat_activity so the cross-run GC never collects a clone whose test
// binary is still alive; it is released when the process exits (or Close is
// called). The keeper is the single liveness signal the generation model
// rests on, so a background loop pings it and reconnects if the session ever
// dies (idle-session timeout, pg_terminate_backend, server hiccup).
type CloneHandle struct {
	Name string
	DSN  string

	mu         sync.Mutex // guards keeperConn against the keepAlive loop
	keeperDB   *sql.DB
	keeperConn *sql.Conn
	keeperStop chan struct{}
}

// Close releases the keeper connection. Test binaries never call this — the
// keeper lives until process exit; it exists for the lifecycle's own tests.
func (h *CloneHandle) Close() error {
	if h.keeperStop != nil {
		close(h.keeperStop)
		h.keeperStop = nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.keeperConn != nil {
		_ = h.keeperConn.Close()
		h.keeperConn = nil
	}
	if h.keeperDB != nil {
		return h.keeperDB.Close()
	}
	return nil
}

// keepAlive pings the keeper connection every 30s (keeping the session
// non-idle) and replaces it when it died. Best-effort: if the clone itself is
// gone, tests are failing loudly anyway.
func (h *CloneHandle) keepAlive(stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			h.mu.Lock()
			if h.keeperConn == nil || h.keeperConn.PingContext(ctx) != nil {
				if h.keeperConn != nil {
					_ = h.keeperConn.Close()
				}
				h.keeperConn, _ = h.keeperDB.Conn(ctx)
			}
			h.mu.Unlock()
			cancel()
		}
	}
}

// CreateClone creates this run's clone of the template for the package in
// the current working directory. Under the lifecycle lock it first collects
// clones of dead runs (generation GC), then creates the clone and pins it
// with a keeper connection before releasing the lock.
func CreateClone(ctx context.Context, cfg *Config, runID string) (*CloneHandle, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determine package working directory: %w", err)
	}
	name := CloneName(runID, wd)

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()

	unlock, err := acquireLifecycleLock(ctx, maint)
	if err != nil {
		return nil, fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	// Generation GC: clones of this run are spared even when their binary
	// already finished (the wrapper's sweep still wants to inspect and drop
	// them); everything else without a live connection belongs to a dead run.
	if _, err := gcLocked(ctx, maint, ClonePrefix+SanitizeRunID(runID)+"_", cfg.TemplateName()); err != nil {
		return nil, err
	}

	if err := dropDatabase(ctx, maint, name); err != nil {
		return nil, fmt.Errorf("drop stale package test database %q: %w", name, err)
	}
	if _, err := maint.ExecContext(ctx,
		`CREATE DATABASE `+quoteIdentifier(name)+` TEMPLATE `+quoteIdentifier(cfg.TemplateName())); err != nil {
		return nil, fmt.Errorf("clone test database %q from %q: %w", name, cfg.TemplateName(), err)
	}

	handle := &CloneHandle{Name: name, DSN: cfg.DatabaseDSN(name)}
	handle.keeperDB = openSQL(handle.DSN)
	handle.keeperDB.SetMaxOpenConns(1)
	handle.keeperDB.SetConnMaxLifetime(0)
	handle.keeperDB.SetConnMaxIdleTime(0)
	conn, err := handle.keeperDB.Conn(ctx)
	if err != nil {
		_ = handle.keeperDB.Close()
		return nil, fmt.Errorf("pin package test database %q: %w", name, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		_ = handle.keeperDB.Close()
		return nil, fmt.Errorf("pin package test database %q: %w", name, err)
	}
	handle.keeperConn = conn
	handle.keeperStop = make(chan struct{})
	go handle.keepAlive(handle.keeperStop)

	return handle, nil
}

// gcLocked drops every clone-prefixed database that has no live connection
// and does not start with sparePrefix. templateName is never dropped, even
// if the configured template happens to carry the clone prefix. Callers must
// hold the lifecycle lock.
func gcLocked(ctx context.Context, maint *sql.DB, sparePrefix, templateName string) ([]string, error) {
	rows, err := maint.QueryContext(ctx, `
		SELECT d.datname
		FROM pg_database d
		WHERE d.datname LIKE $1
		  AND NOT EXISTS (SELECT 1 FROM pg_stat_activity a WHERE a.datname = d.datname)`,
		ClonePrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("list orphaned test database clones: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var orphans []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan orphaned clone name: %w", err)
		}
		if name == templateName {
			continue
		}
		if sparePrefix != "" && strings.HasPrefix(name, sparePrefix) {
			continue
		}
		orphans = append(orphans, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list orphaned test database clones: %w", err)
	}

	var dropped []string
	for _, name := range orphans {
		if err := dropDatabase(ctx, maint, name); err != nil {
			return dropped, fmt.Errorf("drop orphaned clone %q: %w", name, err)
		}
		dropped = append(dropped, name)
	}
	return dropped, nil
}
