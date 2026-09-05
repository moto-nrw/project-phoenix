//go:build darwin || linux

package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const idleLabel = "de.moto.testdb.lifecycle"

var errIdleUnmanaged = errors.New("container is not opted into managed cleanup")

var serverLeases = struct {
	sync.Mutex
	files map[string]*os.File
}{files: make(map[string]*os.File)}

type idleServer struct {
	Host    string
	Project string
	Docker  string
}

// HoldServer protects a local test server for this process's entire lifetime.
// Kernel locks survive pauses and disappear on exit, including SIGKILL. Keep
// the files reachable: a finalizer must never release a live process's lease.
// CI and remote Docker engines are deliberately outside this local lifecycle.
func HoldServer(ctx context.Context, cfg *Config) error {
	if os.Getenv("CI") != "" || !isLoopbackHost(cfg.templateURL.Hostname()) {
		return nil
	}
	serverLeases.Lock()
	defer serverLeases.Unlock()
	project := composeProjectFor(cfg)
	server, err := localIdleServer(ctx, project)
	if err != nil {
		return err
	}
	if server == nil {
		return nil
	}
	identity := server.Host + "\n" + project
	if _, ok := serverLeases.files[identity]; ok {
		return nil
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
	dir := filepath.Join(cache, "moto-testdb", "v1", key)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	lease, err := lockFile(ctx, filepath.Join(dir, "users.lock"), syscall.LOCK_SH)
	if err != nil {
		return err
	}
	success := false
	defer func() {
		if !success {
			_ = lease.Close()
		}
	}()
	// Record brief users that begin and end between watcher polls.
	now := time.Now()
	if err := os.Chtimes(lease.Name(), now, now); err != nil {
		return err
	}
	watcher, err := tryLockFile(filepath.Join(dir, "watcher.lock"), syscall.LOCK_EX)
	if err != nil {
		return err
	}
	if watcher != nil {
		defer func() { _ = watcher.Close() }()
		data, err := json.Marshal(server)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "server.json"), data, 0600); err != nil {
			return err
		}
		binary, err := buildIdleWatcher(ctx, filepath.Join(cache, "moto-testdb", "v1"))
		if err != nil {
			return err
		}
		log, err := os.OpenFile(filepath.Join(dir, "watcher.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if err != nil {
			return err
		}
		defer func() { _ = log.Close() }()
		cmd := exec.Command(binary, "--idle-watch", dir)
		cmd.Dir = dir // Keep working after the originating worktree is removed.
		cmd.ExtraFiles = []*os.File{watcher}
		// The watcher needs Docker access, not the test runner's credentials.
		cmd.Env = []string{}
		for _, key := range []string{"PATH", "HOME", "DOCKER_CONFIG"} {
			if value, ok := os.LookupEnv(key); ok {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
		}
		cmd.Stdout, cmd.Stderr = log, log
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if err := cmd.Start(); err != nil {
			return err
		}
		// Reap it if we outlive it; do not leave zombies in a long-running test.
		go func() { _ = cmd.Wait() }()
	}
	serverLeases.files[identity] = lease
	success = true
	return nil
}

func localIdleServer(ctx context.Context, project string) (*idleServer, error) {
	docker, err := exec.LookPath("docker")
	if errors.Is(err, exec.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	host := os.Getenv("DOCKER_HOST")
	// DOCKER_CONTEXT takes precedence over DOCKER_HOST in the Docker CLI.
	if os.Getenv("DOCKER_CONTEXT") != "" || host == "" {
		cmd := exec.CommandContext(ctx, docker, "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("resolve local Docker endpoint: %w", err)
		}
		host = strings.TrimSpace(string(out))
	}
	if !strings.HasPrefix(host, "unix://") {
		return nil, nil
	}
	socket, err := filepath.EvalSymlinks(strings.TrimPrefix(host, "unix://"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("resolve Docker socket: %w", err)
	}
	return &idleServer{Host: "unix://" + socket, Project: project, Docker: docker}, nil
}

func tryLockFile(path string, mode int) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), mode|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

func lockFile(ctx context.Context, path string, mode int) (*os.File, error) {
	for {
		f, err := tryLockFile(path, mode)
		if err != nil || f != nil {
			return f, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// Build a content-addressed helper once. It remains runnable after its source
// worktree is removed and requires no global installation or OS service.
func buildIdleWatcher(ctx context.Context, cache string) (string, error) {
	root, err := backendRoot()
	if err != nil {
		return "", err
	}
	sources, err := filepath.Glob(filepath.Join(root, "internal", "testdb", "*.go"))
	if err != nil {
		return "", err
	}
	sources = append(sources, filepath.Join(root, "internal/testdb/cmd/bootstrap/main.go"), filepath.Join(root, "go.mod"), filepath.Join(root, "go.sum"))
	hash := sha256.New()
	for _, path := range sources {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		_, _ = hash.Write(data)
	}
	binary := filepath.Join(cache, fmt.Sprintf("idle-%x", hash.Sum(nil)[:12]))
	lock, err := lockFile(ctx, filepath.Join(cache, "build.lock"), syscall.LOCK_EX)
	if err != nil {
		return "", err
	}
	defer func() { _ = lock.Close() }()
	if _, err := os.Stat(binary); err == nil {
		return binary, nil
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, "./internal/testdb/cmd/bootstrap")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build idle watcher: %w: %s", err, out)
	}
	return binary, nil
}

// WatchIdle runs in a detached helper. The inherited singleton lock and the
// exclusive users lock must be released in that order when exiting: a new
// starter then either sees this watcher or starts its replacement, never a gap.
func WatchIdle(ctx context.Context, dir string, singleton *os.File, idle, poll time.Duration) error {
	if idle <= 0 || poll <= 0 {
		return fmt.Errorf("idle and poll durations must be positive")
	}
	data, err := os.ReadFile(filepath.Join(dir, "server.json"))
	if err != nil {
		return err
	}
	var server idleServer
	if err := json.Unmarshal(data, &server); err != nil {
		return err
	}
	lastUsed := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
		lease, err := tryLockFile(filepath.Join(dir, "users.lock"), syscall.LOCK_EX)
		if err != nil {
			return err
		}
		if lease == nil {
			lastUsed = time.Now()
			continue
		}
		info, err := lease.Stat()
		if err != nil {
			_ = lease.Close()
			return err
		}
		if info.ModTime().After(lastUsed) {
			lastUsed = info.ModTime()
		}
		if time.Since(lastUsed) < idle {
			_ = lease.Close()
			continue
		}
		done, err := server.stopIdle(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "testdb idle: %v\n", err)
			lastUsed = time.Now() // Docker/inspection failures are not permission to stop.
			// Do not leave a background helper behind for legacy servers.
			done = errors.Is(err, errIdleUnmanaged)
		}
		if done {
			_ = singleton.Close()
			_ = lease.Close()
			return nil
		}
		_ = lease.Close()
	}
}

func (s idleServer) command(ctx context.Context, args ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(commandCtx, s.Docker, append([]string{"--host", s.Host}, args...)...)
	// Pin the engine even if the invoking shell selected a different context.
	cmd.Env = replaceCommandEnvironment(os.Environ(), "DOCKER_CONTEXT", "")
	return cmd.CombinedOutput()
}

type idleContainer struct {
	ID         string
	Config     struct{ Labels map[string]string }
	State      struct{ Running bool }
	HostConfig struct {
		PortBindings map[string][]struct{ HostPort string }
	}
}

func (s idleServer) candidate(ctx context.Context) (*idleContainer, error) {
	if !strings.HasPrefix(s.Host, "unix://") || !strings.HasPrefix(s.Project, composeProjectPrefix+"-") {
		return nil, fmt.Errorf("not a managed local test server: %w", errIdleUnmanaged)
	}
	out, err := s.command(ctx, "ps", "-aq", "--filter", "label=com.docker.compose.project="+s.Project, "--filter", "label=com.docker.compose.service=postgres-test")
	if err != nil {
		return nil, fmt.Errorf("list test container: %w", err)
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) != 1 {
		return nil, fmt.Errorf("ambiguous test containers; refusing cleanup")
	}
	out, err = s.command(ctx, "inspect", ids[0])
	if err != nil {
		return nil, fmt.Errorf("inspect test container: %w", err)
	}
	var containers []idleContainer
	if err := json.Unmarshal(out, &containers); err != nil {
		return nil, err
	}
	if len(containers) != 1 {
		return nil, fmt.Errorf("unexpected container inspection")
	}
	c := containers[0]
	labels := c.Config.Labels
	port := strings.TrimPrefix(s.Project, composeProjectPrefix+"-")
	bindings := c.HostConfig.PortBindings["5432/tcp"]
	if c.ID == "" || labels[idleLabel] != "1" || labels["com.docker.compose.project"] != s.Project || labels["com.docker.compose.service"] != "postgres-test" || len(bindings) == 0 {
		return nil, errIdleUnmanaged
	}
	for _, binding := range bindings {
		if binding.HostPort != port {
			return nil, fmt.Errorf("test container port mismatch: %w", errIdleUnmanaged)
		}
	}
	return &c, nil
}

func (s idleServer) stopIdle(ctx context.Context) (bool, error) {
	c, err := s.candidate(ctx)
	if err != nil {
		return false, err
	}
	if c == nil || !c.State.Running {
		return true, nil
	}
	// Additional protection for manual SQL sessions and older test binaries.
	// Managed tests are protected before connecting by the kernel lease above.
	out, err := s.command(ctx, "exec", c.ID, "psql", "-XAt", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "postgres", "-c", "SELECT count(*) FROM pg_stat_activity WHERE backend_type = 'client backend' AND pid <> pg_backend_pid()")
	if err != nil {
		return false, fmt.Errorf("inspect database clients: %w", err)
	}
	if strings.TrimSpace(string(out)) != "0" {
		return false, nil
	}
	if _, err := s.command(ctx, "stop", "--timeout", "-1", c.ID); err != nil {
		return false, fmt.Errorf("stop idle test container: %w", err)
	}
	fmt.Printf("testdb idle: stopped %s (volumes retained)\n", s.Project)
	return true, nil
}

// IdleStatus is read-only. Unknown/legacy containers are never adopted.
func IdleStatus(ctx context.Context) error {
	cache, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	paths, err := filepath.Glob(filepath.Join(cache, "moto-testdb", "v1", "*", "server.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var server idleServer
		if err := json.Unmarshal(data, &server); err != nil {
			return err
		}
		lease, err := tryLockFile(filepath.Join(filepath.Dir(path), "users.lock"), syscall.LOCK_EX)
		if err != nil {
			return err
		}
		busy := lease == nil
		if lease != nil {
			_ = lease.Close()
		}
		c, err := server.candidate(ctx)
		state := "absent"
		if err != nil {
			state = "unmanaged/unavailable"
		} else if c != nil {
			state = "stopped"
			if c.State.Running {
				state = "running"
			}
		}
		fmt.Printf("%s\t%s\tactive_test_process=%t\n", server.Project, state, busy)
	}
	return nil
}
