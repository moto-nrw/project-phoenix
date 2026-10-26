// Wer ein Kind an- oder abgemeldet hat (#4761). Das Backend liefert die Art
// und, nur bei Mitarbeitenden, den Namen; der Text entsteht hier, damit
// Kinderseite und Anwesenheitsprotokoll dieselben Wörter zeigen.

export type AttendanceActorKind =
  "staff" | "device" | "former_staff" | "automatic";

export interface AttendanceActor {
  kind: AttendanceActorKind;
  name?: string;
}

const ACTOR_KINDS: ReadonlySet<string> = new Set([
  "staff",
  "device",
  "former_staff",
  "automatic",
]);

/** Liest die Angabe aus der Antwort; eine unbekannte Art zeigt nichts. */
export function parseAttendanceActor(
  raw: { kind?: string; name?: string } | null | undefined,
): AttendanceActor | null {
  if (!raw?.kind || !ACTOR_KINDS.has(raw.kind)) return null;
  const kind = raw.kind as AttendanceActorKind;
  if (kind === "staff" && !raw.name) return null;
  return kind === "staff" ? { kind, name: raw.name } : { kind };
}

/** Der Wert hinter „Angemeldet:“ und „Abgemeldet:“. */
export function attendanceActorLabel(actor: AttendanceActor | null): string {
  switch (actor?.kind) {
    case "staff":
      return actor.name ?? "";
    case "device":
      return "am Gerät";
    case "former_staff":
      return "Ehemalige Mitarbeitende";
    case "automatic":
      return "automatisch";
    default:
      return "";
  }
}
