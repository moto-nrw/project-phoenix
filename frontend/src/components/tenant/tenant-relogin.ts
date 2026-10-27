import { signOut } from "next-auth/react";
import { apiErrorFromResponse } from "~/lib/api-error";
import { clearSessionCache, DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { clearPersonalStorage } from "~/lib/personal-storage";
import { mutate } from "~/lib/swr";
import { clientEnv } from "~/env.client";
import { unsubscribePushSilently } from "~/lib/push-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantRelogin" });
let pendingRelogin: Promise<void> | null = null;

/** Same logout endpoints/order as the tenant shell, without its source-host redirect. */
export function reloginAtTenant(subdomain: string): Promise<void> {
  // Coalesce repeated clicks: no late logout response may reissue the cookie.
  pendingRelogin ??= endSessionAndNavigate(subdomain).finally(() => {
    pendingRelogin = null;
  });
  return pendingRelogin;
}

async function endSessionAndNavigate(subdomain: string): Promise<void> {
  await unsubscribePushSilently("tenant");
  // Revoke the refresh family BEFORE Auth.js deletes the shared domain cookie.
  // The response-aware BFF can refresh/reissue that cookie while logging out.
  const response = await fetch("/api/auth/logout", {
    method: "POST",
    signal: AbortSignal.timeout(5000),
  });
  // 401 means there is no live authenticated family left to revoke, e.g.
  // retrying after cookie deletion failed. Still remove its Auth.js cookie.
  if (!response.ok && response.status !== 401) {
    throw await apiErrorFromResponse(response, "Tenant logout failed");
  }
  try {
    await mutate(() => true, undefined, { revalidate: false });
  } catch (error) {
    // Cache cleanup must never prevent deletion of a revoked session cookie.
    logger.warn("relogin_cache_clear_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }
  // Nothing personal of the source school's account stays on the device (#5000).
  clearPersonalStorage();
  clearSessionCache();
  try {
    sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
  } catch {
    // Storage is optional; the server-side revocation and cookie deletion are not.
  }
  const port = window.location.port ? `:${window.location.port}` : "";
  const callbackUrl = `${window.location.protocol}//${subdomain}.${clientEnv.NEXT_PUBLIC_TENANT_DOMAIN}${port}/`;
  // Auth.js clears the domain cookie and navigates BEFORE refetching local
  // session consumers. A manual redirect after signOut({redirect:false}) can
  // lose to a protected page's source-school login redirect.
  await signOut({ callbackUrl });
}
