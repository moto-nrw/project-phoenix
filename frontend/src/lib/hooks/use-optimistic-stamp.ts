"use client";

import { useRef } from "react";
import type { KeyedMutator } from "~/lib/swr";
import {
  useOptimisticAction,
  type OptimisticOutcome,
} from "~/lib/swr/optimistic";
import { timeTrackingKeys } from "~/lib/swr/portal-keys";
import { STAMP_REFUSALS } from "~/lib/everyday-action-refusals";
import { timeTrackingService } from "~/lib/time-tracking-api";
import type { WorkSession } from "~/lib/time-tracking-helpers";

/** Both normal stamps and the reason-dialog retry use the same locked write. */
export function useOptimisticStamp(
  confirmed: WorkSession | null | undefined,
  mutate: KeyedMutator<WorkSession | null>,
  today: string,
) {
  const action = useOptimisticAction(timeTrackingKeys.current(), confirmed);
  const inFlight = useRef(false);
  const stamp = async (
    status: WorkSession["status"] | null,
    reason?: string,
  ): Promise<OptimisticOutcome<WorkSession>> => {
    if (inFlight.current || confirmed === undefined)
      return { status: "discarded" };
    inFlight.current = true;
    const startedAt = Date.now();
    const now = new Date(startedAt).toISOString(); // Instant only, never a calendar date.
    try {
      return await action.run({
        optimistic: (current, { id }) =>
          status === null
            ? current && { ...current, checkOutTime: now }
            : {
                id,
                staffId: current?.staffId ?? "",
                date: today,
                status,
                source: "app",
                checkInTime: now,
                checkOutTime: null,
                breakMinutes: 0,
                notes: "",
                autoCheckedOut: false,
                createdBy: "",
                updatedBy: null,
                createdAt: now,
                updatedAt: now,
              },
        request: () =>
          status === null
            ? reason === undefined
              ? timeTrackingService.checkOut()
              : timeTrackingService.checkOut(reason)
            : reason === undefined
              ? timeTrackingService.checkIn(status)
              : timeTrackingService.checkIn(status, reason),
        commit: (saved) => {
          void mutate(saved, { revalidate: false });
        },
        refusal: {
          codes: STAMP_REFUSALS,
          // Every own time-tracking read needs the refused permission. The
          // config stays readable with time_tracking:manage alone.
          matches: (key) =>
            key.startsWith("time-tracking-") && key !== "time-tracking-config",
        },
      });
    } finally {
      inFlight.current = false;
    }
  };
  return { ...action, stamp };
}
