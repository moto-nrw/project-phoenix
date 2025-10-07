"use client";

import { useState, useEffect } from "react";
import { activeService } from "~/lib/active-api";
import type { ActiveGroup } from "~/lib/active-helpers";

interface UnclaimedRoomsProps {
  onClaimed: () => void;
}

export function UnclaimedRooms({ onClaimed }: UnclaimedRoomsProps) {
  const [unclaimedGroups, setUnclaimedGroups] = useState<ActiveGroup[]>([]);
  const [loading, setLoading] = useState(true);
  const [claiming, setClaiming] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    void loadUnclaimedGroups();
  }, []);

  async function loadUnclaimedGroups() {
    try {
      setLoading(true);
      console.log("[UnclaimedRooms] Fetching unclaimed groups...");
      const groups = await activeService.getUnclaimedGroups();
      console.log("[UnclaimedRooms] Got groups:", groups.length, groups);
      setUnclaimedGroups(groups);
      setError(null);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : "Unknown error";
      console.error("[UnclaimedRooms] Failed to load unclaimed groups:", errorMessage);
      setError("Fehler beim Laden der verfügbaren Räume");
    } finally {
      setLoading(false);
    }
  }

  async function handleClaim(groupId: string, roomName: string) {
    try {
      setClaiming(groupId);
      await activeService.claimActiveGroup(groupId);

      // Remove from unclaimed list
      setUnclaimedGroups(prev => prev.filter(g => g.id !== groupId));

      // Notify parent to refresh their room list
      onClaimed();

      setError(null);
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : "Unknown error";
      console.error("Failed to claim group:", errorMessage);
      setError(`Fehler beim Beanspruchen von ${roomName}. Bitte versuchen Sie es erneut.`);
    } finally {
      setClaiming(null);
    }
  }

  if (loading) {
    return (
      <div className="mb-6 p-4 bg-gray-50 border-l-4 border-gray-300 rounded animate-pulse">
        <div className="h-6 bg-gray-200 rounded w-1/3 mb-3"></div>
        <div className="h-20 bg-gray-200 rounded"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mb-6 p-4 bg-red-50 border-l-4 border-red-400 rounded">
        <p className="text-red-700">{error}</p>
      </div>
    );
  }

  if (unclaimedGroups.length === 0) {
    return null; // Don't show anything if no unclaimed rooms
  }

  return (
    <div className="mb-6 p-4 bg-yellow-50 border-l-4 border-yellow-400 rounded shadow-sm">
      <div className="flex items-center gap-2 mb-3">
        <svg className="h-5 w-5 text-yellow-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
        </svg>
        <h3 className="text-lg font-semibold text-gray-900">
          Verfügbare Räume ({unclaimedGroups.length})
        </h3>
      </div>

      <p className="text-sm text-gray-600 mb-4">
        Diese Räume haben derzeit keine Aufsicht. Klicken Sie auf &quot;Beanspruchen&quot;, um die Aufsicht zu übernehmen.
      </p>

      <div className="grid gap-3">
        {unclaimedGroups.map(group => {
          const startTime = group.startTime ? new Date(group.startTime) : null;
          const isClaiming = claiming === group.id;
          const groupIdStr = String(group.id);

          return (
            <div
              key={group.id}
              className="flex items-center justify-between bg-white p-4 rounded-lg shadow-sm border border-yellow-200 hover:border-yellow-400 transition-colors"
            >
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-1">
                  <svg className="h-5 w-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                          d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                  </svg>
                  <div className="font-semibold text-gray-900">
                    {group.room?.name ?? `Raum ${group.roomId}`}
                  </div>
                </div>

                <div className="text-sm text-gray-600 ml-7">
                  {group.actualGroup?.name ?? `Gruppe #${group.groupId}`}
                  {startTime && (
                    <span className="ml-2">
                      • Gestartet um {startTime.toLocaleTimeString("de-DE", { hour: "2-digit", minute: "2-digit" })}
                    </span>
                  )}
                </div>
              </div>

              <button
                onClick={() => void handleClaim(groupIdStr, "Raum")}
                disabled={isClaiming}
                className={`
                  px-6 py-2 rounded-lg font-medium transition-all
                  ${isClaiming
                    ? "bg-gray-300 text-gray-500 cursor-not-allowed"
                    : "bg-green-600 text-white hover:bg-green-700 hover:shadow-md active:scale-95"
                  }
                `}
              >
                {isClaiming ? (
                  <span className="flex items-center gap-2">
                    <svg className="animate-spin h-4 w-4" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Beanspruche...
                  </span>
                ) : (
                  "Beanspruchen"
                )}
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}
