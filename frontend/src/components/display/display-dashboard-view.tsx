"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { createLogger } from "~/lib/logger";
import type { DashboardPayload } from "~/lib/display-api";
import {
  fetchDisplayDashboard,
  DisplayDashboardError,
} from "~/lib/display-api";
import { RoomOccupancyPanel } from "./room-occupancy-panel";
import { ActivitiesPanel } from "./activities-panel";
import { PickupTimesPanel } from "./pickup-times-panel";
import { DisplayClock } from "./display-clock";

const logger = createLogger({ component: "DisplayDashboardView" });

/** Poll cadence for the aggregate endpoint. */
const POLL_INTERVAL_MS = 25_000;
/** Full page reload once a day — TV browsers run for weeks and leak memory. */
const DAILY_RELOAD_MS = 24 * 60 * 60 * 1000;

type ViewState =
  | { kind: "loading" }
  | { kind: "invalid" } // unknown/revoked token (404)
  | { kind: "inactive" } // known display, deactivated by the school
  | { kind: "active"; payload: DashboardPayload };

interface DisplayDashboardViewProps {
  readonly token: string;
}

export function DisplayDashboardView({ token }: DisplayDashboardViewProps) {
  const [state, setState] = useState<ViewState>({ kind: "loading" });
  const [lastSuccessAt, setLastSuccessAt] = useState<Date | null>(null);
  const [staleMinutes, setStaleMinutes] = useState(0);
  const lastSuccessRef = useRef<Date | null>(null);

  const poll = useCallback(
    async (signal: AbortSignal) => {
      try {
        const payload = await fetchDisplayDashboard(token, signal);
        if (signal.aborted) return;
        if (payload.status === "inactive") {
          setState({ kind: "inactive" });
          return;
        }
        const now = new Date();
        lastSuccessRef.current = now;
        setLastSuccessAt(now);
        setStaleMinutes(0);
        setState({ kind: "active", payload });
      } catch (error) {
        if (signal.aborted) return;
        if (error instanceof DisplayDashboardError && error.status === 404) {
          setState({ kind: "invalid" });
          return;
        }
        // Transient failure: keep the last good data on screen; the stale
        // banner (driven by staleMinutes) tells viewers how old it is.
        logger.warn("display_dashboard_poll_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [token],
  );

  // Polling loop + daily self-reload.
  useEffect(() => {
    const controller = new AbortController();
    void poll(controller.signal);
    const pollTimer = setInterval(() => {
      void poll(controller.signal);
    }, POLL_INTERVAL_MS);
    const reloadTimer = setTimeout(() => {
      window.location.reload();
    }, DAILY_RELOAD_MS);
    return () => {
      controller.abort();
      clearInterval(pollTimer);
      clearTimeout(reloadTimer);
    };
  }, [poll]);

  // Staleness ticker (once a minute is plenty).
  useEffect(() => {
    const timer = setInterval(() => {
      const last = lastSuccessRef.current;
      if (!last) return;
      setStaleMinutes(Math.floor((Date.now() - last.getTime()) / 60_000));
    }, 30_000);
    return () => clearInterval(timer);
  }, []);

  if (state.kind === "loading") {
    return (
      <DisplayShell>
        <p className="text-3xl text-gray-400">Dashboard wird geladen …</p>
      </DisplayShell>
    );
  }

  if (state.kind === "invalid" || state.kind === "inactive") {
    return <DisplayInvalidScreen />;
  }

  const { payload } = state;
  const isStale = staleMinutes >= 2;

  return (
    <div className="min-h-screen bg-gray-50 p-6 lg:p-10">
      <header className="mb-8 flex flex-wrap items-end justify-between gap-6">
        <div>
          <h1 className="text-4xl font-bold text-gray-900 lg:text-6xl">
            {payload.school_name}
          </h1>
          <p className="mt-2 text-2xl text-gray-500 lg:text-3xl">
            {payload.display_name}
          </p>
        </div>
        <DisplayClock />
      </header>

      {isStale && lastSuccessAt && (
        <div className="mb-6 rounded-2xl border border-[#F78C10] bg-[#F78C10]/10 px-6 py-4 text-2xl text-[#F78C10]">
          Keine Verbindung — Stand: vor {staleMinutes}{" "}
          {staleMinutes === 1 ? "Minute" : "Minuten"}
        </div>
      )}

      <main className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <div className="xl:col-span-2">
          <RoomOccupancyPanel
            rooms={payload.room_occupancy ?? []}
            totals={payload.totals}
          />
        </div>
        <div className="flex flex-col gap-6">
          <ActivitiesPanel
            running={payload.running_activities ?? []}
            upcoming={payload.upcoming_activities ?? []}
          />
          <PickupTimesPanel buckets={payload.pickup_times ?? []} />
        </div>
      </main>
    </div>
  );
}

function DisplayShell({ children }: { readonly children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-gray-50 p-10">
      {children}
    </div>
  );
}

/**
 * Full-screen "deactivated / invalid link" state. Also rendered by the display
 * page itself when the URL carries no token fragment.
 */
export function DisplayInvalidScreen() {
  return (
    <DisplayShell>
      <div className="text-center">
        <p className="text-5xl font-bold text-gray-700">Display deaktiviert</p>
        <p className="mt-6 text-2xl text-gray-500">
          Dieses Info-Display wurde deaktiviert oder der Link ist nicht mehr
          gültig. Bitte wende dich an die OGS-Leitung.
        </p>
      </div>
    </DisplayShell>
  );
}
