"use client";

import { useState, useEffect, useCallback } from "react";
import { X } from "lucide-react";
import { activeService } from "~/lib/active-api";
import { userContextService } from "~/lib/usercontext-api";
import type { Supervisor } from "~/lib/active-helpers";
import { createLogger } from "~/lib/logger";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SpinnerIcon } from "~/components/ui/icons";

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
  const [error, setError] = useState<string | null>(null);
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
            ? userContextService.getCurrentStaff().catch(() => null)
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
      logger.error("failed to load schulhof status", {
        error: err instanceof Error ? err.message : String(err),
      });
      setState((prev) => ({ ...prev, loading: false }));
    }
  }, [providedGroups, providedStaffId]);

  // Load on mount and check dismissed state
  useEffect(() => {
    const wasDismissed = localStorage.getItem(DISMISSED_KEY) === "true";
    setDismissed(wasDismissed);
    loadSchulhofStatus().catch(() => undefined);
  }, [loadSchulhofStatus]);

  async function handleClaim() {
    if (!state.activeGroupId) return;

    try {
      setClaiming(true);
      await activeService.claimActiveGroup(state.activeGroupId);
      setError(null);
      onClaimed();
      // Reload to update supervisor list
      await loadSchulhofStatus();
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : "Unknown error";
      logger.error("failed to claim schulhof", { error: errorMessage });
      setError("Fehler beim Übernehmen der Schulhof-Aufsicht.");
    } finally {
      setClaiming(false);
    }
  }

  function handleDismiss() {
    setDismissed(true);
    localStorage.setItem(DISMISSED_KEY, "true");
  }

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

  if (error) {
    return (
      <div className="border-moto-red/30 bg-moto-red/10 mb-4 rounded-lg border p-3">
        <p className="text-moto-red-hover text-sm">{error}</p>
      </div>
    );
  }

  const hasSupervisors = state.supervisors.length > 0;
  const supervisorNames = state.supervisors
    .map((s) => s.staffName ?? "Unbekannt")
    .join(", ");

  return (
    <div className="border-moto-amber/30 bg-moto-amber/10 relative mb-4 flex flex-col gap-3 rounded-xl border px-4 py-3 shadow-sm sm:flex-row sm:items-center sm:justify-between">
      {/* Dismiss badge - positioned top right corner */}
      {hasSupervisors && (
        <button
          type="button"
          onClick={handleDismiss}
          className="absolute -top-2 -right-2 z-10 flex h-6 w-6 items-center justify-center rounded-full bg-gray-100 text-gray-500 shadow-sm ring-2 ring-white transition-colors hover:bg-gray-200 hover:text-gray-700"
          aria-label="Banner schließen"
        >
          <X className="h-3.5 w-3.5" strokeWidth={2.5} aria-hidden="true" />
        </button>
      )}

      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-xl bg-gray-100">
          <MotoConceptIcon concept="schoolyard" size={20} />
        </div>
        <div className="pr-6">
          <h3 className="font-semibold text-gray-900">
            Schulhof-Aufsicht verfügbar
          </h3>
          <p className="text-sm text-gray-600">
            {hasSupervisors
              ? `Aktuelle Aufsicht: ${supervisorNames}`
              : "Der Schulhof hat derzeit keine Aufsicht."}
          </p>
        </div>
      </div>

      <button
        type="button"
        onClick={() => void handleClaim()}
        disabled={claiming}
        className={`rounded-lg px-3.5 py-1.5 text-sm font-medium transition-[background-color,box-shadow] duration-200 ${
          claiming
            ? "cursor-not-allowed bg-gray-200 text-gray-500"
            : "bg-gray-900 text-white shadow-sm hover:bg-gray-800 hover:shadow-md"
        } `}
      >
        {claiming ? (
          <span className="flex items-center gap-2">
            <SpinnerIcon />
            Wird übernommen...
          </span>
        ) : (
          "Beaufsichtigen"
        )}
      </button>
    </div>
  );
}
