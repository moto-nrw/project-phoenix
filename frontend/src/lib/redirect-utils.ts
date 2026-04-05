/**
 * Utility for determining smart redirect paths based on user permissions and supervision state
 */

import type { Session } from "next-auth";
import { hasRole, isCaregiver } from "~/lib/auth-utils";

export interface SupervisionState {
  hasGroups: boolean;
  isLoadingGroups: boolean;
  isSupervising: boolean;
  isLoadingSupervision: boolean;
}

/**
 * Determines the best redirect path for a user based on their permissions and supervision state
 * Priority order:
 * 1. Caregivers with groups → /ogs-groups
 * 2. Caregivers actively supervising → /active-supervisions
 * 3. Other caregivers → /ogs-groups
 * 4. Admin-only users → /dashboard
 */
export function getSmartRedirectPath(
  session: Session | null,
  supervisionState: SupervisionState,
): string {
  const canUseCaregiverFlows = isCaregiver(session);
  const canUseAdminFlows = hasRole(session, "admin");

  if (canUseCaregiverFlows) {
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

  // If still loading supervision state, use ogs-groups as fallback
  if (
    supervisionState.isLoadingGroups ||
    supervisionState.isLoadingSupervision
  ) {
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
): { redirectPath: string; isReady: boolean } {
  const isReady =
    !supervisionState.isLoadingGroups && !supervisionState.isLoadingSupervision;
  const redirectPath = getSmartRedirectPath(session, supervisionState);

  return {
    redirectPath,
    isReady,
  };
}
