//go:build darwin || linux

package testdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func idleFixture(t *testing.T) (string, idleServer) {
	t.Helper()
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	script := `#!/bin/sh
set -eu
cd "$(dirname "$0")"
[ "$1" = --host ] && [ "$2" = unix:///fixture.sock ] || exit 91
shift 2
case "$1" in
 ps) printf 'fixture\n';;
 inspect) cat container.json;;
 exec) if [ -f query-fails ]; then exit 1; fi; cat clients;;
 stop) printf '%s\n' "$*" > stopped; while [ -f block-stop ]; do sleep 0.01; done;;
 *) exit 92;;
esac
`
	require.NoError(t, os.WriteFile(docker, []byte(script), 0700))
	server := idleServer{Host: "unix:///fixture.sock", Project: composeProjectPrefix + "-6543", Docker: docker}
	c := idleContainer{ID: "fixture"}
	c.Config.Labels = map[string]string{idleLabel: "1", "com.docker.compose.project": server.Project, "com.docker.compose.service": "postgres-test"}
	c.State.Running = true
	require.NoError(t, json.Unmarshal([]byte(`{"5432/tcp":[{"HostPort":"6543"}]}`), &c.HostConfig.PortBindings))
	data, err := json.Marshal([]idleContainer{c})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "container.json"), data, 0600))
	data, err = json.Marshal(server)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "server.json"), data, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "clients"), []byte("0\n"), 0600))
	return dir, server
}

func runFixtureWatcher(t *testing.T, dir string) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	singleton, err := tryLockFile(filepath.Join(dir, "watcher.lock"), syscall.LOCK_EX)
	require.NoError(t, err)
	require.NotNil(t, singleton)
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer func() { _ = singleton.Close() }()
		done <- WatchIdle(ctx, dir, singleton, 100*time.Millisecond, 10*time.Millisecond)
	}()
	t.Cleanup(func() { cancel(); <-finished })
	return done
}

func TestIdleWatcherProtectsParallelUsersAndStopsAfterLastExit(t *testing.T) {
	t.Parallel()
	dir, _ := idleFixture(t)
	first, err := lockFile(t.Context(), filepath.Join(dir, "users.lock"), syscall.LOCK_SH)
	require.NoError(t, err)
	second, err := lockFile(t.Context(), filepath.Join(dir, "users.lock"), syscall.LOCK_SH)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Close(); _ = second.Close() })
	done := runFixtureWatcher(t, dir)
	require.NoError(t, first.Close())
	require.Never(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "stopped"))
		return err == nil
	}, time.Second, 10*time.Millisecond, "a live sibling must block cleanup even with no SQL connections")
	require.NoError(t, second.Close())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("last user exited but container was not stopped")
	}
	stopped, err := os.ReadFile(filepath.Join(dir, "stopped"))
	require.NoError(t, err)
	require.Equal(t, "stop --timeout -1 fixture\n", string(stopped))
}

func TestIdleLeaseProcess(t *testing.T) {
	t.Parallel()
	path := os.Getenv("PHX_IDLE_TEST_LEASE")
	if path == "" {
		return
	}
	lease, err := lockFile(t.Context(), path, syscall.LOCK_SH)
	require.NoError(t, err)
	defer func() { _ = lease.Close() }()
	require.NoError(t, os.WriteFile(path+".ready", nil, 0600))
	// Parent deliberately kills this process; no deferred cleanup can run.
	time.Sleep(time.Minute)
}

