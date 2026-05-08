import { revalidateTag } from "next/cache";
import type { NextRequest } from "next/server";
import { apiPut, apiDelete } from "~/lib/api-helpers";
import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "~/lib/settings-keys";

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

// Mirrors proxy.ts extractTenantSlug — kept inline (not imported) because
// proxy.ts runs as Edge middleware and pulls in env-validation that the
// route handler shouldn't carry. The route handler reaches the slug from
// the original Host/X-Forwarded-Host header set by the browser.
function extractTenantSlugFromRequest(request: NextRequest): string | null {
  const tenantDomain = process.env.TENANT_DOMAIN;
  if (!tenantDomain) return null;
  const host =
    request.headers.get("x-forwarded-host") ??
    request.headers.get("host") ??
    "";
  const hostnameNoPort = host.split(":")[0] ?? "";
  if (
    hostnameNoPort !== tenantDomain &&
    hostnameNoPort.endsWith(`.${tenantDomain}`)
  ) {
    return hostnameNoPort.slice(0, -(tenantDomain.length + 1));
  }
  return null;
}

function maybeRevalidateTenant(request: NextRequest, key: string): void {
  if (!TENANT_RESOLVE_AFFECTING_KEYS.has(key)) return;
  const slug = extractTenantSlugFromRequest(request);
  if (!slug) return;
  // `expire: 0` forces immediate invalidation — match the pattern used by
  // /api/settings/login-image where the same cache tag is busted.
  revalidateTag(`tenant-${slug}`, { expire: 0 });
}

export const PUT = createPutHandler(
  async (
    request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    const result = await apiPut(`/api/settings/values/${key}`, token, body);
    maybeRevalidateTenant(request, key);
    return result;
  },
);

export const DELETE = createDeleteHandler(
  async (
    request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const key = params.key as string;
    if (!VALID_KEY_PATTERN.test(key)) {
      throw new Error("API error (400): Invalid settings key format");
    }
    const result = await apiDelete(`/api/settings/values/${key}`, token);
    maybeRevalidateTenant(request, key);
    return result;
  },
);
