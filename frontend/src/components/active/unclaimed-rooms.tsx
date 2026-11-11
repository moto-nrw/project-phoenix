"use client";

import {
  useState,
  useEffect,
  useCallback,
  useLayoutEffect,
  useRef,
} from "react";
import { activeService } from "~/lib/active-api";
import { userContextService } from "~/lib/usercontext-api";
import type { Supervisor } from "~/lib/active-helpers";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { SpinnerIcon } from "~/components/ui/icons";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import { schoolyardConcept } from "~/lib/concepts/schoolyard";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

const logger = createLogger({ component: "UnclaimedRooms" });

// NOTE: Schulhof is now handled as a permanent tab in active-supervisions page.
// This banner is disabled and returns null. Keeping the component for future
// use with other unclaimed rooms if needed.
const SCHULHOF_ROOM_NAME = "Schulhof";
const DISMISSED_KEY = "schulhof-banner-dismissed";

// Feature flag: set to true to re-enable the banner (not recommended while permanent tab exists)
const ENABLE_SCHULHOF_BANNER = false;

/** Minimal interface for groups passed from parent - compatible with both helper types */
interface MinimalActiveGroup {
  id: string;
  room?: { name?: string };
}

interface UnclaimedRoomsProps {
  readonly onClaimed: () => void;
  /** Pre-fetched active groups to avoid duplicate API call */
  readonly activeGroups?: ReadonlyArray<MinimalActiveGroup>;
  /** Current staff ID to check supervisor status without extra API call */
  readonly currentStaffId?: string;
}

interface SchulhofState {
  /** We only need the group ID for API calls */
  activeGroupId: string | null;
  supervisors: Supervisor[];
  isUserSupervisor: boolean;
  loading: boolean;
}

