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

export interface SaasAdminState {
  isSaasAdmin: boolean;
  isLoading: boolean;
}

/**
 * Check if user is a SaaS admin via API
 */
export async function checkSaasAdminStatus(): Promise<boolean> {
  try {
    const response = await fetch("/api/auth/check-saas-admin", {
      method: "GET",
      credentials: "include",
    });

    if (!response.ok) {
      return false;
    }

    const data = (await response.json()) as { isSaasAdmin: boolean };
    return data.isSaasAdmin;
  } catch (error) {
    console.error("Failed to check SaaS admin status:", error);
    return false;
  }
}

/**
 * Determines the best redirect path for a user based on their permissions and supervision state
 * Priority order:
 * 1. SaaS Admins → /console (if on main domain with no tenant)
 * 2. Org Admins → /dashboard
 * 3. Users with groups → /ogs-groups
 * 4. Users actively supervising → /active-supervisions
 * 5. Regular users → /ogs-groups
 */
export function getSmartRedirectPath(
  session: Session | null,
  supervisionState: SupervisionState,
  saasAdminState?: SaasAdminState,
): string {
  // If still loading supervision state, use ogs-groups as fallback
  if (
    supervisionState.isLoadingGroups ||
    supervisionState.isLoadingSupervision
  ) {
    return "/ogs-groups";
  }

  // SaaS admins go to admin dashboard (only on main domain)
  if (saasAdminState?.isSaasAdmin && !saasAdminState.isLoading) {
    return "/console";
  }

  // Org admins always go to dashboard
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
  saasAdminState?: SaasAdminState,
): { redirectPath: string; isReady: boolean } {
  const isReady =
    !supervisionState.isLoadingGroups &&
    !supervisionState.isLoadingSupervision &&
    !saasAdminState?.isLoading;
  const redirectPath = getSmartRedirectPath(
    session,
    supervisionState,
    saasAdminState,
  );

  return {
    redirectPath,
    isReady,
  };
}
