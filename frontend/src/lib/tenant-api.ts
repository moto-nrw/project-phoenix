/**
 * Tenant resolution and switching API client.
 * Provides public tenant resolution (pre-login) and authenticated
 * tenant listing/switching (post-login).
 */

import { clearSessionCache, sessionFetch } from "./session-cache";

const TENANT_ACCESS_DENIED_MESSAGE =
  "account does not have access to this tenant";

export interface TenantSettings {
  logoUrl?: string;
  primaryColor?: string;
  [key: string]: unknown;
}

export interface TenantInfo {
  tenantId: number;
  slug: string;
  name: string;
  subdomain: string;
  organizationId: number;
  organizationName: string;
  settings: TenantSettings;
}

interface TenantResolveResponse {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
  settings: TenantSettings;
}

/**
 * Resolve a tenant slug to its metadata.
 * This is a public (no-auth) call used before login.
 */
export async function resolveTenant(slug: string): Promise<TenantInfo | null> {
  try {
    const response = await fetch(
      `/api/tenant/resolve?slug=${encodeURIComponent(slug)}`,
    );

    if (!response.ok) {
      return null;
    }

    const json = (await response.json()) as {
      status: string;
      data: TenantResolveResponse;
    };
    const data = json.data;
    return {
      tenantId: data.tenant_id,
      slug: data.slug,
      name: data.name,
      subdomain: data.subdomain,
      organizationId: data.organization_id,
      organizationName: data.organization_name,
      settings: data.settings ?? {},
    };
  } catch {
    return null;
  }
}

/** Public tenant response (no internal IDs) */
interface PublicTenantBackend {
  slug: string;
  name: string;
  subdomain: string;
  organization_name: string;
}

export interface TenantListResult {
  tenants: TenantInfo[];
  /** "ok" = backend responded successfully, "error" = network or server failure */
  status: "ok" | "error";
}

/**
 * List all active tenants with retry logic.
 * This is a public (no-auth) call used on the root tenant selector page.
 * The backend omits internal IDs from this public endpoint.
 *
 * Retries up to {@link maxAttempts} times with exponential backoff to
 * handle transient failures (e.g. after registration redirect).
 */
export async function listAllTenants(
  maxAttempts = 3,
): Promise<TenantListResult> {
  let lastError: unknown;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const response = await fetch("/api/tenant/list");
      if (!response.ok) {
        lastError = new Error(`HTTP ${response.status}`);
        if (attempt < maxAttempts) {
          await delay(1000 * 2 ** (attempt - 1));
          continue;
        }
        return { tenants: [], status: "error" };
      }

      const json = (await response.json()) as {
        data?: PublicTenantBackend[];
      };
      const items = json.data ?? [];
      return {
        tenants: items.map((t) => ({
          tenantId: 0,
          slug: t.slug,
          name: t.name,
          subdomain: t.subdomain,
          organizationId: 0,
          organizationName: t.organization_name,
          settings: {},
        })),
        status: "ok",
      };
    } catch (error) {
      lastError = error;
      if (attempt < maxAttempts) {
        await delay(1000 * 2 ** (attempt - 1));
        continue;
      }
    }
  }

  // All attempts exhausted
  void lastError; // acknowledged but not rethrown — callers check status
  return { tenants: [], status: "error" };
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Backend response shape for account tenants (snake_case) */
interface AccountTenantBackend {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
}

/**
 * List tenants the current user has access to.
 * Requires an authenticated session.
 */
export async function listAvailableTenants(): Promise<TenantInfo[]> {
  const response = await sessionFetch("/api/auth/account-tenants");
  if (!response.ok) {
    return [];
  }
  const json = (await response.json()) as { data?: AccountTenantBackend[] };
  const items = json.data ?? [];
  return items.map((t) => ({
    tenantId: t.tenant_id,
    slug: t.slug,
    name: t.name,
    subdomain: t.subdomain,
    organizationId: t.organization_id,
    organizationName: t.organization_name,
    settings: {},
  }));
}

/** Response shape from the switch-tenant endpoint */
interface SwitchTenantResponse {
  access_token: string;
  refresh_token: string;
}

interface ErrorResponseBody {
  error?: string;
  message?: string;
}

export class TenantSwitchError extends Error {
  status: number;
  code: "access_denied" | "unknown";

  constructor(
    message: string,
    status: number,
    code: "access_denied" | "unknown" = "unknown",
  ) {
    super(message);
    this.name = "TenantSwitchError";
    this.status = status;
    this.code = code;
  }
}

async function parseErrorMessage(response: Response): Promise<string> {
  const text = await response.text();

  if (!text) {
    return "Failed to switch tenant";
  }

  try {
    const payload = JSON.parse(text) as ErrorResponseBody;
    return payload.error ?? payload.message ?? text;
  } catch {
    return text;
  }
}

/**
 * Perform the full tenant switch sequence (steps 1-4 of the switch spec).
 * Returns the new tokens. Callers handle the final navigation/update (step 5).
 *
 * 1. Get new JWT tokens from backend
 * 2. Update NextAuth session via signIn("credentials", { internalRefresh })
 * 3. Clear SWR cache to prevent stale cross-tenant data
 * 4. Clear session cache for fresh token resolution
 */
export async function performTenantSwitch(
  slug: string,
  signIn: (
    provider: string,
    options: Record<string, unknown>,
  ) => Promise<unknown>,
  swrMutate: (
    matcher: (key: unknown) => boolean,
    data: undefined,
    opts: { revalidate: boolean },
  ) => Promise<unknown>,
): Promise<SwitchTenantResponse> {
  const tokens = await switchTenant(slug);

  await signIn("credentials", {
    redirect: false,
    internalRefresh: true,
    token: tokens.access_token,
    refreshToken: tokens.refresh_token,
  });

  await swrMutate(() => true, undefined, { revalidate: false });

  clearSessionCache();

  return tokens;
}

/**
 * Switch the current session to a different tenant.
 * Returns new JWT tokens scoped to the target tenant.
 */
export async function switchTenant(
  slug: string,
): Promise<SwitchTenantResponse> {
  const response = await sessionFetch("/api/auth/switch-tenant", {
    method: "POST",
    body: JSON.stringify({ tenant_slug: slug }),
  });
  if (!response.ok) {
    const message = await parseErrorMessage(response);
    const code =
      response.status === 401 && message === TENANT_ACCESS_DENIED_MESSAGE
        ? "access_denied"
        : "unknown";
    throw new TenantSwitchError(message, response.status, code);
  }
  return (await response.json()) as SwitchTenantResponse;
}
