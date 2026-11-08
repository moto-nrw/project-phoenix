//go:build darwin || linux

package testdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// corruptClusterMarker is PostgreSQL's startup PANIC when pg_control names a
// checkpoint that the WAL no longer contains. Test servers run with fsync=off,
// so writes lost by the Docker engine (VM killed, host disk full) leave this
// state behind even though PostgreSQL itself shut down cleanly (#6230). Such a
// cluster can never start again, so no session can still be using it.
const corruptClusterMarker = "could not locate a valid checkpoint record"

const testDataDirectory = "/var/lib/postgresql/data"

// rebuildCorruptServer replaces the managed test server behind cfg when its
// last start failed with corruptClusterMarker: it removes exactly that
// container and its data volume and starts a fresh one. It reports whether it
// rebuilt. Unmanaged, running, cleanly stopped or otherwise failed servers are
// left untouched. The rebuild lock is held through start, so a concurrent test
// process sees the replacement container instead of rebuilding it again.
func rebuildCorruptServer(ctx context.Context, cfg *Config, start func(context.Context) error) (bool, error) {
	if os.Getenv("CI") != "" || !isLoopbackHost(cfg.templateURL.Hostname()) {
		return false, nil
	}
	server, err := localIdleServer(ctx, composeProjectFor(cfg))
	if err != nil || server == nil {
		return false, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return false, err
	}
	dir := filepath.Join(cache, "moto-testdb", "v1", identifierHash(server.Host+"\n"+server.Project))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return false, err
	}
	lock, err := lockFile(ctx, filepath.Join(dir, "rebuild.lock"), syscall.LOCK_EX)
	if err != nil {
		return false, err
	}
	defer func() { _ = lock.Close() }()

	rebuilt, err := server.removeCorrupt(ctx)
	if err != nil || !rebuilt {
		return false, err
	}
	if err := start(ctx); err != nil {
		return true, fmt.Errorf("start rebuilt test server: %w", err)
	}
	return true, nil
}

func (s idleServer) removeCorrupt(ctx context.Context) (bool, error) {
	c, err := s.candidate(ctx)
	if err != nil || c == nil || c.State.Running || c.State.ExitCode == 0 || c.State.StartedAt == "" {
		// Unmanaged or unavailable servers are not ours to rebuild; the
		// caller keeps reporting the original readiness failure.
		return false, nil
	}
	// Only the most recent start counts: an older, repaired failure must not
	// turn a later unrelated exit into a data loss.
	out, err := s.command(ctx, "logs", "--since", c.State.StartedAt, c.ID)
	if err != nil {
		return false, fmt.Errorf("read test container logs: %w", err)
	}
	if !strings.Contains(string(out), corruptClusterMarker) {
		return false, nil
	}
	volume := ""
	for _, mount := range c.Mounts {
		if mount.Type == "volume" && mount.Destination == testDataDirectory {
			volume = mount.Name
		}
	}
	if volume != s.Project+"_postgres_test" {
		return false, fmt.Errorf("corrupt test server %s has unexpected data volume %q; rebuild it manually (docs/agents/operations.md#test-database)", s.Project, volume)
	}
	fmt.Fprintf(os.Stderr, "testdb: %s cannot start (%s); removing its container and volume %s and starting a fresh server\n", s.Project, corruptClusterMarker, volume)
	if _, err := s.command(ctx, "rm", "--force", c.ID); err != nil {
		return false, fmt.Errorf("remove corrupt test container: %w", err)
	}
	if _, err := s.command(ctx, "volume", "rm", volume); err != nil {
		return false, fmt.Errorf("remove corrupt test volume: %w", err)
	}
	return true, nil
}
