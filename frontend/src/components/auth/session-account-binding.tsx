"use client";

import { useLayoutEffect } from "react";
import { useSession } from "next-auth/react";
import { confirmSessionSnapshot } from "~/lib/session-cache";
import { hasSessionAccess } from "~/lib/session-access";
import { bindSessionAccount } from "~/lib/session-account";
import { installSessionTenantMismatchObserver } from "~/lib/session-tenant-mismatch-observer";

// Installed when the module loads: the portal's pages fetch before any
// effect of the binding runs.
installSessionTenantMismatchObserver();

/**
 * Binds every request of this tab to its BFF to the account the
 * SessionProvider shows (#5183). The BFF refuses a request whose cookie
 * holds another account, for example after another account signed in in a
 * second tab; the refusal wakes the provider like a school mismatch, and
 * SWRScopeBoundary and PersonalStorageGuard drop what the page held.
 *
 * Mounted once in every portal as the first child of the SessionProvider:
 * its layout effect runs before the effects of the pages that fetch, and
 * before SWRScopeBoundary reloads the cache for a new account. Draws nothing.
 */
export function SessionAccountBinding() {
  const { data: session, status } = useSession();
  const accountId = hasSessionAccess(session) ? session.user.id : null;

  useLayoutEffect(() => {
    // A first lookup still running is no change of the account.
    if (status === "loading") return;
    bindSessionAccount(accountId);
    confirmSessionSnapshot();
  }, [status, accountId, session]);

  return null;
}