func TestIdleWatcherRecoversAfterKilledOwner(t *testing.T) {
	t.Parallel()
	dir, _ := idleFixture(t)
	path := filepath.Join(dir, "users.lock")
	process, err := os.StartProcess(os.Args[0], []string{os.Args[0], "-test.run=^TestIdleLeaseProcess$"}, &os.ProcAttr{Env: append(os.Environ(), "PHX_IDLE_TEST_LEASE="+path), Files: []*os.File{os.Stdin, os.Stdout, os.Stderr}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = process.Kill() })
	require.Eventually(t, func() bool { _, err := os.Stat(path + ".ready"); return err == nil }, 5*time.Second, 10*time.Millisecond)
	done := runFixtureWatcher(t, dir)
	time.Sleep(150 * time.Millisecond)
	_, err = os.Stat(filepath.Join(dir, "stopped"))
	require.ErrorIs(t, err, os.ErrNotExist)
	require.NoError(t, process.Kill())
	state, err := process.Wait()
	require.NoError(t, err)
	require.False(t, state.Success())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("dead owner's lease was not released")
	}
}

func TestIdleStopSerializesWithNewStarter(t *testing.T) {
	t.Parallel()
	dir, _ := idleFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "block-stop"), nil, 0600))
	done := runFixtureWatcher(t, dir)
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(dir, "stopped")); return err == nil }, 5*time.Second, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	_, err := lockFile(ctx, filepath.Join(dir, "users.lock"), syscall.LOCK_SH)
	require.ErrorIs(t, err, context.DeadlineExceeded, "starter must wait until stop completes")
	require.NoError(t, os.Remove(filepath.Join(dir, "block-stop")))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not finish")
	}
	lease, err := lockFile(t.Context(), filepath.Join(dir, "users.lock"), syscall.LOCK_SH)
	require.NoError(t, err)
	defer func() { _ = lease.Close() }()
	watcher, err := tryLockFile(filepath.Join(dir, "watcher.lock"), syscall.LOCK_EX)
	require.NoError(t, err)
	require.NotNil(t, watcher, "starter must be able to replace the exiting watcher")
	require.NoError(t, watcher.Close())
}

func TestIdleCleanupRefusesUnmanagedBusyOrUncertainContainers(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"unlabelled", "development", "wrong-port", "remote", "clients", "query-fails", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			dir, server := idleFixture(t)
			var containers []idleContainer
			data, err := os.ReadFile(filepath.Join(dir, "container.json"))
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &containers))
			switch mode {
			case "unlabelled":
				delete(containers[0].Config.Labels, idleLabel)
			case "development":
				containers[0].Config.Labels["com.docker.compose.service"] = "postgres"
			case "wrong-port":
				containers[0].HostConfig.PortBindings["5432/tcp"][0].HostPort = "5432"
			case "remote":
				server.Host = "tcp://remote:2375"
			case "clients":
				require.NoError(t, os.WriteFile(filepath.Join(dir, "clients"), []byte("1\n"), 0600))
			case "query-fails":
				require.NoError(t, os.WriteFile(filepath.Join(dir, "query-fails"), nil, 0600))
			}
			data, err = json.Marshal(containers)
			require.NoError(t, err)
			if mode == "malformed" {
				data = []byte("not-json")
			}
			require.NoError(t, os.WriteFile(filepath.Join(dir, "container.json"), data, 0600))
			done, err := server.stopIdle(t.Context())
			require.False(t, done)
			if mode != "clients" {
				require.Error(t, err)
			}
			_, err = os.Stat(filepath.Join(dir, "stopped"))
			require.ErrorIs(t, err, os.ErrNotExist, fmt.Sprintf("must not stop %s", mode))
		})
	}
}

func TestIdleWatcherExitsWithoutAdoptingLegacyServer(t *testing.T) {
	t.Parallel()
	dir, _ := idleFixture(t)
	data, err := os.ReadFile(filepath.Join(dir, "container.json"))
	require.NoError(t, err)
	var containers []idleContainer
	require.NoError(t, json.Unmarshal(data, &containers))
	delete(containers[0].Config.Labels, idleLabel)
	data, err = json.Marshal(containers)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "container.json"), data, 0600))
	done := runFixtureWatcher(t, dir)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("legacy server left a watcher running")
	}
	_, err = os.Stat(filepath.Join(dir, "stopped"))
	require.ErrorIs(t, err, os.ErrNotExist)
}
