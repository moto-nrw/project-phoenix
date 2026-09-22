"use client";

import useSWR from "swr";
import { useSession } from "next-auth/react";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import {
  fetchSchoolSetup,
  schoolSetupSWRKey,
  type SchoolSetupState,
} from "~/lib/school-setup-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";

const LIVE_REFRESH_MS = 10_000;

/**
 * Der Assistent der angemeldeten Person (#2832). Nur wer die Einstellungen
 * der Schule ändern darf, fragt überhaupt nach; alle anderen sehen ihn nie.
 *
 * Der Stand lädt beim Zurückkehren in den Tab neu: Wer nebenan Räume anlegt,
 * soll den Haken sehen, ohne neu zu laden.
 */
export function useSchoolSetup(
  /** Offene Checkliste: alle 10 Sekunden neu laden, damit Haken zeitnah kommen. */
  live = false,
): {
  state: SchoolSetupState | null;
  accountID: string | null;
  refresh: () => Promise<unknown>;
  replace: (next: SchoolSetupState) => Promise<unknown>;
} {
  const { data: session, status } = useSession();
  const tenantSlug = useTenantSlugSafe();
  const accountID = session?.user?.id ?? null;
  const allowed =
    status === "authenticated" &&
    (isAdmin(session) || hasPermission(session, "config:update"));
  const cacheKey =
    allowed && tenantSlug && accountID
      ? schoolSetupSWRKey(tenantSlug, accountID)
      : null;
  const { data, mutate } = useSWR<SchoolSetupState | null>(
    cacheKey,
    fetchSchoolSetup,
    { revalidateOnFocus: true, refreshInterval: live ? LIVE_REFRESH_MS : 0 },
  );

  return {
    state: data ?? null,
    accountID,
    refresh: () => mutate(),
    replace: (next) => mutate(next, { revalidate: false }),
  };
}
