import { readCodedApiError } from "./coded-api-error";

/**
 * Fehlercodes, wenn ein Raum oder eine Aktivität voll ist (#3632, #3633).
 * Beide Grenzen gelten unabhängig voneinander. Die Meldung muss sagen, welche
 * erreicht ist: sonst erhöht das Personal die Raumkapazität, obwohl die
 * Aktivität voll ist.
 */
export const ROOM_CAPACITY_CODE = "presence.room_capacity_exceeded";
export const ACTIVITY_PARTICIPANT_LIMIT_CODE =
  "presence.activity_participant_limit_reached";

interface CapacityMessageOptions {
  /**
   * false, wo die Person die Grenze nicht selbst ändern kann (Schulportal).
   * Dann nennt die Meldung die OGS statt der Datenverwaltung.
   */
  canChangeLimit?: boolean;
}

interface CapacityKind {
  /** „Die Aktivität“ / „Der Raum“, Satzanfang. */
  subject: string;
  /** „In der Aktivität“ / „Im Raum“, Satzanfang. */
  inside: string;
  nameKey: string;
  maxKey: string;
  unit: (count: number) => string;
  whereToChange: string;
}

const ACTIVITY: CapacityKind = {
  subject: "Die Aktivität",
  inside: "In der Aktivität",
  nameKey: "activity_name",
  maxKey: "max_participants",
  unit: (count) => (count === 1 ? "Kind" : "Kindern"),
  whereToChange:
    "Die Grenze ändern Sie unter Datenverwaltung → Aktivitäten bei „Maximale Teilnehmer“.",
};

const ROOM: CapacityKind = {
  subject: "Der Raum",
  inside: "Im Raum",
  nameKey: "room_name",
  maxKey: "max_capacity",
  unit: (count) => (count === 1 ? "Platz" : "Plätzen"),
  whereToChange:
    "Die Grenze ändern Sie unter Datenverwaltung → Räume bei „Maximale Belegung“.",
};

function numberField(details: Record<string, unknown>, key: string) {
  const value = details[key];
  return typeof value === "number" ? value : null;
}

function occupancySentence(
  kind: CapacityKind,
  details: Record<string, unknown>,
): string {
  const rawName = details[kind.nameKey];
  const name =
    typeof rawName === "string" && rawName.trim() !== ""
      ? ` „${rawName.trim()}“`
      : "";
  const current = numberField(details, "current_occupancy");
  const max = numberField(details, kind.maxKey);
  if (current === null || max === null) {
    return `${kind.subject}${name} ist voll.`;
  }
  const occupancy = `(${current} von ${max} ${kind.unit(max)})`;
  const incoming = numberField(details, "incoming_students") ?? 1;
  const free = max - current;
  // Eine Sammelaktion passt nicht mehr ganz hinein, es ist aber noch Platz:
  // „voll“ wäre hier falsch und würde zum Weiterprobieren einladen.
  if (incoming > 1 && free > 0) {
    const places =
      free === 1 ? "ist nur noch 1 Platz" : `sind nur noch ${free} Plätze`;
    return `${kind.inside}${name} ${places} frei ${occupancy}. Es sollen ${incoming} Kinder dazukommen.`;
  }
  return `${kind.subject}${name} ist voll ${occupancy}.`;
}

/**
 * Meldung für einen vollen Raum oder eine volle Aktivität, sonst null. Name
 * und Belegung kommen aus den Details der Antwort; fehlen sie, bleibt die
 * Meldung eindeutig „Raum voll“ bzw. „Aktivität voll“.
 */
export function capacityErrorMessage(
  err: unknown,
  { canChangeLimit = true }: CapacityMessageOptions = {},
): string | null {
  const coded = readCodedApiError(err);
  let kind: CapacityKind | null = null;
  if (coded?.code === ROOM_CAPACITY_CODE) kind = ROOM;
  if (coded?.code === ACTIVITY_PARTICIPANT_LIMIT_CODE) kind = ACTIVITY;
  if (!coded || !kind) return null;
  const hint = canChangeLimit
    ? kind.whereToChange
    : "Mehr Plätze kann die OGS freigeben.";
  return `${occupancySentence(kind, coded.details)} ${hint}`;
}
