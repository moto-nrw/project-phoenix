"use client";

import { staffOnboardingKeys } from "~/lib/swr/portal-keys";
import { useSWRAuth } from "~/lib/swr";
import { useSession } from "next-auth/react";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import {
  fetchStaffOnboarding,
  type StaffOnboardingState,
} from "~/lib/staff-onboarding-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";

/**
 * Die ersten Schritte der angemeldeten Person (#3748). Wer die Einstellungen
 * der Schule ändern darf, hat stattdessen die Einrichtung der Schule
 * (#2832): So stehen nie beide Checklisten übereinander.
 */
export function useStaffOnboarding(): {
  state: StaffOnboardingState | null;
  accountID: string | null;
  replace: (next: StaffOnboardingState) => Promise<unknown>;
} {
  const { data: session, status } = useSession();
  const tenantSlug = useTenantSlugSafe();
  const accountID = session?.user?.id ?? null;
  const allowed =
    status === "authenticated" &&
    !isAdmin(session) &&
    !hasPermission(session, "config:update");
  const cacheKey =
    allowed && tenantSlug && accountID ? staffOnboardingKeys.key() : null;
  const { data, mutate } = useSWRAuth<StaffOnboardingState | null>(
    cacheKey,
    fetchStaffOnboarding,
    // Der Stand ändert sich nur durch die Person selbst; die Schule wird
    // beim Zurückkehren in den Tab bereit.
    { revalidateOnFocus: true },
  );

  return {
    state: data ?? null,
    accountID,
    replace: (next) => mutate(next, { revalidate: false }),
  };
}
