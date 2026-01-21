/**
 * Utility for determining smart redirect paths based on user permissions and supervision state
 */

import type { BetterAuthSession } from "~/lib/auth-utils";
import { isAdmin } from "~/lib/auth-utils";

type Session = BetterAuthSession;

export interface SupervisionState {
  hasGroups: boolean;
  isLoadingGroups: boolean;
  isSupervising: boolean;
  isLoadingSupervision: boolean;
}

/**
 * Determines the best redirect path for a user based on their permissions and supervision state
 * Priority order:
 * 1. Admins → /dashboard
 * 2. Users with groups → /ogs-groups
 * 3. Users actively supervising → /active-supervisions
 * 4. Regular users → /ogs-groups
 */
export function getSmartRedirectPath(
  session: Session | null,
  supervisionState: SupervisionState,
): string {
  // If still loading supervision state, use ogs-groups as fallback
  if (
    supervisionState.isLoadingGroups ||
    supervisionState.isLoadingSupervision
  ) {
    return "/ogs-groups";
  }

  // Admins always go to dashboard
  if (isAdmin(session)) {
    return "/dashboard";
  }

  // Users with groups go to their groups page
  if (supervisionState.hasGroups) {
    return "/ogs-groups";
  }

  // Users actively supervising a room go to room page
  if (supervisionState.isSupervising) {
    return "/active-supervisions";
  }

  // Regular users default to ogs-groups (shows empty state on page)
  return "/ogs-groups";
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
