"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import type { FormError } from "~/components/ui/form-error";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError, wireErrorCode } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { activeService } from "~/lib/active-api";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  SCHULHOF_ROOM_NAME,
  type SchulhofStatusResponse,
} from "~/components/active-supervisions/view-model";

const logger = createLogger({ component: "ActiveSupervisionsPage" });

interface SchulhofActionsOptions {
  readonly schulhofStatus: SchulhofStatusResponse | null;
  readonly currentStaffId: string | undefined;
  /**
   * The Schulhof session the caller supervises, when they do. Named
   * explicitly rather than read off the selected room: a released room can
   * hold several sessions, so "the room on screen" no longer identifies one
   * supervision (#3065).
   */
  readonly supervisedActiveGroupId: string | null;
  readonly spontaneousStartBlockedReason?: string;
  readonly refresh: () => void;
}

export interface SchulhofActions {
  readonly showReleaseModal: boolean;
  /** A failed release; stays in the open release dialog. */
  readonly releaseError: FormError | null;
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
  const {
    schulhofStatus,
    currentStaffId,
    supervisedActiveGroupId,
    spontaneousStartBlockedReason,
    refresh,
  } = options;
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const releaseErrors = useApiFormError();
  const { clear: clearReleaseError, show: showReleaseError } = releaseErrors;
  // „Wiederholen“ gibt mit dem aktuellen Stand ab.
  const retryReleaseRef = useRef<() => void>(() => undefined);

  const [showReleaseModal, setShowReleaseModalState] = useState(false);
  const setShowReleaseModal = useCallback(
    (open: boolean) => {
      clearReleaseError();
      setShowReleaseModalState(open);
    },
    [clearReleaseError],
  );
  const [isReleasingSupervision, setIsReleasingSupervision] = useState(false);
  const [isTogglingSchulhof, setIsTogglingSchulhof] = useState(false);

  // Ref to always have latest schulhofStatus (prevents stale closure in callbacks)
  const schulhofStatusRef = useLatest(schulhofStatus);

  // Handle releasing Schulhof supervision
  const handleReleaseSupervision = useCallback(async () => {
    if (!supervisedActiveGroupId || !currentStaffId) return;

    clearReleaseError();
    try {
      setIsReleasingSupervision(true);

      // Get all supervisors for this active group
      const supervisors = await activeService.getActiveGroupSupervisors(
        supervisedActiveGroupId,
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
      // Der Fehler bleibt im offenen Dialog „Aufsicht abgeben“.
      void showReleaseError(err, {
        object: "das Abgeben der Schulhof-Aufsicht",
        retry: () => retryReleaseRef.current(),
      });
    } finally {
      setIsReleasingSupervision(false);
    }
  }, [
    supervisedActiveGroupId,
    currentStaffId,
    refresh,
    setShowReleaseModal,
    clearReleaseError,
    showReleaseError,
  ]);
  useLayoutEffect(() => {
    retryReleaseRef.current = () => void handleReleaseSupervision();
  });

  // Start a fresh Schulhof session via the generic spontaneous flow (#2161).
  // A "room is already occupied" conflict means another session won the race
  // between status fetch and start — join that session instead of failing.
  const startSchulhofSpontaneously = useCallback(async () => {
    const schulhofState = schulhofStatusRef.current;
    if (!schulhofState?.roomId) {
      throw new Error("Schulhof room is not provisioned");
    }
    if (!currentStaffId) {
      // Same code as the backend's refusal, so the catalog names the reason.
      throw new ApiError(
        "no staff profile for spontaneous Schulhof start",
        403,
        {
          code: "timetable.no_staff_profile",
        },
      );
    }
    try {
      await timetableOperationsApi.createAndStartSpontaneous({
        title: SCHULHOF_ROOM_NAME,
        room_id: Number(schulhofState.roomId),
        activity_group_id: schulhofState.activityGroupId
          ? Number(schulhofState.activityGroupId)
          : undefined,
        staff_ids: [Number(currentStaffId)],
      });
    } catch (err) {
      const occupied =
        err instanceof ApiError &&
        wireErrorCode(err.code) === "timetable.room_occupied";
      if (!occupied) throw err;
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
        if (spontaneousStartBlockedReason) {
          // Kein API-Fehler: der Grund (z. B. Wochenende) steht als Hinweis.
          toast.info(spontaneousStartBlockedReason);
          setIsTogglingSchulhof(false);
          return;
        }
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
      void showActionError(err, {
        object: schulhofStatus.isUserSupervising
          ? "das Abgeben der Schulhof-Aufsicht"
          : "die Schulhof-Aufsicht",
      });
      // Only reset loading state on error - success case handled by useEffect
      setIsTogglingSchulhof(false);
    }
  }, [
    refresh,
    schulhofStatus,
    showActionError,
    spontaneousStartBlockedReason,
    toast,
    startSchulhofSpontaneously,
  ]);

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
    releaseError: releaseErrors.error,
    setShowReleaseModal,
    isReleasingSupervision,
    isTogglingSchulhof,
    handleReleaseSupervision,
    handleToggleSchulhof,
  };
}
