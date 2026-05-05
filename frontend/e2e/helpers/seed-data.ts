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

// Optional second tenant created by `scripts/seed-e2e-multi-tenant.sh`.
// Tests that depend on this should check for its presence and skip
// gracefully when it is missing — the multi-tenant setup is opt-in.
export const SECOND_TENANT_SLUG = "second-school";
export const SECOND_TENANT_NAME = "Demo School 2";
export const SECOND_BASE_URL = `http://${SECOND_TENANT_SLUG}.${TENANT_DOMAIN}:${E2E_FRONTEND_PORT}`;

export const E2E_PASSWORD = "E2EPass1234!";

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
