import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

/**
 * Single source of truth for E2E test data.
 *
 * These constants must match the flags passed to the deterministic seeder
 * (`scripts/seed-e2e.sh`). Changing a value here without updating the seeder
 * script will break every test that authenticates.
 *
 * Why these specific accounts?
 * - `demo1@mail.de` is seeded with the OGS-Büro position → admin role
 * - `demo11@mail.de` is seeded with Pädagogische Fachkraft → user role
 * Both are part of the static `DemoStaff` array in
 * `backend/seed/api/data.go`, so their identity is stable across runs.
 */

// Resolve relative to THIS FILE so STORAGE_STATE_PATH is correct regardless
// of process.cwd(). Playwright resolves relative `storageState` paths
// against cwd for both writes (page.context().storageState({ path })) and
// reads (project use.storageState), so a relative form silently scattered
// auth artifacts to <cwd>/e2e/.auth/ when the suite was launched from
// anywhere other than frontend/. Anchoring to the file's own URL via
// import.meta.url keeps the resolution invariant.
const HERE = dirname(fileURLToPath(import.meta.url));
// helpers/ → e2e/ → frontend/
const FRONTEND_DIR = resolve(HERE, "..", "..");

export const TENANT_SLUG = "demo-school";
export const TENANT_NAME = "Demo School";

// localtest.me is a public DNS wildcard pointing to 127.0.0.1 — used here
// instead of *.localhost so NextAuth can set domain-scoped cookies and
// tenant switching shares sessions across subdomains. See the matching
// TENANT_DOMAIN value in the root .env.
export const TENANT_DOMAIN = "localtest.me";

// E2E runs on :3030 so a developer's dev frontend on :3000 can keep
// running alongside. Must stay in sync with playwright.config.ts:webServer
// and with server-e2e's CORS_ALLOWED_ORIGINS in docker-compose.example.yml.
// :3030 (not :3001) because :3001 is commonly taken by other dev tools.
export const E2E_FRONTEND_PORT = 3030;
export const BASE_URL = `http://${TENANT_SLUG}.${TENANT_DOMAIN}:${E2E_FRONTEND_PORT}`;

// Second-tenant constants live in seed-state.ts (`getSecondTenant()`)
// because the seeder is the single source of truth for slug/name/admin
// after we moved the multi-tenant provisioning into Go. Importing them
// here would invert the dependency: the source of the slug should be
// the seeder, not a frontend constant. See frontend/e2e/helpers/seed-state.ts.

// Test-only credential. Used exclusively by the deterministic seeder for
// the isolated postgres-test stack (server-e2e on :8081). Never exists in
// any real environment, never authenticates against any production system,
// and is gitignored in seed-state JSON. The seeder accepts it as a flag,
// so changing it here requires updating scripts/seed-e2e.sh in lockstep.
export const E2E_PASSWORD = "E2EPass1234!"; // NOSONAR — test fixture, not a real credential

export const ADMIN = {
  email: "demo1@mail.de",
  password: E2E_PASSWORD,
  role: "admin" as const,
  firstName: "Anna",
  lastName: "Müller",
  displayName: "Anna Müller",
};

export const STAFF = {
  email: "demo11@mail.de",
  password: E2E_PASSWORD,
  role: "user" as const,
  firstName: "Julia",
  lastName: "Klein",
  displayName: "Julia Klein",
};

export type Role = "admin" | "staff";

export const STORAGE_STATE_PATH: Record<Role, string> = {
  admin: resolve(FRONTEND_DIR, "e2e", ".auth", "admin.json"),
  staff: resolve(FRONTEND_DIR, "e2e", ".auth", "staff.json"),
};
