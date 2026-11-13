import { bffJson } from "./bff-fetch";

/** Was das Entfernen eines Raums bewirkt (#6845). */
export type RoomRemovalOutcome = "delete" | "archive" | "blocked";

interface RoomReferences {
  /** Aktivitäten und Regeltermine, die den Raum planen. */
  readonly activity_groups: number;
  /** Geplante oder laufende Termine ab heute. */
  readonly upcoming_blocks: number;
  readonly education_groups: number;
  readonly devices: number;
  readonly past_blocks: number;
  readonly archived_activity_groups: number;
  readonly has_session_history: boolean;
}

export interface RoomRemovalImpact {
  readonly outcome: RoomRemovalOutcome;
  readonly active_session: boolean;
  readonly care_offering: boolean;
  readonly references: RoomReferences;
}

export function fetchRoomRemovalImpact(
  roomId: string,
): Promise<RoomRemovalImpact> {
  return bffJson<RoomRemovalImpact>(
    `/api/rooms/${encodeURIComponent(roomId)}/removal-impact`,
  );
}

/** Löscht oder archiviert den Raum; das Ergebnis sagt, was geschah. */
export async function removeRoom(
  roomId: string,
): Promise<Exclude<RoomRemovalOutcome, "blocked">> {
  const result = await bffJson<{ outcome?: RoomRemovalOutcome } | undefined>(
    `/api/rooms/${encodeURIComponent(roomId)}`,
    { method: "DELETE" },
  );
  return result?.outcome === "archive" ? "archive" : "delete";
}
