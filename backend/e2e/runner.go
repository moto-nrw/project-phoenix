package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	contract "github.com/moto-nrw/project-phoenix/e2e/contract"
	"github.com/moto-nrw/project-phoenix/e2e/scenarios"
)

const (
	frontendReadyTimeout = 3 * time.Minute
	e2eComposeProject    = "phoenix-e2e"
	e2eBackendService    = "backend"
	e2ePostgresService   = "postgres"
)

type RunOptions struct {
	Scenario string
	Verbose  bool
}

type harnessPaths struct {
	repoRoot    string
	backendRoot string
	frontendDir string
	composeFile string
}

func Run(ctx context.Context, options RunOptions) error {
	options.normalize()
	paths, err := resolveHarnessPaths()
	if err != nil {
		return err
	}

	session, err := newHarnessSession(paths, options.Scenario, options.Verbose)
	if err != nil {
		return err
	}
	defer session.cleanup()

	if err := session.startInfra(ctx); err != nil {
		return err
	}
	if err := session.prepareWorld(ctx); err != nil {
		return err
	}

	state, err := session.loadState()
	if err != nil {
		return err
	}

	frontend, err := session.startFrontend(state)
	if err != nil {
		return err
	}
	defer frontend.stop()

	if err := waitForHTTP(statePrimaryOrigin(state), frontendReadyTimeout); err != nil {
		return fmt.Errorf("frontend did not become ready: %w", err)
	}

	if err := session.runPlaywright(ctx); err != nil {
		return err
	}

	return nil
}

func (o *RunOptions) normalize() {
	if o.Scenario == "" {
		o.Scenario = defaultScenarioName()
	}
}

func defaultScenarioName() string {
	return scenarios.DefaultPrepareScenario().Name
}

type harnessSession struct {
	paths      harnessPaths
	scenario   string
	verbose    bool
	composeEnv []string
}

func newHarnessSession(paths harnessPaths, scenario string, verbose bool) (*harnessSession, error) {
	definition, ok := scenarios.Lookup(scenario)
	if !ok {
		return nil, fmt.Errorf("unknown e2e scenario %q", scenario)
	}

	composeEnv, err := buildComposeEnv(paths.repoRoot, definition)
	if err != nil {
		return nil, err
	}

	return &harnessSession{
		paths:      paths,
		scenario:   scenario,
		verbose:    verbose,
		composeEnv: composeEnv,
	}, nil
}

func (s *harnessSession) cleanup() {
	_ = s.captureInfraLogs()
	_ = s.stopInfra(context.Background())
}

func (s *harnessSession) startInfra(ctx context.Context) error {
	if err := ensureScenarioHostsResolvable(s.scenario); err != nil {
		return err
	}

	return runCommand(
		ctx,
		s.paths.repoRoot,
		s.composeEnv,
		os.Stdout,
		os.Stderr,
		"docker",
		s.composeArgs("up", "-d", "--wait", "--force-recreate", e2ePostgresService, e2eBackendService)...,
	)
}

func (s *harnessSession) stopInfra(ctx context.Context) error {
	return runCommand(
		ctx,
		s.paths.repoRoot,
		s.composeEnv,
		io.Discard,
		io.Discard,
		"docker",
		s.composeArgs("down", "--volumes", "--remove-orphans")...,
	)
}

func (s *harnessSession) prepareWorld(ctx context.Context) error {
	args := []string{
		"exec", "-T",
		e2eBackendService,
		"go", "run", "main.go", "e2e", "prepare",
		"--scenario", s.scenario,
	}
	if s.verbose {
		args = append(args, "--verbose")
	}
	return runCommand(ctx, s.paths.repoRoot, s.composeEnv, os.Stdout, os.Stderr, "docker", s.composeArgs(args...)...)
}

func (s *harnessSession) loadState() (*contract.State, error) {
	return contract.LoadState(filepath.Join(s.paths.backendRoot, contract.StatePath))
}

func (s *harnessSession) startFrontend(state *contract.State) (*managedProcess, error) {
	env := os.Environ()
	for key, value := range frontendEnv(state) {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	logPath := filepath.Join(s.paths.frontendDir, ".e2e-frontend.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("create frontend log: %w", err)
	}

	cmd := exec.Command("pnpm", "run", "dev", "--port", fmt.Sprint(state.Runtime.FrontendPort))
	cmd.Dir = s.paths.frontendDir
	cmd.Env = env
	cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
	cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return nil, fmt.Errorf("start frontend dev server: %w", err)
	}

	return &managedProcess{
		cmd:     cmd,
		logFile: logFile,
	}, nil
}

func (s *harnessSession) runPlaywright(ctx context.Context) error {
	return runCommand(
		ctx,
		s.paths.frontendDir,
		append(os.Environ(), "PHOENIX_E2E_HARNESS=1"),
		os.Stdout,
		os.Stderr,
		"pnpm",
		"exec",
		"playwright",
		"test",
	)
}

func (s *harnessSession) captureInfraLogs() error {
	artifactDir := filepath.Join(s.paths.backendRoot, ".e2e-artifacts", "backend-logs")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return fmt.Errorf("create e2e artifact dir: %w", err)
	}

	for _, service := range []string{e2eBackendService, e2ePostgresService} {
		if err := s.writeComposeLogs(service, filepath.Join(artifactDir, service+".log")); err != nil {
			return err
		}
	}
	return nil
}

func (s *harnessSession) writeComposeLogs(service, path string) error {
	logFile, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create log file %s: %w", path, err)
	}
	defer logFile.Close()

	cmd := exec.Command("docker", s.composeArgs("logs", "--no-color", "--tail=2000", service)...)
	cmd.Dir = s.paths.repoRoot
	cmd.Env = s.composeEnv
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("capture compose logs for %s: %w", service, err)
	}
	return nil
}

