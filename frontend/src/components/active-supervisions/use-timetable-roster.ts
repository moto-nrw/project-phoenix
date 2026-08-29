"use client";

import { useState } from "react";
import { useSWRAuth } from "~/lib/swr";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { TimetableRoster } from "~/lib/timetable-operations-types";
import { activeSupervisionRosterKey } from "~/components/active-supervisions/view-model";

export interface TimetableRosterState {
  /** The roster iff it belongs to the current selection, else null. */
  readonly currentTimetableRoster: TimetableRoster | null;
  readonly activeTimetableInstanceId: string | null;
  readonly isWaitingForTimetableRoster: boolean;
  readonly mutateRoster: (
    data?: TimetableRoster | null,
    opts?: { revalidate?: boolean },
  ) => Promise<unknown>;
}

/**
 * Fetches the Web-Anwesenheit roster of the selected session — by explicit
 * timetable instance when one was just started/reopened, else by the
 * selected active group. A 404 (no timetable session behind the group) is
 * memoized per group so the page falls back to the visit grid without
 * re-asking every render.
 */
export function useTimetableRoster(options: {
  readonly selectedTimetableInstanceId: string | null;
  readonly currentRoomId: string | null | undefined;
}): TimetableRosterState {
  const { selectedTimetableInstanceId, currentRoomId } = options;

  const [missingRosterActiveGroupIds, setMissingRosterActiveGroupIds] =
    useState<Set<string>>(() => new Set());

  const timetableRosterKey = activeSupervisionRosterKey({
    selectedTimetableInstanceId,
    currentRoomId,
    missingRosterActiveGroupIds,
  });
  const {
    data: timetableRoster,
    isLoading: isTimetableRosterLoading,
    mutate: mutateRoster,
  } = useSWRAuth<TimetableRoster | null>(
    timetableRosterKey,
    async () => {
      try {
        if (selectedTimetableInstanceId) {
          return await timetableOperationsApi.roster(
            selectedTimetableInstanceId,
          );
        }
        if (!currentRoomId) return null;
        return await timetableOperationsApi.rosterByActiveGroup(currentRoomId);
      } catch (err) {
        if (
          err instanceof Error &&
          (err.message.includes("404") ||
            err.message.toLowerCase().includes("not found"))
        ) {
          if (!selectedTimetableInstanceId && currentRoomId) {
            setMissingRosterActiveGroupIds((current) => {
              if (current.has(currentRoomId)) return current;
              const next = new Set(current);
              next.add(currentRoomId);
              return next;
            });
          }
          return null;
        }
        throw err;
      }
    },
    { keepPreviousData: false, revalidateOnFocus: false },
  );

  const timetableRosterMatchesSelection =
    timetableRoster !== undefined &&
    timetableRoster !== null &&
    (selectedTimetableInstanceId
      ? timetableRoster.instance.id === selectedTimetableInstanceId
      : !!currentRoomId &&
        timetableRoster.instance.activeGroupId === currentRoomId);
  const currentTimetableRoster = timetableRosterMatchesSelection
    ? timetableRoster
    : null;
  const isWaitingForTimetableRoster =
    timetableRosterKey !== null &&
    (timetableRoster === undefined ||
      (timetableRoster !== null && !timetableRosterMatchesSelection)) &&
    isTimetableRosterLoading;

  return {
    currentTimetableRoster,
    activeTimetableInstanceId: currentTimetableRoster?.instance?.id ?? null,
    isWaitingForTimetableRoster,
    mutateRoster,
  };
}
