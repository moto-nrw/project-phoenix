"use client";

import { useCallback, useEffect, useState } from "react";
import { createLogger } from "~/lib/logger";
import { activeService } from "~/lib/active-api";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  SCHULHOF_ROOM_NAME,
  type ActiveSupervisionRoom,
  type SchulhofStatusResponse,
} from "~/components/active-supervisions/view-model";
import { spontaneousActivityWindow } from "~/components/active-supervisions/spontaneous-window";

const logger = createLogger({ component: "ActiveSupervisionsPage" });

interface SchulhofActionsOptions {
  readonly schulhofStatus: SchulhofStatusResponse | null;
  readonly currentStaffId: string | undefined;
  readonly currentRoom: ActiveSupervisionRoom | null;
  readonly refresh: () => void;
  readonly setError: (message: string | null) => void;
}

export interface SchulhofActions {
  readonly showReleaseModal: boolean;
  readonly setShowReleaseModal: (open: boolean) => void;
  readonly isReleasingSupervision: boolean;
  readonly isTogglingSchulhof: boolean;
  readonly handleReleaseSupervision: () => Promise<void>;
  readonly handleToggleSchulhof: () => Promise<void>;
}

/**
 * Start / claim / release actions of the Schulhof supervision (#2161).
 * Toggling rides on the generic mechanics: claim the open session, start a
 * spontaneous one when the yard is empty, end the own supervision to stop.
 */
export function useSchulhofActions(
  options: SchulhofActionsOptions,
): SchulhofActions {
  const { schulhofStatus, currentStaffId, currentRoom, refresh, setError } =
    options;

  const [showReleaseModal, setShowReleaseModal] = useState(false);
  const [isReleasingSupervision, setIsReleasingSupervision] = useState(false);
  const [isTogglingSchulhof, setIsTogglingSchulhof] = useState(false);

  // Ref to always have latest schulhofStatus (prevents stale closure in callbacks)
  const schulhofStatusRef = useLatest(schulhofStatus);

  // Handle releasing Schulhof supervision
  const handleReleaseSupervision = useCallback(async () => {
    if (!currentRoom || !currentStaffId) return;

    try {
      setIsReleasingSupervision(true);

      // Get all supervisors for this active group
      const supervisors = await activeService.getActiveGroupSupervisors(
        currentRoom.id,
      );

      // Find the supervisor record for the current user (using cached staff ID)
      const mySupervision = supervisors.find(
        (sup) => sup.staffId === currentStaffId && sup.isActive,
      );

      if (mySupervision) {
        await activeService.endSupervision(mySupervision.id);
      } else {
        logger.warn("no active supervision found for current user");
      }

      setShowReleaseModal(false);

      // Refresh the page to show updated state
      refresh();
    } catch (err) {
      logger.error("failed to release Schulhof supervision", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Fehler beim Abgeben der Schulhof-Aufsicht.");
    } finally {
      setIsReleasingSupervision(false);
    }
  }, [currentRoom, currentStaffId, refresh, setError]);

  // Start a fresh Schulhof session via the generic spontaneous flow (#2161).
  // A "room is already occupied" conflict means another session won the race
  // between status fetch and start — join that session instead of failing.
  const startSchulhofSpontaneously = useCallback(async () => {
    const schulhofState = schulhofStatusRef.current;
    if (!schulhofState?.roomId) {
      throw new Error("Schulhof room is not provisioned");
    }
    if (!currentStaffId) {
      throw new Error("no staff profile for spontaneous Schulhof start");
    }
    const window = spontaneousActivityWindow(new Date());
    try {
      await timetableOperationsApi.createAndStartSpontaneous({
        date: window.date,
        start_time: window.startTime,
        end_time: window.endTime,
        title: SCHULHOF_ROOM_NAME,
        room_id: Number(schulhofState.roomId),
        activity_group_id: schulhofState.activityGroupId
          ? Number(schulhofState.activityGroupId)
          : undefined,
        staff_ids: [Number(currentStaffId)],
        student_ids: [],
      });
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (!message.includes("room is already occupied")) throw err;
      const fresh = await activeService.getSchulhofStatus();
      if (!fresh.activeGroupId) throw err;
      await activeService.claimActiveGroup(fresh.activeGroupId);
    }
  }, [currentStaffId, schulhofStatusRef]);

  // Handle toggling Schulhof supervision (start/stop).
  const handleToggleSchulhof = useCallback(async () => {
    if (!schulhofStatus) return;

    try {
      setIsTogglingSchulhof(true);
      if (schulhofStatus.isUserSupervising) {
        if (!schulhofStatus.supervisionId) {
          throw new Error("no supervision id in Schulhof status");
        }
        await activeService.endSupervision(schulhofStatus.supervisionId);
      } else if (schulhofStatus.activeGroupId) {
        await activeService.claimActiveGroup(schulhofStatus.activeGroupId);
      } else {
        await startSchulhofSpontaneously();
      }

      // Refresh to get updated status
      // Note: Don't reset isTogglingSchulhof here - let the useEffect below handle it
      // when schulhofStatus actually updates, to avoid flickering
      refresh();
    } catch (err) {
      logger.error("failed to toggle Schulhof supervision", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        schulhofStatus.isUserSupervising
          ? "Fehler beim Abgeben der Schulhof-Aufsicht."
          : "Fehler beim Übernehmen der Schulhof-Aufsicht.",
      );
      // Only reset loading state on error - success case handled by useEffect
      setIsTogglingSchulhof(false);
    }
  }, [refresh, schulhofStatus, setError, startSchulhofSpontaneously]);

  // Reset toggling state when schulhofStatus updates (prevents flicker after successful toggle)
  // Also includes a timeout fallback to prevent stuck loading state if SWR refresh fails
  useEffect(() => {
    if (isTogglingSchulhof && schulhofStatus) {
      // When SWR has updated the data, reset the loading state
      setIsTogglingSchulhof(false);
    }
    // Only react to schulhofStatus changes, not isTogglingSchulhof
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [schulhofStatus?.isUserSupervising]);

  // Safety timeout: Reset loading state after 5s if SWR refresh doesn't update status
  // This prevents stuck loading state when refresh fails or returns stale data
  useEffect(() => {
    if (!isTogglingSchulhof) return;

    const timeout = setTimeout(() => {
      logger.warn("Schulhof toggle timeout: resetting loading state after 5s");
      setIsTogglingSchulhof(false);
    }, 5000);

    return () => clearTimeout(timeout);
  }, [isTogglingSchulhof]);

  return {
    showReleaseModal,
    setShowReleaseModal,
    isReleasingSupervision,
    isTogglingSchulhof,
    handleReleaseSupervision,
    handleToggleSchulhof,
  };
}