type managedProcess struct {
	cmd     *exec.Cmd
	logFile *os.File
}

func (p *managedProcess) stop() {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(p.cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}
	done := make(chan struct{})
	go func() {
		_ = p.cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		if pgid, err := syscall.Getpgid(p.cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
	}
	_ = p.logFile.Close()
}

func resolveHarnessPaths() (harnessPaths, error) {
	_, currentFile, _, ok := goruntime.Caller(0)
	if !ok {
		return harnessPaths{}, fmt.Errorf("resolve harness paths: runtime.Caller failed")
	}

	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), ".."))
	repoRoot := filepath.Clean(filepath.Join(backendRoot, ".."))
	frontendDir := filepath.Join(repoRoot, "frontend")
	composeFile := filepath.Join(backendRoot, "e2e", "compose.yml")

	if _, err := os.Stat(composeFile); err != nil {
		return harnessPaths{}, fmt.Errorf("resolve harness paths: %w", err)
	}

	return harnessPaths{
		repoRoot:    repoRoot,
		backendRoot: backendRoot,
		frontendDir: frontendDir,
		composeFile: composeFile,
	}, nil
}

func buildComposeEnv(repoRoot string, definition scenarios.Definition) ([]string, error) {
	basePath := filepath.Join(repoRoot, ".env")
	baseEnv, err := godotenv.Read(basePath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w; copy .env.example to .env before running the E2E harness", basePath, err)
	}

	for key := range baseEnv {
		if override, ok := os.LookupEnv(key); ok && strings.TrimSpace(override) != "" {
			baseEnv[key] = override
		}
	}

	overrides, err := composeRuntimeEnv(definition, baseEnv)
	if err != nil {
		return nil, err
	}
	for key, value := range overrides {
		baseEnv[key] = value
	}

	return mergeEnv(os.Environ(), baseEnv), nil
}

func composeRuntimeEnv(definition scenarios.Definition, baseEnv map[string]string) (map[string]string, error) {
	postgresPassword := strings.TrimSpace(baseEnv["POSTGRES_PASSWORD"])
	if postgresPassword == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD is required in .env for the E2E harness")
	}

	primaryOrigin := fmt.Sprintf(
		"http://%s.%s:%d",
		definition.Seed.TenantSlug,
		canonicalTenantDomain,
		canonicalFrontendPort,
	)

	return map[string]string{
		"APP_ENV":                       "test",
		"CLEANUP_SCHEDULER_ENABLED":     "false",
		"CORS_ALLOWED_ORIGINS":          fmt.Sprintf("*.%s:%d,http://localhost:%d", canonicalTenantDomain, canonicalFrontendPort, canonicalFrontendPort),
		"DB_DSN":                        fmt.Sprintf("postgres://postgres:%s@postgres:5432/phoenix_test?sslmode=disable", url.QueryEscape(postgresPassword)),
		"ENABLE_CORS":                   "true",
		"FRONTEND_URL":                  primaryOrigin,
		"NEXT_PUBLIC_OPERATOR_HOSTNAME": fmt.Sprintf("%s:%d", canonicalOperatorHost(), canonicalFrontendPort),
		"OGS_DEVICE_PIN":                definition.Identity.StaffPIN,
		"PORT":                          "8080",
		"SESSION_CLEANUP_ENABLED":       "false",
		"SESSION_END_SCHEDULER_ENABLED": "false",
	}, nil
}

func frontendEnv(state *contract.State) map[string]string {
	primaryOrigin := statePrimaryOrigin(state)
	return map[string]string{
		"SKIP_ENV_VALIDATION":           "true",
		"PORT":                          fmt.Sprint(state.Runtime.FrontendPort),
		"API_URL":                       state.Runtime.BackendURL,
		"NEXT_PUBLIC_API_URL":           state.Runtime.BackendURL,
		"TENANT_DOMAIN":                 state.Runtime.TenantDomain,
		"NEXT_PUBLIC_TENANT_DOMAIN":     state.Runtime.TenantDomain,
		"NEXT_PUBLIC_OPERATOR_HOSTNAME": state.Runtime.OperatorHostname,
		"NEXTAUTH_URL":                  primaryOrigin,
		"NEXTAUTH_SECRET":               state.Runtime.NextAuthSecret,
		"AUTH_TRUST_HOST":               fmt.Sprint(state.Runtime.AuthTrustHost),
	}
}

func (s *harnessSession) composeArgs(args ...string) []string {
	base := []string{
		"compose",
		"-p", e2eComposeProject,
		"-f", s.paths.composeFile,
	}
	return append(base, args...)
}

func statePrimaryOrigin(state *contract.State) string {
	return stateTenantOrigin(state, state.World.Tenants.Primary)
}

func stateTenantOrigin(state *contract.State, tenant contract.Tenant) string {
	return fmt.Sprintf("http://%s.%s:%d", tenant.Slug, state.Runtime.TenantDomain, state.Runtime.FrontendPort)
}

func waitForHTTP(rawURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		resp, err := client.Get(rawURL)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
			lastErr = fmt.Errorf("received status %d from %s", resp.StatusCode, rawURL)
		} else {
			lastErr = err
		}
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("timed out waiting for %s: %w", rawURL, lastErr)
}

func mergeEnv(parent []string, overrides map[string]string) []string {
	merged := make(map[string]string, len(parent)+len(overrides))
	for _, entry := range parent {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+merged[key])
	}
	return out
}

func runCommand(ctx context.Context, dir string, env []string, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}
