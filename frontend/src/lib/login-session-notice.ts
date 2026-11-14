import { useEffect, useState } from "react";
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
