/**
 * Refuses any backend URL that doesn't resolve to a local development host.
 *
 * Mirrors the Go-side guard in `backend/cmd/safeguard.go:assertNonProductionURL`.
 * If `E2E_BACKEND_URL` is ever pointed at staging or production by mistake,
 * the seeder/test scripts can write `E2EPass1234!` into a real environment.
 * Three guard layers — TS (this file), bash (`scripts/lib/assert-local-url.sh`),
 * Go — keep the allowlist consistent and catch each other's bypasses.
 */

const ALLOWED_HOSTNAMES = new Set([
  "localhost",
  "127.0.0.1",
  "::1",
  // Docker-internal hostnames used in development. Keep in sync with
  // backend/cmd/safeguard.go:localDockerHostnames and
  // scripts/lib/assert-local-url.sh.
  "server",
  "server-e2e",
  "host.docker.internal",
  "gateway.docker.internal",
]);

export function assertLocalBackendUrl(rawUrl: string): void {
  let parsed: URL;
  try {
    parsed = new URL(rawUrl);
  } catch (err) {
    throw new Error(
      `invalid backend URL ${JSON.stringify(rawUrl)}: ${
        err instanceof Error ? err.message : String(err)
      }`, { cause: err },
    );
  }

  const host = parsed.hostname.toLowerCase();
  if (!host) {
    throw new Error(`backend URL ${JSON.stringify(rawUrl)} has no hostname`);
  }

  if (ALLOWED_HOSTNAMES.has(host)) return;

  // Any IPv4 loopback (127.0.0.0/8), not just 127.0.0.1.
  if (/^127\.\d+\.\d+\.\d+$/.test(host)) return;

  throw new Error(
    `refusing to run E2E against ${rawUrl} — only localhost, ` +
      `loopback, and Docker-internal hostnames are permitted. ` +
      `Set E2E_BACKEND_URL to a local URL.`,
  );
}
