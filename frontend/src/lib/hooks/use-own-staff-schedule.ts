"use client";

import { me as meKeys } from "~/lib/swr/me";
import { useCallback } from "react";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { staffScheduleService, type StaffSchedule } from "~/lib/staff-api";
import { useSWRAuth } from "~/lib/swr";

export interface OwnStaffSchedule {
  /** The caller's staff ID, or null while unknown or without a staff record. */
  readonly ownStaffId: string | null;
  readonly schedule: StaffSchedule | undefined;
  /** Failed load of the own staff record or of its work schedule. */
  readonly error: unknown;
  /** Revalidates the staff record and the work schedule after a failure. */
  readonly retry: () => void;
}

/**
 * The caller's staff ID and work schedule for the Zeiterfassung (#4992).
 *
 * The staff ID comes from the user context the tenant shell preloads on the
 * server (`user-context` SWR fallback), so the schedule request starts with
 * the other page data instead of waiting for `GET /api/me/staff`. The backend
 * still decides whose schedule the caller may read.
 *
 * Only when the context names no staff record (an account without one, or a
 * context that failed to load) does the page ask `/api/me/staff` as before,
 * so that case keeps its existing result: no ID and the same load error.
 */
export function useOwnStaffSchedule(): OwnStaffSchedule {
  const { userContext, isReady: contextReady } = useUserContext();
  const contextStaffId = userContext?.currentStaff?.id ?? null;

  const {
    data: fallbackStaff,
    error: staffError,
    mutate: mutateStaff,
  } = useSWRAuth(
    contextReady && !contextStaffId ? meKeys.key() : null,
    meKeys.fetcher,
    { revalidateOnFocus: false },
  );
  const ownStaffId = contextStaffId ?? fallbackStaff?.id ?? null;

  const {
    data: schedule,
    error: scheduleError,
    mutate: mutateSchedule,
  } = useSWRAuth(
    ownStaffId ? `time-tracking-own-schedule-${ownStaffId}` : null,
    () => staffScheduleService.getSchedule(ownStaffId as string),
    { revalidateOnFocus: false },
  );

  const retry = useCallback(() => {
    void mutateStaff();
    void mutateSchedule();
  }, [mutateStaff, mutateSchedule]);

  return {
    ownStaffId,
    schedule,
    error: staffError ?? scheduleError,
    retry,
  };
}
