// Abruf des Raumplans (#4768): GET /api/timetable/instances mit `room_id`,
// abgebildet auf die wenigen Felder des Reiters. Eigener kleiner Client statt
// `timetableService.getWeek`, weil dessen Abbildung die Planner-Typen und
// -Helfer mitzieht.

import { apiErrorFromBody, transportFetch } from "~/lib/api-error";
import type { RoomPlanBlock } from "~/lib/room-plan";

interface BackendRoomPlanInstance {
  readonly id: number | string;
  readonly date: string;
  readonly start_time: string;
  readonly end_time: string;
  readonly title: string;
  readonly status: string;
  readonly group_name?: string;
  readonly cancel_reason?: string | null;
}

interface RoomPlanEnvelope {
  readonly data?: { readonly instances?: BackendRoomPlanInstance[] };
}

/** Die Blöcke des Raums von `from` bis `to` (beide inklusive). */
export async function fetchRoomPlanBlocks(
  roomId: string,
  from: string,
  to: string,
): Promise<RoomPlanBlock[]> {
  const params = new URLSearchParams({ from, to, room_id: roomId });
  const response = await transportFetch(`/api/timetable/instances?${params}`, {
    method: "GET",
    headers: { Accept: "application/json" },
    credentials: "include",
  });
  if (!response.ok) {
    throw apiErrorFromBody(
      `Anfrage fehlgeschlagen (HTTP ${response.status})`,
      response.status,
      await response.json().catch(() => undefined),
    );
  }
  const body = (await response.json()) as RoomPlanEnvelope;
  return (body.data?.instances ?? []).map((raw) => ({
    id: String(raw.id),
    date: raw.date,
    startTime: raw.start_time,
    endTime: raw.end_time,
    title: raw.title,
    status: raw.status,
    ...(raw.group_name ? { groupName: raw.group_name } : {}),
    ...(raw.cancel_reason ? { cancelReason: raw.cancel_reason } : {}),
  }));
}
