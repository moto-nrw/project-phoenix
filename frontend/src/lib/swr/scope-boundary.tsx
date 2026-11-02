"use client";

import { useEffect, useLayoutEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import {
  onOtherTabOwnerChange,
  personalStorageAccount,
  personalStorageOwner,
} from "~/lib/personal-storage";
import { hasSessionAccess } from "~/lib/session-access";
import { announceSessionChange } from "~/lib/session-cache";
import {
  authGeneration,
  resetSWRScope,
  suspendSWRSession,
  resumeSWRSession,
} from "./scope";

/**
 * Drops the SWR cache at every boundary of the session (#5006): login,
 * logout, another account, school, preview or rights, also when another
 * tab causes it. Draws nothing.
 *
 * - This tab's session changes its auth generation: the cache goes before
 *   any hook of the new generation fetches. A leaf rendered BEFORE the
 *   provider tree, like SessionCachePrimer: its layout effect runs before
 *   the mount revalidation of every SWR hook in the same commit. Hooks of an
 *   unchanged key load again under the new session, unless it is signed out
 *   or bound to another school than `expectedTenantId` (the BFF refuses
 *   those requests; TenantGuard switches back, #5130).
 * - Another tab binds this browser to another account or signs out (the
 *   owner entry of personal-storage.ts): the cache goes at once, before this
 *   tab's SessionProvider knows, and the provider is asked to read the
 *   cookie. Nothing loads until it answered: the cookie already holds the
 *   other session. PersonalStorageGuard then leaves the page.
 *
 * Another tab only removing the owner is a logout or a school switch. A
 * school switch binds the new owner on the other school's host, whose
 * storage this tab never sees: a fresh lookup that shows this tab's account
 * at another school than the page's confirms it. Readers load again and meet
 * the mismatch path of #5130 (the BFF refuses, the session cache wakes the
 * provider, the new generation resets the cache). A lookup that still shows
 * this school is a logout whose signout has not deleted the cookie yet: its
 * session family is revoked, so a reader loading now would start a competing
 * logout. The tab waits for the lookup without the account.
 *
 * Mounted once in every portal as a child of the SessionProvider.
 */
export function SWRScopeBoundary({
  expectedTenantId,
}: {
  /** School of the page (tenant portal); requests for another are refused. */
  readonly expectedTenantId?: number | null;
}) {
  const { data: session, status } = useSession();
  // undefined while the provider has no answer yet: a lookup is no change.
  const generation = status === "loading" ? undefined : authGeneration(session);
  const accountId = hasSessionAccess(session) ? session.user.id : null;
  const tenantId = session?.user?.tenantId;
  const committed = useRef<{
    generation: string | null;
    accountId: string | null;
    session: typeof session;
  } | null>(null);

  const pending = useRef<{
    session: typeof session;
    accountId: string | null;
    /** The account this tab showed when the other tab changed the owner. */
    shownAccountId: string | null;
    /** School/org portals must confirm their full owner, not just the account. */
    portalOwner?: string | null;
  } | null>(null);

  useLayoutEffect(() => {
    if (generation === undefined) return;
    const previous = committed.current;
    committed.current = { generation, accountId, session };
    const waiting = pending.current;
    if (waiting) {
      // A render is no confirmation; a lookup confirms the expected account
      // or, after a mere owner removal, this tab's account at another school
      // (#5007).
      const confirmed =
        (accountId === waiting.accountId &&
          (waiting.portalOwner === undefined ||
            personalStorageOwner(session?.user) === waiting.portalOwner)) ||
        (waiting.accountId === null &&
          accountId !== null &&
          accountId === waiting.shownAccountId &&
          expectedTenantId != null &&
          tenantId !== expectedTenantId);
      if (session === waiting.session || !confirmed) return;
      pending.current = null;
      // Clear while still paused: old-key subscribers may not have committed
      // their disabled keys yet. The null -> scoped key transition reloads
      // the confirmed session after resuming, including immutable readers.
      resetSWRScope({ revalidate: false });
      resumeSWRSession();
      return;
    } else if (previous === null || previous.generation === generation) return;
    resetSWRScope({
      revalidate:
        generation !== null &&
        (expectedTenantId == null || tenantId === expectedTenantId),
    });
  }, [generation, accountId, tenantId, expectedTenantId, session]);

  useEffect(
    () =>
      onOtherTabOwnerChange((owner) => {
        const shown = committed.current;
        if (shown === null) return;
        const nextAccount = personalStorageAccount(owner);
        const portalOwnerChanged =
          ["org", "school"].includes(shown.session?.user.scope ?? "") &&
          owner !== personalStorageOwner(shown.session?.user);
        if (
          pending.current === null &&
          nextAccount === shown.accountId &&
          !portalOwnerChanged
        )
          return;
        pending.current = {
          session: shown.session,
          accountId: nextAccount,
          shownAccountId: shown.accountId,
          portalOwner: ["org", "school"].includes(
            shown.session?.user.scope ?? "",
          )
            ? owner
            : undefined,
        };
        suspendSWRSession();
        resetSWRScope({ revalidate: false });
        announceSessionChange();
      }),
    [],
  );

  return null;
}
