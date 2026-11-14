import { useEffect, useState } from "react";
import type { Session } from "next-auth";
import { signOut } from "next-auth/react";
import type { ReadonlyURLSearchParams } from "next/navigation";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";

const SESSION_ERRORS = new Set(["SessionRequired", "SessionExpired"]);

/** Takes NextAuth's session error and its callbackUrl out of the address. */
export function clearSessionErrorFromUrl() {
  const url = new URL(window.location.href);
  if (!SESSION_ERRORS.has(url.searchParams.get("error") ?? "")) return;
  url.searchParams.delete("error");
  url.searchParams.delete("callbackUrl");
  window.history.replaceState(
    {},
    "",
    `${url.pathname}${url.search}${url.hash}`,
  );
}

/**
 * Whether a login page shows „Sitzung abgelaufen“. NextAuth's
 * useSession({ required: true }) races the logout navigation and can
 * redirect to the login with ?error=SessionRequired before the page
 * unloads, so a deliberate logout shows no notice.
 */
export function useSessionExpiredNotice(searchParams: ReadonlyURLSearchParams) {
  const [sessionExpired, setSessionExpired] = useState(false);
  useEffect(() => {
    if (!SESSION_ERRORS.has(searchParams.get("error") ?? "")) return;
    let deliberate = false;
    try {
      deliberate = sessionStorage.getItem(DELIBERATE_LOGOUT_KEY) === "1";
      sessionStorage.removeItem(DELIBERATE_LOGOUT_KEY);
    } catch {
      // sessionStorage unavailable: show the notice, it does no harm.
    }
    if (!deliberate) setSessionExpired(true);
    clearSessionErrorFromUrl();
  }, [searchParams]);
  return [sessionExpired, setSessionExpired] as const;
}

interface StaleSessionSignOut {
  readonly status: "authenticated" | "loading" | "unauthenticated";
  readonly session: Session | null;
  /** Set while a sign-out runs or is done, so one stale session ends once. */
  readonly started: { current: boolean };
  readonly logger: { warn: (event: string, data: { error: string }) => void };
  /** The sign-out failed; a later call tries again. */
  readonly onFailure?: (cause: unknown) => void;
}

/**
 * Ends a session the backend no longer refreshes when a login page opens.
 * Without this the JWT callback keeps retrying the refused refresh, and the
 * stale cookie competes with the new sign-in. Answers null, at once, when
 * there is nothing to end or a sign-out already runs, so the page goes on
 * to its other checks in the same tick; otherwise the sign-out it started. Nothing is shown for a failure
 * unless the page asks for it: no one requested the cleanup.
 */
export function signOutStaleSession({
  status,
  session,
  started,
  logger,
  onFailure,
}: StaleSessionSignOut): Promise<void> | null {
  if (status !== "authenticated" || !session?.error || started.current) {
    return null;
  }
  started.current = true;
  return signOut({ redirect: false }).then(
    () => undefined,
    (cause: unknown) => {
      started.current = false;
      logger.warn("stale_session_signout_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      onFailure?.(cause);
    },
  );
}
