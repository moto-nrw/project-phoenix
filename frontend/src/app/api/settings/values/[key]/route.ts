import type { NextRequest } from "next/server";
import { revalidatePath, revalidateTag } from "next/cache";
import { apiPut, apiDelete } from "~/lib/api-helpers";
import { createPutHandler, createDeleteHandler } from "~/lib/route-wrapper";
import { TENANT_RESOLVE_AFFECTING_KEYS } from "~/lib/settings-keys";
import { tenantSlugFromHost } from "~/lib/tenant-host";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SettingsValuesRoute" });

const VALID_KEY_PATTERN = /^[a-z0-9_.]{1,255}$/;

// Settings whose value rides on /auth/tenant/resolve are tagged with
// `tenant-${slug}` in the layout fetch (revalidate: 300). A naked
// router.refresh() on the client would still see the cached value until
// the TTL expires, so we purge the tag here before responding.
function revalidateTenantIfNeeded(request: NextRequest, key: string): void {
  if (!TENANT_RESOLVE_AFFECTING_KEYS.has(key)) return;

  const slug = tenantSlugFromHost(request.headers.get("host"));
  if (!slug) {
    logger.warn("tenant_revalidate_skipped_no_slug", {
      key,
      host: request.headers.get("host") ?? "",
    });
    return;
  }

  revalidateTag(`tenant-${slug}`, { expire: 0 });
  revalidatePath(`/${slug}`, "layout");
  logger.info("tenant_cache_revalidated", { slug, key });
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
    revalidateTenantIfNeeded(request, key);
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
    revalidateTenantIfNeeded(request, key);
    return result;
  },
);
