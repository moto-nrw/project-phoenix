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

// pkgCommentPrefix marks a clone's package label ("phx-pkg:services/active").
const pkgCommentPrefix = "phx-pkg:"

// keeperRefreshInterval is how often a live clone verifies its keeper
// connection and replaces a dead session.
const keeperRefreshInterval = 20 * time.Second

// packageLabel turns a package's working directory into the backend-relative
// path the leftover report names. Outside a recognizable checkout it falls
// back to the directory's base name: this is a label, and failing to name a
// clone must never fail the run that creates it.
func packageLabel(workdir string) string {
	if root, err := ProjectRoot(); err == nil {
		if rel, relErr := filepath.Rel(filepath.Join(root, "backend"), workdir); relErr == nil &&
			!strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.Base(workdir)
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
// called). A background loop pings it and reconnects if the session ever
// dies (idle-session timeout, pg_terminate_backend, server hiccup), and
// keeps the session visible to foreign GC while this package is alive.
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

// keepAlive replaces a dead keeper session. Best-effort: if the clone itself
// is gone, tests are failing loudly anyway.
func (h *CloneHandle) keepAlive(stop <-chan struct{}) {
	ticker := time.NewTicker(keeperRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			h.mu.Lock()
			// Open and verify the replacement before releasing the old keeper.
			// That guarantees pg_stat_activity contains a liveness session for
			// the clone throughout a normal reconnect.
			replacement, err := h.keeperDB.Conn(ctx)
			if err == nil {
				err = replacement.PingContext(ctx)
			}
			if err == nil {
				previous := h.keeperConn
				h.keeperConn = replacement
				if previous != nil {
					_ = previous.Close()
				}
			} else if replacement != nil {
				_ = replacement.Close()
			}
			h.mu.Unlock()
			cancel()
		}
	}
}

// CreateClone creates this run's clone of the template for the package in
// the current working directory. It first tries the generation GC (exclusive
// lock, skipped when someone else holds it), then creates the clone under the
// SHARED lock — so the ~93 package binaries of a run clone concurrently
// instead of queueing — and pins it with a keeper connection.
func CreateClone(ctx context.Context, cfg *Config, runID string) (*CloneHandle, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determine package working directory: %w", err)
	}
	name := CloneName(runID, wd)

	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()

	if err := collectDeadRuns(ctx, maint, cfg, runID); err != nil {
		return nil, err
	}

	// Shared, not exclusive: two cloners never write the same database (the
	// name carries run ID and package), and CREATE DATABASE ... TEMPLATE only
	// needs the template to sit still, which is exactly what the exclusive
	// holders — template rebuild and GC — are kept out for.
	conn, unlock, err := acquireLifecycleLockShared(ctx, maint)
	if err != nil {
		return nil, fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	if err := createCloneDatabase(ctx, conn, cfg, name, packageLabel(wd)); err != nil {
		return nil, err
	}
	return openCloneHandle(ctx, cfg, name)
}

func createCloneDatabase(ctx context.Context, conn sqlExecutor, cfg *Config, name, label string) error {
	if err := dropDatabase(ctx, conn, name); err != nil {
		return fmt.Errorf("drop stale package test database %q: %w", name, err)
	}
	if _, err := conn.ExecContext(ctx,
		`CREATE DATABASE `+quoteIdentifier(name)+` TEMPLATE `+quoteIdentifier(cfg.TemplateName())); err != nil {
		return fmt.Errorf("clone test database %q from %q: %w", name, cfg.TemplateName(), err)
	}
	// The name hashes the working directory; the label makes leftover reports
	// identify the package that owned the clone.
	if _, err := conn.ExecContext(ctx, `COMMENT ON DATABASE `+quoteIdentifier(name)+` IS `+
		quoteLiteral(pkgCommentPrefix+label)); err != nil {
		return fmt.Errorf("stamp package label on clone %q: %w", name, err)
	}
	return nil
}

func openCloneHandle(ctx context.Context, cfg *Config, name string) (*CloneHandle, error) {
	handle := &CloneHandle{Name: name, DSN: cfg.DatabaseDSN(name)}
	handle.keeperDB = openSQL(handle.DSN)
	handle.keeperDB.SetMaxOpenConns(2)
	handle.keeperDB.SetConnMaxLifetime(0)
	handle.keeperDB.SetConnMaxIdleTime(0)
	keeperConn, err := handle.keeperDB.Conn(ctx)
	if err != nil {
		_ = handle.keeperDB.Close()
		return nil, fmt.Errorf("pin package test database %q: %w", name, err)
	}
	if err := keeperConn.PingContext(ctx); err != nil {
		_ = keeperConn.Close()
		_ = handle.keeperDB.Close()
		return nil, fmt.Errorf("pin package test database %q: %w", name, err)
	}
	handle.keeperConn = keeperConn
	handle.keeperStop = make(chan struct{})
	go handle.keepAlive(handle.keeperStop)

	return handle, nil
}

// DropClone drops one clone by name. Deliberately WITHOUT the lifecycle
// lock: a test binary drops its own clone at exit, and 90 of those queueing
// behind the lock would serialize against the clone creation of every package
// still starting up. Nothing else can be doing anything with this clone —
// dropping is idempotent (IF EXISTS), and a concurrent GC pass that listed it
// a moment ago simply finds it gone.
func DropClone(ctx context.Context, cfg *Config, name string) error {
	maint := openSQL(cfg.MaintenanceDSN())
	defer func() { _ = maint.Close() }()
	return dropDatabase(ctx, maint, name)
}

// gcLocked drops every clone no package process can still use. Each package
// runs its leftover gate before releasing its keeper, so a finished sibling
// clone no longer needs run-level protection and can be collected at once.
//
// templateName is never dropped, even if the configured template happens to
// carry the clone prefix, and clones under sparePrefixes are kept regardless.
// Callers must hold the lifecycle lock.
func gcLocked(ctx context.Context, maint sqlExecutor, templateName string, sparePrefixes ...string) ([]string, error) {
	clones, err := listClones(ctx, maint)
	if err != nil {
		return nil, err
	}

	var dropped []string
	for _, c := range clones {
		if c.name == templateName || c.connected || hasAnyPrefix(c.name, sparePrefixes) {
			continue
		}
		if err := dropDatabase(ctx, maint, c.name); err != nil {
			return dropped, fmt.Errorf("drop orphaned clone %q: %w", c.name, err)
		}
		dropped = append(dropped, c.name)
	}
	return dropped, nil
}

// collectDeadRuns runs the generation GC if the exclusive lifecycle lock is
// free right now, and does nothing if it is not.
//
// Clones of this run are spared even when their binary already finished (the
// wrapper's sweep still wants to inspect and drop them); everything else
// without a live connection belongs to a dead run.
//
// Two spare prefixes, not one: runID is the run the clone being created
// belongs to, and RunID() is the run of the PROCESS asking. They differ
// exactly once — in internal/testdb's own tests, which create clones under
// their own throwaway run while the suite around them is still going. Sparing
// only the first would let those tests collect every already-finished package
// clone of the suite, and the leftover gate would then inspect a third of the
// packages and call the rest clean.
//
// Skipping is safe by design: the GC only removes clones of runs that are
// already dead, so a skipped pass costs disk until the next binary, the run's
// sweep, or the next run picks it up. Waiting for the lock instead would
// serialize every binary's start again, which is what moving the clone itself
// to the shared lock removed.
func collectDeadRuns(ctx context.Context, maint *sql.DB, cfg *Config, runID string) error {
	conn, unlock, ok, err := tryAcquireLifecycleLock(ctx, maint)
	if err != nil {
		return fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	if !ok {
		return nil
	}
	defer unlock()

	_, err = gcLocked(ctx, conn, cfg.TemplateName(),
		ClonePrefix+SanitizeRunID(runID)+"_", ClonePrefix+RunID()+"_")
	return err
}

func hasAnyPrefix(name string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if prefix != "" && strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// cloneState is what the GC needs to know about one clone database.
type cloneState struct {
	name      string
	connected bool
}

func listClones(ctx context.Context, maint sqlExecutor) ([]cloneState, error) {
	rows, err := maint.QueryContext(ctx, `
		SELECT d.datname,
		       EXISTS (SELECT 1 FROM pg_stat_activity a WHERE a.datname = d.datname)
		FROM pg_database d
		WHERE LEFT(d.datname, char_length($1)) = $1
		ORDER BY d.datname`, ClonePrefix)
	if err != nil {
		return nil, fmt.Errorf("list test database clones: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clones []cloneState
	for rows.Next() {
		var c cloneState
		if err := rows.Scan(&c.name, &c.connected); err != nil {
			return nil, fmt.Errorf("scan clone state: %w", err)
		}
		clones = append(clones, c)
	}
	return clones, rows.Err()
}
