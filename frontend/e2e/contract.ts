import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  e2eManifestSchema,
  type E2EManifest,
  type E2ETwitterTenant,
  type E2ERuntimeConfig,
} from "./contract.generated";
export type {
  Device,
  E2EActivityRef,
  E2EAuthSetup,
  E2ECheckinFixture,
  E2EGroupPair,
  E2EGroupRef,
  E2EManifest,
  E2ERoomRef,
  E2ERuntimeConfig,
  E2ESetup,
  E2ETwitterTenant,
  E2EStudentPair,
  E2EStudentRef,
  OperatorCredentials,
  SeedAdminActor,
  SeedStaffActor,
  Switching,
} from "./contract.generated";
export {
  E2E_SCENARIO_MODE_MULTI_TENANT,
  E2E_SETUP_ROLE_ADMIN,
  E2E_SETUP_ROLE_STAFF,
} from "./contract.generated";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");
const REPO_ROOT = resolve(FRONTEND_DIR, "..");

export const E2E_MANIFEST_PATH = resolve(
  REPO_ROOT,
  "backend",
  ".e2e-manifest.json",
);

export interface AppUrls {
  primary(path?: string): string;
  secondary(path?: string): string;
  tenant(tenant: E2ETwitterTenant, path?: string): string;
  origin(tenant: E2ETwitterTenant): string;
}

let cachedManifest: E2EManifest | undefined;
let cachedAppUrls: AppUrls | undefined;

function normalizePath(path = "/"): string {
  return path.startsWith("/") ? path : `/${path}`;
}

function tenantOrigin(
  runtime: E2ERuntimeConfig,
  tenant: E2ETwitterTenant,
): string {
  return `http://${tenant.slug}.${runtime.tenant_domain}:${runtime.frontend_port}`;
}

function tenantAppURL(
  runtime: E2ERuntimeConfig,
  tenant: E2ETwitterTenant,
  path = "/",
): string {
  return `${tenantOrigin(runtime, tenant)}${normalizePath(path)}`;
}

function loadManifest(): E2EManifest {
  try {
    const raw = readFileSync(E2E_MANIFEST_PATH, "utf-8");
    const parsed = e2eManifestSchema.safeParse(JSON.parse(raw) as unknown);
    if (!parsed.success) {
      throw new Error(parsed.error.message);
    }
    return parsed.data;
  } catch (err) {
    throw new Error(
      `Could not read ${E2E_MANIFEST_PATH}. Run \`pnpm --dir frontend run e2e\` first.\n` +
        `Underlying error: ${err instanceof Error ? err.message : String(err)}`,
      { cause: err },
    );
  }
}

function createAppUrls(manifest: E2EManifest): AppUrls {
  const runtime = manifest.runtime;
  return {
    primary: (path = "/") =>
      tenantAppURL(runtime, manifest.tenants.primary, path),
    secondary: (path = "/") =>
      tenantAppURL(runtime, requireSecondaryTenant(manifest), path),
    tenant: (tenant, path = "/") => tenantAppURL(runtime, tenant, path),
    origin: (tenant) => tenantOrigin(runtime, tenant),
  };
}

export function getE2EManifest(): E2EManifest {
  if (!cachedManifest) {
    cachedManifest = loadManifest();
  }
  return cachedManifest;
}

export function getE2ERuntime(): E2ERuntimeConfig {
  return getE2EManifest().runtime;
}

export function getAppUrls(): AppUrls {
  if (!cachedAppUrls) {
    cachedAppUrls = createAppUrls(getE2EManifest());
  }
  return cachedAppUrls;
}

export function requireSecondaryTenant(
  manifest: E2EManifest = getE2EManifest(),
): E2ETwitterTenant {
  const tenant = manifest.tenants.secondary;
  if (!tenant) {
    throw new Error(
      "e2e manifest secondary tenant is missing. Re-run `pnpm --dir frontend run e2e`.",
    );
  }
  return tenant;
}

export function _resetContractCacheForTesting(): void {
  cachedManifest = undefined;
  cachedAppUrls = undefined;
}