export function UnclaimedRooms({
  onClaimed,
  activeGroups: providedGroups,
  currentStaffId: providedStaffId,
}: UnclaimedRoomsProps) {
  const [state, setState] = useState<SchulhofState>({
    activeGroupId: null,
    supervisors: [],
    isUserSupervisor: false,
    loading: true,
  });
  const [claiming, setClaiming] = useState(false);
  const { show: showError } = useApiErrorDisplay();
  // „Wiederholen“ übernimmt mit dem aktuellen Stand.
  const retryClaimRef = useRef<() => void>(() => undefined);
  const [dismissed, setDismissed] = useState(false);

  // Schulhof is now handled by the permanent tab in active-supervisions page
  // This banner is disabled to avoid duplicate UI
  // Hooks must be called unconditionally, so we return null after them
  const isDisabled = !ENABLE_SCHULHOF_BANNER;

  const loadSchulhofStatus = useCallback(async () => {
    try {
      // Use provided groups if available and non-empty, otherwise fetch all active groups
      // This ensures Schulhof banner shows even when parent has no cached groups
      const allGroups =
        providedGroups && providedGroups.length > 0
          ? providedGroups
          : await activeService.getActiveGroups({ active: true });
      const schulhofGroup = allGroups.find(
        (g) => g.room?.name === SCHULHOF_ROOM_NAME,
      );

      if (!schulhofGroup) {
        setState({
          activeGroupId: null,
          supervisors: [],
          isUserSupervisor: false,
          loading: false,
        });
        return;
      }

      // Fetch supervisors (and staff if not provided) in parallel
      const needsStaffFetch = !providedStaffId;
      const promises: [Promise<Supervisor[]>, Promise<{ id: string } | null>] =
        [
          activeService.getActiveGroupSupervisors(schulhofGroup.id),
          needsStaffFetch
            ? // Bewusst still: ohne Personalprofil gilt niemand als Aufsicht,
              // das Banner bietet die Übernahme dann an.
              userContextService.getCurrentStaff().catch(() => null)
            : Promise.resolve(null),
        ];

      const [supervisorsResult, staffResult] =
        await Promise.allSettled(promises);

      // Extract supervisors (gracefully handle failure)
      let activeSupervisors: Supervisor[] = [];
      if (supervisorsResult.status === "fulfilled") {
        activeSupervisors = supervisorsResult.value.filter((s) => s.isActive);
      } else {
        logger.warn("failed to load supervisors", {
          reason: String(supervisorsResult.reason),
        });
      }

      // Check if current user is a supervisor
      let isUserSupervisor = false;
      const staffId =
        providedStaffId ??
        (staffResult.status === "fulfilled" && staffResult.value
          ? staffResult.value.id
          : null);

      if (staffId) {
        isUserSupervisor = activeSupervisors.some((s) => s.staffId === staffId);
      }

      // If there are no supervisors, reset dismissed state
      if (activeSupervisors.length === 0) {
        setDismissed(false);
        localStorage.removeItem(DISMISSED_KEY);
      }

      setState({
        activeGroupId: schulhofGroup.id,
        supervisors: activeSupervisors,
        isUserSupervisor,
        loading: false,
      });
    } catch (err) {
      // Bewusst still: das Banner ist ein Zusatzangebot; ohne Status bleibt
      // es aus, die Aufsicht übernimmt man weiter über den Schulhof-Reiter.
      logger.error("failed to load schulhof status", {
        error: err instanceof Error ? err.message : String(err),
      });
      setState((prev) => ({ ...prev, loading: false }));
    }
  }, [providedGroups, providedStaffId]);

  // Nach dem Übernehmen lädt das Banner nach; ein Fehler dabei ist kein
  // Fehler der Übernahme (#6652).
  const claimAfterWrite = useRefreshAfterWrite(
    "die Anzeige des Schulhofs",
    loadSchulhofStatus,
  );

  // Load on mount and check dismissed state
  useEffect(() => {
    const wasDismissed = localStorage.getItem(DISMISSED_KEY) === "true";
    setDismissed(wasDismissed);
    void loadSchulhofStatus();
  }, [loadSchulhofStatus]);

  async function handleClaim() {
    if (!state.activeGroupId) return;

    try {
      setClaiming(true);
      await activeService.claimActiveGroup(state.activeGroupId);
      onClaimed();
      // Reload to update supervisor list
      await claimAfterWrite.refresh();
    } catch (err) {
      logger.error("failed to claim schulhof", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, {
        object: "die Schulhof-Aufsicht",
        retry: () => retryClaimRef.current(),
      });
    } finally {
      setClaiming(false);
    }
  }

  function handleDismiss() {
    setDismissed(true);
    localStorage.setItem(DISMISSED_KEY, "true");
  }

  useLayoutEffect(() => {
    retryClaimRef.current = () => void handleClaim();
  });

  // Schulhof banner is disabled - permanent tab handles this now
  if (isDisabled) {
    return null;
  }

  // Don't show if loading
  if (state.loading) {
    return null;
  }

  // Don't show if no active Schulhof group
  if (!state.activeGroupId) {
    return null;
  }

  // Don't show if user is already a supervisor
  if (state.isUserSupervisor) {
    return null;
  }

  // Don't show if dismissed (but only when there ARE supervisors)
  if (dismissed && state.supervisors.length > 0) {
    return null;
  }

  const hasSupervisors = state.supervisors.length > 0;
  const supervisorNames = state.supervisors
    .map((s) => s.staffName ?? "Unbekannt")
    .join(", ");

  return (
    <>
      <LoadErrorAlert error={claimAfterWrite.error} className="mb-4" />
      <Alert
        type="warning"
        announce="off"
        className="mb-4"
        icon={<MotoConceptIcon concept={schoolyardConcept} size={20} />}
        title="Schulhof-Aufsicht verfügbar"
        message={
          hasSupervisors
            ? `Aktuelle Aufsicht: ${supervisorNames}`
            : "Der Schulhof hat derzeit keine Aufsicht."
        }
        // Ein Schließen-X nur, solange jemand den Schulhof beaufsichtigt: ohne
        // Aufsicht bleibt der Hinweis stehen.
        onDismiss={hasSupervisors ? handleDismiss : undefined}
        dismissLabel="Banner schließen"
        action={
          <Button
            type="button"
            variant="primary"
            size="compact"
            onClick={() => void handleClaim()}
            disabled={claiming}
          >
            {claiming ? (
              <span className="flex items-center gap-2">
                <SpinnerIcon />
                Wird übernommen...
              </span>
            ) : (
              "Beaufsichtigen"
            )}
          </Button>
        }
      />
    </>
  );
}
