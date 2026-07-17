// Client API for the info-point display domain (issue #1325).
// Admin CRUD goes through the session-authenticated Next.js proxy routes
// under /api/displays; the public dashboard fetch is token-only.

import type {
  BackendDisplay,
  BackendDisplayCreateResponse,
  InfoDisplay,
} from "~/lib/display-helpers";
import { mapDisplayResponse } from "~/lib/display-helpers";

// ---------------------------------------------------------------------------
// Public dashboard payload (mirrors backend services/display DashboardPayload)
// ---------------------------------------------------------------------------

export interface DashboardRoomOccupancy {
  name: string;
  group_name?: string;
  category_name?: string;
  student_count: number;
  capacity?: number | null;
  is_occupied: boolean;
}

export interface DashboardRunningActivity {
  id: string;
  name: string;
  category: string;
  room_name: string;
  participants: number;
  max_capacity?: number | null;
}

export interface DashboardUpcomingActivity {
  id: string;
  name: string;
  category: string;
  start_time: string; // HH:MM
  room_name: string;
}

export interface DashboardPickupBucket {
  time: string; // HH:MM
  count: number;
}

export interface DashboardPayload {
  status: "active" | "inactive";
  school_name?: string;
  display_name?: string;
  server_time?: string;
  date?: string; // YYYY-MM-DD
  room_occupancy?: DashboardRoomOccupancy[];
  running_activities?: DashboardRunningActivity[];
  upcoming_activities?: DashboardUpcomingActivity[];
  pickup_times?: DashboardPickupBucket[];
  totals?: {
    students_present: number;
    rooms_occupied: number;
    activities_running: number;
  };
}

export class DisplayDashboardError extends Error {
  constructor(public readonly status: number) {
    super(`display dashboard request failed with status ${status}`);
    this.name = "DisplayDashboardError";
  }
}

/**
 * Fetches the public dashboard aggregate. No session required — the token
 * authenticates via the X-Display-Token header (never the URL, so it stays
 * out of request-path logs). Throws DisplayDashboardError on non-OK
 * responses (404 = unknown/revoked token).
 */
export async function fetchDisplayDashboard(
  token: string,
  signal?: AbortSignal,
): Promise<DashboardPayload> {
  const response = await fetch("/api/display/dashboard", {
    cache: "no-store",
    signal,
    headers: { "X-Display-Token": token },
  });
  if (!response.ok) {
    throw new DisplayDashboardError(response.status);
  }
  return (await response.json()) as DashboardPayload;
}

// ---------------------------------------------------------------------------
// Admin CRUD (session-authenticated)
// ---------------------------------------------------------------------------

interface ApiEnvelope<T> {
  status?: string;
  data?: T;
}

async function parseEnvelope<T>(response: Response): Promise<T> {
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as {
      error?: string;
    };
    throw new Error(body.error ?? `request failed (${response.status})`);
  }
  const envelope = (await response.json()) as ApiEnvelope<T>;
  return (envelope.data ?? envelope) as T;
}

export async function listDisplays(): Promise<InfoDisplay[]> {
  const response = await fetch("/api/displays", { cache: "no-store" });
  const displays = await parseEnvelope<BackendDisplay[]>(response);
  // parseEnvelope falls back to the envelope object when `data` is absent
  // (empty list responses) — only a real array may be mapped.
  return (Array.isArray(displays) ? displays : []).map(mapDisplayResponse);
}

export interface DisplayWithToken {
  display: InfoDisplay;
  token: string;
}

export async function createDisplay(name: string): Promise<DisplayWithToken> {
  const response = await fetch("/api/displays", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  const data = await parseEnvelope<BackendDisplayCreateResponse>(response);
  return { display: mapDisplayResponse(data.display), token: data.token };
}

export async function updateDisplay(
  id: string,
  changes: { name?: string; isActive?: boolean },
): Promise<InfoDisplay> {
  const response = await fetch(`/api/displays/${encodeURIComponent(id)}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      ...(changes.name !== undefined ? { name: changes.name } : {}),
      ...(changes.isActive !== undefined
        ? { is_active: changes.isActive }
        : {}),
    }),
  });
  const data = await parseEnvelope<{ display: BackendDisplay }>(response);
  return mapDisplayResponse(data.display);
}

export async function regenerateDisplayToken(id: string): Promise<string> {
  const response = await fetch(
    `/api/displays/${encodeURIComponent(id)}/regenerate`,
    { method: "POST" },
  );
  const data = await parseEnvelope<{ token: string }>(response);
  return data.token;
}

export async function deleteDisplay(id: string): Promise<void> {
  const response = await fetch(`/api/displays/${encodeURIComponent(id)}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => ({}))) as {
      error?: string;
    };
    throw new Error(body.error ?? `delete failed (${response.status})`);
  }
}
