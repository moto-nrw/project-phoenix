//go:build darwin || linux

package testdb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func idleFixture(t *testing.T) (string, idleServer) {
	t.Helper()
	dir := t.TempDir()
	server := idleServer{Host: "unix:///fixture.sock", Project: composeProjectPrefix + "-6543", Docker: "/fixture/docker", run: idleFixtureCommand(dir)}
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

// Keep lock/timer tests independent of OS process-launch latency. The real
// command path is exercised by the Docker smoke check and discovery child.
func idleFixtureCommand(dir string) func(context.Context, ...string) ([]byte, error) {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		if len(args) < 3 || args[0] != "--host" || args[1] != "unix:///fixture.sock" {
			return nil, fmt.Errorf("Docker endpoint was not pinned")
		}
		args = args[2:]
		switch args[0] {
		case "ps":
			return []byte("fixture\n"), nil
		case "inspect":
			return os.ReadFile(filepath.Join(dir, "container.json"))
		case "exec":
			if _, err := os.Stat(filepath.Join(dir, "query-fails")); err == nil {
				return nil, fmt.Errorf("fixture query failed")
			}
			return os.ReadFile(filepath.Join(dir, "clients"))
		case "stop":
			if err := os.WriteFile(filepath.Join(dir, "stopped"), []byte(strings.Join(args, " ")+"\n"), 0600); err != nil {
				return nil, err
			}
			for {
				if _, err := os.Stat(filepath.Join(dir, "block-stop")); os.IsNotExist(err) {
					return nil, nil
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(10 * time.Millisecond):
				}
			}
		default:
			return nil, fmt.Errorf("unexpected Docker command: %s", args[0])
		}
	}
}

func runFixtureWatcher(t *testing.T, dir string) <-chan error {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	singleton, err := tryLockFile(filepath.Join(dir, "watcher.lock"), syscall.LOCK_EX)
	require.NoError(t, err)
	require.NotNil(t, singleton)
	data, err := os.ReadFile(filepath.Join(dir, "server.json"))
	require.NoError(t, err)
	var server idleServer
	require.NoError(t, json.Unmarshal(data, &server))
	server.run = idleFixtureCommand(dir)
	done := make(chan error, 1)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer func() { _ = singleton.Close() }()
		done <- watchIdle(ctx, dir, singleton, 100*time.Millisecond, 10*time.Millisecond, server)
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

// The child runs the public entry point with a broken Docker context, without
// changing the environment of parallel tests in the parent process.
func TestIdleDiscoveryFailureChild(t *testing.T) {
	t.Parallel()
	dsn := os.Getenv("PHX_IDLE_DISCOVERY_TEST_DSN")
	if dsn == "" {
		return
	}
	cfg, err := NewConfig(dsn)
	require.NoError(t, err)
	require.NoError(t, pingServer(t.Context(), cfg), "the database must already be reachable")
	require.NoError(t, EnsureServer(t.Context(), cfg), "optional Docker discovery must not block an already-running database")
}

func TestIdleDiscoveryFailureAllowsReachableDatabase(t *testing.T) {
	t.Parallel()
	cfg := integrationConfig(t)
	dir := t.TempDir()
	docker := filepath.Join(dir, "docker")
	require.NoError(t, os.WriteFile(docker, []byte("#!/bin/sh\necho 'test Docker context unavailable' >&2\nexit 73\n"), 0700))
	output, err := os.Create(filepath.Join(dir, "child.log"))
	require.NoError(t, err)
	defer func() { _ = output.Close() }()
	env := os.Environ()
	for key, value := range map[string]string{
		"PATH": dir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME": dir, "XDG_CACHE_HOME": dir, "CI": "",
		"DOCKER_CONTEXT": "unavailable-test-context", "DOCKER_HOST": "",
		"PHX_IDLE_DISCOVERY_TEST_DSN": cfg.MaintenanceDSN(),
	} {
		env = replaceCommandEnvironment(env, key, value)
	}
	process, err := os.StartProcess(os.Args[0], []string{os.Args[0], "-test.run=^TestIdleDiscoveryFailureChild$"}, &os.ProcAttr{Env: env, Files: []*os.File{os.Stdin, output, output}})
	require.NoError(t, err)
	state, err := process.Wait()
	require.NoError(t, err)
	log, err := os.ReadFile(output.Name())
	require.NoError(t, err)
	require.True(t, state.Success(), "%s", log)
}

func TestIdleDiscoveryFallbackKeepsRegisteredServerProtected(t *testing.T) {
	t.Parallel()
	dir, server := idleFixture(t)
	cache := t.TempDir()
	registration := filepath.Join(cache, "moto-testdb", "v1", "registered")
	require.NoError(t, os.MkdirAll(filepath.Dir(registration), 0700))
	require.NoError(t, os.Symlink(dir, registration))
	held := make(map[string]*os.File)
	t.Cleanup(func() {
		for _, lease := range held {
			_ = lease.Close()
		}
	})
	require.NoError(t, holdRegisteredIdleServers(t.Context(), cache, server.Project, held))
	done := runFixtureWatcher(t, dir)
	require.Never(t, func() bool { _, err := os.Stat(filepath.Join(dir, "stopped")); return err == nil }, time.Second, 10*time.Millisecond)
	require.Len(t, held, 1)
	// A second call retains the same lease, rather than opening/leaking another.
	require.NoError(t, holdRegisteredIdleServers(t.Context(), cache, server.Project, held))
	require.Len(t, held, 1)
	for _, lease := range held {
		require.NoError(t, lease.Close())
	}
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after fallback lease was released")
	}
}

func TestIdleDiscoveryFallbackDoesNotHideLeaseErrors(t *testing.T) {
	t.Parallel()
	dir, server := idleFixture(t)
	cache := t.TempDir()
	registration := filepath.Join(cache, "moto-testdb", "v1", "registered")
	require.NoError(t, os.MkdirAll(filepath.Dir(registration), 0700))
	require.NoError(t, os.Symlink(dir, registration))
	held := make(map[string]*os.File)
	require.NoError(t, holdRegisteredIdleServers(t.Context(), cache, server.Project+"-other", held))
	require.Empty(t, held, "unrelated test ports must not be held")
	exclusive, err := lockFile(t.Context(), filepath.Join(dir, "users.lock"), syscall.LOCK_EX)
	require.NoError(t, err)
	defer func() { _ = exclusive.Close() }()
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, holdRegisteredIdleServers(ctx, cache, server.Project, held), context.DeadlineExceeded)
	require.Empty(t, held, "cleanup/startup contention must never be silently bypassed")
	require.NoError(t, exclusive.Close())
	require.NoError(t, os.Remove(filepath.Join(dir, "users.lock")))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "users.lock"), 0700))
	require.Error(t, holdRegisteredIdleServers(t.Context(), cache, server.Project, held), "invalid lease storage must remain an error")
}
