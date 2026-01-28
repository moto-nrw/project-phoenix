"use client";

import { useState, useEffect, useCallback } from "react";
import { activeService } from "~/lib/active-api";
import { userContextService } from "~/lib/usercontext-api";
import type { Supervisor } from "~/lib/active-helpers";

// Only show claiming UI for Schulhof room
const SCHULHOF_ROOM_NAME = "Schulhof";
const DISMISSED_KEY = "schulhof-banner-dismissed";

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
        console.warn(
          "[SchulhofBanner] Failed to load supervisors:",
          supervisorsResult.reason,
        );
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
      console.error("[SchulhofBanner] Failed to load status:", err);
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
      console.error("Failed to claim Schulhof:", errorMessage);
      setError("Fehler beim Übernehmen der Schulhof-Aufsicht.");
    } finally {
      setClaiming(false);
    }
  }

  function handleDismiss() {
    setDismissed(true);
    localStorage.setItem(DISMISSED_KEY, "true");
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
      <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-3">
        <p className="text-sm text-red-700">{error}</p>
      </div>
    );
  }

  const hasSupervisors = state.supervisors.length > 0;
  const supervisorNames = state.supervisors
    .map((s) => s.staffName ?? "Unbekannt")
    .join(", ");

  return (
    <div className="relative mb-4 flex flex-col gap-3 rounded-xl border border-amber-200 bg-gradient-to-r from-amber-50 to-yellow-50 px-4 py-3 shadow-sm sm:flex-row sm:items-center sm:justify-between">
      {/* Dismiss badge - positioned top right corner */}
      {hasSupervisors && (
        <button
          onClick={handleDismiss}
          className="absolute -top-2 -right-2 z-10 flex h-6 w-6 items-center justify-center rounded-full bg-gray-100 text-gray-500 shadow-sm ring-2 ring-white transition-colors hover:bg-gray-200 hover:text-gray-700"
          aria-label="Banner schließen"
        >
          <svg
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M6 18L18 6M6 6l12 12"
            />
          </svg>
        </button>
      )}

      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-full bg-amber-100">
          <svg
            className="h-5 w-5 text-amber-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 12a9 9 0 11-18 0 9 9 0 0118 0zM12 12a8 8 0 008 4M7.5 13.5a12 12 0 008.5 6.5M12 12a8 8 0 00-7.464 4.928M12.951 7.353a12 12 0 00-9.88 4.111M12 12a8 8 0 00-.536-8.928M15.549 15.147a12 12 0 001.38-10.611"
            />
          </svg>
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
        onClick={() => void handleClaim()}
        disabled={claiming}
        className={`rounded-lg px-3.5 py-1.5 text-sm font-medium transition-all duration-200 ${
          claiming
            ? "cursor-not-allowed bg-gray-200 text-gray-500"
            : "bg-gradient-to-br from-amber-400 to-yellow-500 text-white shadow-sm hover:scale-[1.02] hover:shadow-md hover:shadow-amber-400/20 hover:brightness-105 active:scale-95"
        } `}
      >
        {claiming ? (
          <span className="flex items-center gap-2">
            <svg
              className="h-4 w-4 animate-spin"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="opacity-25"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              ></circle>
              <path
                className="opacity-75"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              ></path>
            </svg>
            Wird übernommen...
          </span>
        ) : (
          "Beaufsichtigen"
        )}
      </button>
    </div>
  );
}
