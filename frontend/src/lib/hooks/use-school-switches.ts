"use client";

import { useUserContext } from "~/lib/hooks/use-user-context";
import type {
  ParentInviteMode,
  SchoolSwitches,
} from "~/lib/user-context-types";

/**
 * School settings the signed-in app shell reads outside the settings page
 * (#4996), from the navigation context the tenant layout preloads. Only the
 * settings page loads the whole settings schema.
 *
 * `enabled = false` skips the request, for surfaces that render outside a
 * signed-in school session (the operator's navigation).
 */
function useSchoolSwitches(enabled = true): {
  switches: SchoolSwitches | undefined;
  isLoading: boolean;
  error: Error | undefined;
} {
  const { userContext, isLoading, error } = useUserContext(enabled);
  return {
    switches: userContext?.schoolSwitches ?? undefined,
    isLoading,
    error: userContext?.schoolSwitches ? undefined : error,
  };
}

/**
 * Whether the school writes parent announcements
 * (operations.parent_news_enabled). Fails closed: while the context loads or
 * when it failed, the entry points stay hidden.
 */
export function useSchoolParentNewsEnabled(enabled = true): boolean {
  return useSchoolSwitches(enabled).switches?.parentNewsEnabled === true;
}

/** The Betreuungsplan's day as "HH:MM"; undefined until the context is there. */
export function useSchoolDayTimes(): {
  start: string | undefined;
  end: string | undefined;
} {
  const { switches } = useSchoolSwitches();
  return {
    start: switches?.timetableDayStartTime,
    end: switches?.timetableDayEndTime,
  };
}

/** guardians.parent_invite_mode with the state of its request. */
export function useSchoolParentInviteMode(enabled = true): {
  mode: ParentInviteMode | undefined;
  isLoading: boolean;
  error: Error | undefined;
} {
  const { switches, isLoading, error } = useSchoolSwitches(enabled);
  return { mode: switches?.parentInviteMode, isLoading, error };
}
