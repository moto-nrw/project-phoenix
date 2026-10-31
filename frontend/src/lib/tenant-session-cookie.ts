import { apiErrorFromResponse, unavailableApiError } from "./api-error";
import { markTenantSessionNavigation } from "./tenant-session-end";

/**
 * Auth.js' tenant signout protocol with cancellation between its CSRF and
 * cookie-deletion requests. The SDK's signOut cannot receive an AbortSignal.
 * Navigation belongs to the current attempt, not a late SDK continuation.
 */
export async function signOutTenant(
  { callbackUrl }: { callbackUrl: string },
  signal: AbortSignal,
): Promise<void> {
  signal.throwIfAborted();
  const csrf = await fetch("/api/auth/csrf", { signal });
  signal.throwIfAborted();
  if (!csrf.ok) throw await apiErrorFromResponse(csrf, "Logout CSRF failed");
  const body = (await csrf.json()) as { csrfToken?: unknown };
  signal.throwIfAborted();
  if (typeof body.csrfToken !== "string" || !body.csrfToken) {
    throw unavailableApiError(new Error("Logout CSRF token missing"));
  }
  const response = await fetch("/api/auth/signout", {
    method: "POST",
    signal,
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
      "X-Auth-Return-Redirect": "1",
    },
    body: new URLSearchParams({ csrfToken: body.csrfToken, callbackUrl }),
  });
  signal.throwIfAborted();
  // Preserve the SDK's JSON-error fallback too: after family revocation,
  // the target guard can explicitly retry a cookie deletion that failed.
  const data = (await response.json()) as { url?: string };
  signal.throwIfAborted();
  // Both callers clearPersonalStorage before this request. Its owner-key
  // removal already invalidates other tabs through SWRScopeBoundary, without
  // waking this document's SessionProvider while navigation is pending.
  // Keep the menu's relative home destination, as its SDK redirect:false
  // path did; Auth.js' configured base URL can be a different tenant host.
  window.location.href = callbackUrl === "/" ? "/" : (data.url ?? callbackUrl);
  markTenantSessionNavigation(signal);
}
