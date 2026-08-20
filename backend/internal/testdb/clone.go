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
	"strconv"
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
// The comment also carries the run's heartbeat, in the same shape the
// template stamp uses: "phx-pkg:<label> touched:<unix>".
const pkgCommentPrefix = "phx-pkg:"

// heartbeatInterval is how often a live clone refreshes the timestamp in its
// comment, and runGracePeriod is how long after the last refresh a run still
// counts as alive. The gap between them is what a run needs to survive the
// window between its last binary exiting and its sweep starting — the sweep
// is a `go run`, so that window includes a compile.
const (
	heartbeatInterval = 20 * time.Second
	runGracePeriod    = 90 * time.Second
)

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

// PackageLabelOf extracts the package label from a clone's database comment.
func PackageLabelOf(comment string) string {
	if !strings.HasPrefix(comment, pkgCommentPrefix) {
		return ""
	}
	label := strings.TrimPrefix(comment, pkgCommentPrefix)
	if idx := strings.Index(label, touchedCommentKey); idx >= 0 {
		label = label[:idx]
	}
	return label
}

// cloneComment renders a clone's stamp: which package it belongs to, and when
// its run was last seen alive.
func cloneComment(label string, now time.Time) string {
	return pkgCommentPrefix + label + touchedCommentKey + strconv.FormatInt(now.Unix(), 10)
}

