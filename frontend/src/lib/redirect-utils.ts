/**
 * Utility for determining smart redirect paths based on user permissions and supervision state
 */

import type { Session } from "next-auth";
import { hasRole, isCaregiver } from "~/lib/auth-utils";
import type { PresenceMode } from "~/lib/tenant-api";

export interface SupervisionState {
  hasGroups: boolean;
  isLoadingGroups: boolean;
  isSupervising: boolean;
  isLoadingSupervision: boolean;
}

/**
 * Determines the best redirect path for a user based on their permissions and supervision state
 * Priority order:
 * 1. Lehrkraft-only accounts: /klassen (their single reachable area, #1772)
 * 2. Binary-mode caregivers: /students/search
 * 3. Open-care caregivers: /students/search (no group concept, #1544)
 * 4. Caregivers with groups: /ogs-groups
 * 5. Caregivers actively supervising: /active-supervisions
 * 6. Other caregivers: /ogs-groups
 * 7. Admin-only users: /dashboard
 */
export function getSmartRedirectPath(
  session: Session | null,
  supervisionState: SupervisionState,
  presenceMode: PresenceMode = "detailed",
  openCareGroupMode = false,
): string {
  const canUseCaregiverFlows = isCaregiver(session);
  const canUseAdminFlows = hasRole(session, "admin");

  // A pure Lehrkraft account (#1772) holds only class_day:read — every other
  // landing page would 403 or render empty. Dual-role accounts (also
  // caregiver or admin) keep their richer flows below.
  if (
    hasRole(session, "lehrkraft") &&
    !canUseCaregiverFlows &&
    !canUseAdminFlows
  ) {
    return "/klassen";
  }

  if (canUseCaregiverFlows) {
    if (presenceMode === "binary" || openCareGroupMode) {
      return "/students/search";
    }

    // If still loading supervision state, use ogs-groups as caregiver fallback
    if (
      supervisionState.isLoadingGroups ||
      supervisionState.isLoadingSupervision
    ) {
      return "/ogs-groups";
    }

    if (supervisionState.hasGroups) {
      return "/ogs-groups";
    }

    if (supervisionState.isSupervising) {
      return "/active-supervisions";
    }

    return "/ogs-groups";
  }

  if (canUseAdminFlows) {
    return "/dashboard";
  }

  return "/dashboard";
}

/**
 * Hook-like function to get the current smart redirect path
 * This is designed to be used after the supervision context has loaded
 */
export function useSmartRedirectPath(
  session: Session | null,
  supervisionState: SupervisionState,
  presenceMode: PresenceMode = "detailed",
  openCareGroupMode = false,
): { redirectPath: string; isReady: boolean } {
  const isReady =
    !supervisionState.isLoadingGroups && !supervisionState.isLoadingSupervision;
  const redirectPath = getSmartRedirectPath(
    session,
    supervisionState,
    presenceMode,
    openCareGroupMode,
  );

  return {
    redirectPath,
    isReady,
  };
}
