"use client";

import { useEffect, useLayoutEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import {
  onOtherTabOwnerChange,
  personalStorageAccount,
} from "~/lib/personal-storage";
import { hasSessionAccess } from "~/lib/session-access";
import { announceSessionChange } from "~/lib/session-cache";
import { authGeneration, resetSWRScope } from "./scope";

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
 * A school change in another tab with the same account is left to the
 * mismatch path of #5130: the BFF refuses this tab's requests, the session
 * cache wakes the provider, and the new generation resets the cache here.
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
  } | null>(null);

  useLayoutEffect(() => {
    if (generation === undefined) return;
    const previous = committed.current;
    committed.current = { generation, accountId };
    if (previous === null || previous.generation === generation) return;
    resetSWRScope({
      revalidate:
        generation !== null &&
        (expectedTenantId == null || tenantId === expectedTenantId),
    });
  }, [generation, accountId, tenantId, expectedTenantId]);

  useEffect(
    () =>
      onOtherTabOwnerChange((owner) => {
        const shown = committed.current;
        if (shown === null) return;
        if (personalStorageAccount(owner) === shown.accountId) return;
        resetSWRScope({ revalidate: false });
        announceSessionChange();
      }),
    [],
  );

  return null;
}
