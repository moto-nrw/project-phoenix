// URL-Helfer des Raumplan-Reiters (#4768). Sie bleiben klein, damit die
// Raumseite den Planinhalt erst beim Öffnen des Reiters laden kann.

import { useTenantAwarePath } from "~/lib/tenant-path";

export type RoomPlanView = "tag" | "woche";

/** Liest `view` der Raumseite; alles außer „tag“ ist die Woche. */
export function parseRoomPlanView(raw: string | null): RoomPlanView {
  return raw === "tag" ? "tag" : "woche";
}

/** Pfad in den Betreuungsplan mit Raumfilter, Tag und Ansicht. */
export function useRoomPlanHref(
  roomId: string,
  view: RoomPlanView,
  anchorISO: string,
  canManageSchedules: boolean,
): string {
  const tenantPath = useTenantAwarePath();
  const query = new URLSearchParams({ room: roomId, d: anchorISO });
  if (view === "tag") query.set("view", "tag");
  // Planende ändern im Betreuungsplan; alle anderen sehen ihn im Kalender
  // (Leseansicht, #2283), der die Plan-Parameter in den Reiter übernimmt.
  const base = canManageSchedules ? "/betreuungsplan" : "/calendar";
  return tenantPath(`${base}?${query.toString()}`);
}