// heartbeatAt reads the trailing "touched:<unix>" out of a CLONE comment.
// Deliberately separate from the template's touchedAt: the two stamps share
// the key but not the prefix, and a template must never be mistaken for a
// clone or the other way round.
func heartbeatAt(comment string) (time.Time, bool) {
	if !strings.HasPrefix(comment, pkgCommentPrefix) {
		return time.Time{}, false
	}
	idx := strings.Index(comment, touchedCommentKey)
	if idx < 0 {
		return time.Time{}, false
	}
	unix, err := strconv.ParseInt(comment[idx+len(touchedCommentKey):], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(unix, 0), true
}

// runIDOf reads the run identifier out of a clone name
// (phx_test_pkg_<runID>_<sha1(workdir)[:12]>). Empty for anything else.
func runIDOf(name string) string {
	rest, ok := strings.CutPrefix(name, ClonePrefix)
	if !ok {
		return ""
	}
	runID, _, ok := strings.Cut(rest, "_")
	if !ok {
		return ""
	}
	return runID
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
// refreshes the heartbeat in the clone's comment — the two together are what
// tells a foreign GC that this clone's RUN is alive, including for the
// sibling clones whose binaries have already exited.
type CloneHandle struct {
	Name string
	DSN  string

	// label is the package stamp the heartbeat rewrites along with the
	// timestamp; the comment carries both.
	label string

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

// keepAlive pings the keeper connection (keeping the session non-idle),
// replaces it when it died, and refreshes the clone's heartbeat. Best-effort:
// if the clone itself is gone, tests are failing loudly anyway.
func (h *CloneHandle) keepAlive(stop <-chan struct{}) {
	ticker := time.NewTicker(heartbeatInterval)
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
			if h.keeperConn != nil {
				// COMMENT ON DATABASE writes a shared catalog, so the clone's
				// own connection can stamp it — no maintenance connection and
				// no lifecycle lock needed for a heartbeat.
				_, _ = h.keeperConn.ExecContext(ctx, `COMMENT ON DATABASE `+quoteIdentifier(h.Name)+
					` IS `+quoteLiteral(cloneComment(h.label, time.Now())))
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

	conn, unlock, err := acquireLifecycleLock(ctx, maint)
	if err != nil {
		return nil, fmt.Errorf("acquire test DB lifecycle lock: %w", err)
	}
	defer unlock()

	// Generation GC: clones of this run are spared even when their binary
	// already finished (the wrapper's sweep still wants to inspect and drop
	// them); everything else without a live connection belongs to a dead run.
	//
	// Two prefixes, not one: runID is the run this clone belongs to, and
	// RunID() is the run of the PROCESS asking. They differ exactly once — in
	// internal/testdb's own tests, which create clones under their own
	// throwaway run while the suite around them is still going. Sparing only
	// the first would let those tests collect every already-finished package
	// clone of the suite, and the leftover gate would then inspect a third of
	// the packages and call the rest clean.
	if _, err := gcLocked(ctx, conn, cfg.TemplateName(),
		ClonePrefix+SanitizeRunID(runID)+"_", ClonePrefix+RunID()+"_"); err != nil {
		return nil, err
	}

	if err := dropDatabase(ctx, conn, name); err != nil {
		return nil, fmt.Errorf("drop stale package test database %q: %w", name, err)
	}
	if _, err := conn.ExecContext(ctx,
		`CREATE DATABASE `+quoteIdentifier(name)+` TEMPLATE `+quoteIdentifier(cfg.TemplateName())); err != nil {
		return nil, fmt.Errorf("clone test database %q from %q: %w", name, cfg.TemplateName(), err)
	}
	// Stamp which package this clone belongs to, plus the run's first
	// heartbeat. The clone name is a hash of the working directory, so without
	// the stamp the gate can report a leftover but not say who left it behind.
	label := packageLabel(wd)
	if _, err := conn.ExecContext(ctx, `COMMENT ON DATABASE `+quoteIdentifier(name)+` IS `+
		quoteLiteral(cloneComment(label, time.Now()))); err != nil {
		return nil, fmt.Errorf("stamp package label on clone %q: %w", name, err)
	}

	handle := &CloneHandle{Name: name, DSN: cfg.DatabaseDSN(name), label: label}
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

// gcLocked drops the clones of DEAD runs. A run is alive while any of its
// clones has a live connection or a fresh heartbeat — not just the clone
// being looked at. That distinction is the whole point: a clone is pinned by
// a keeper connection only while ITS package binary runs, so a run of 90
// packages has, at any moment, 89 finished clones with nothing holding them.
// Judging each clone on its own connection made two concurrent suites collect
// each other's finished clones, which left the leftover gate inspecting a
// handful of packages and calling the rest clean (#2419).
//
// The heartbeat (CloneHandle.keepAlive, refreshed every heartbeatInterval)
// covers the tail: between the last binary exiting and the sweep starting,
// the run has no live connection at all, and without a grace period a
// concurrent run would collect it in that window.
//
// templateName is never dropped, even if the configured template happens to
// carry the clone prefix, and clones under sparePrefixes are kept regardless.
// Callers must hold the lifecycle lock.
func gcLocked(ctx context.Context, maint sqlExecutor, templateName string, sparePrefixes ...string) ([]string, error) {
	clones, err := listClones(ctx, maint)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	liveRuns := make(map[string]bool)
	for _, c := range clones {
		if c.connected {
			liveRuns[runIDOf(c.name)] = true
			continue
		}
		if beat, ok := heartbeatAt(c.comment); ok && now.Sub(beat) < runGracePeriod {
			liveRuns[runIDOf(c.name)] = true
		}
	}

	var dropped []string
	for _, c := range clones {
		if c.name == templateName || liveRuns[runIDOf(c.name)] || hasAnyPrefix(c.name, sparePrefixes) {
			continue
		}
		if err := dropDatabase(ctx, maint, c.name); err != nil {
			return dropped, fmt.Errorf("drop orphaned clone %q: %w", c.name, err)
		}
		dropped = append(dropped, c.name)
	}
	return dropped, nil
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
	comment   string
	connected bool
}

func listClones(ctx context.Context, maint sqlExecutor) ([]cloneState, error) {
	rows, err := maint.QueryContext(ctx, `
		SELECT d.datname,
		       COALESCE(sd.description, ''),
		       EXISTS (SELECT 1 FROM pg_stat_activity a WHERE a.datname = d.datname)
		FROM pg_database d
		LEFT JOIN pg_shdescription sd ON sd.objoid = d.oid AND sd.classoid = 'pg_database'::regclass
		WHERE LEFT(d.datname, char_length($1)) = $1
		ORDER BY d.datname`, ClonePrefix)
	if err != nil {
		return nil, fmt.Errorf("list test database clones: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var clones []cloneState
	for rows.Next() {
		var c cloneState
		if err := rows.Scan(&c.name, &c.comment, &c.connected); err != nil {
			return nil, fmt.Errorf("scan clone state: %w", err)
		}
		clones = append(clones, c)
	}
	return clones, rows.Err()
}
