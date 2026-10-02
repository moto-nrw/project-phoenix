// Fachkräfte, Raum und Gruppe direkt im Block (#3817): Betreuungsplan und
// Vertretungsplan zeigen auf einen Blick, wer einen Block betreut. Die Namen
// kommen aus der Personalliste, die beide Pläne ohnehin laden; die
// Wochenliste trägt nur Personal-IDs.

import type { ScheduleSubstitutionOverview } from "~/lib/substitution-helpers";
import type {
  EnrichedInstance,
  InstanceStaffSummary,
} from "~/lib/timetable-types";

export interface BlockStaffEntry {
  staffId: string;
  /** Voller Name für Tooltip und Screenreader. */
  fullName: string;
  /** Kurzform im Block: „Anna K.“. */
  label: string;
  isAbsent: boolean;
  isSubstitute: boolean;
}

/**
 * Alle Namen, die die Vertretungsübersicht kennt: die möglichen Ersatzkräfte
 * und die Fachkräfte jedes Termins. Die Übersicht braucht nur
 * `schedules:read`, deshalb trägt sie die Namen auch für Konten, die die
 * Personalliste nicht lesen dürfen.
 */
export function staffNamesFromOverview(
  overview: ScheduleSubstitutionOverview | undefined,
): Map<string, string> {
  const names = new Map<string, string>();
  for (const appointment of overview?.appointments ?? []) {
    for (const row of appointment.staff) names.set(row.id, row.name);
  }
  for (const member of overview?.staff ?? []) names.set(member.id, member.name);
  return names;
}

/** Wie viele reguläre Namen ein Block neben Abweichungen zeigen darf. */
const BLOCK_STAFF_VISIBLE = 2;

/**
 * „Anna Kowalski“ → „Anna K.“; ein einzelnes Wort bleibt, wie es ist. Der
 * Nachname zählt ab dem letzten Wort, damit „Anna von Berg“ zu „Anna B.“ wird.
 */
export function shortStaffName(fullName: string): string {
  const parts = fullName.trim().split(/\s+/).filter(Boolean);
  if (parts.length < 2) return parts[0] ?? "";
  const first = parts[0] ?? "";
  const last = parts[parts.length - 1] ?? "";
  return `${first} ${last.charAt(0)}.`;
}

/**
 * Die eingeteilten Fachkräfte eines Blocks in Lesereihenfolge: zuerst die
 * Abweichungen (Abwesende, dann Ersatzkräfte), danach wer regulär betreut,
 * Hauptkraft zuerst. So fällt eine Abwesenheit nie unter „+N“. Eine
 * wieder entfernte Ersatzkraft (Ersatz und abwesend zugleich) betreut nicht
 * und war auch nie eingeplant; sie fehlt. Personen ohne bekannten Namen
 * fehlen ebenfalls, der Block zählt sie weiter in seinen Zahlen.
 */
export function blockStaffEntries(
  staff: readonly InstanceStaffSummary[],
  staffNames: ReadonlyMap<string, string>,
): BlockStaffEntry[] {
  const rank = (row: InstanceStaffSummary) => {
    if (row.isAbsent) return 0;
    if (row.isSubstitute) return 1;
    return row.isPrimary ? 2 : 3;
  };
  const entries = staff
    .filter((row) => !(row.isSubstitute && row.isAbsent))
    .flatMap((row) => {
      const fullName = staffNames.get(row.staffId)?.trim();
      if (!fullName) return [];
      return [
        {
          row,
          entry: {
            staffId: row.staffId,
            fullName,
            label: shortStaffName(fullName),
            isAbsent: row.isAbsent,
            isSubstitute: row.isSubstitute,
          },
        },
      ];
    })
    .sort(
      (a, b) =>
        rank(a.row) - rank(b.row) ||
        a.entry.fullName.localeCompare(b.entry.fullName, "de"),
    )
    .map(({ entry }) => entry);

  // Zwei Kurzformen dürfen sich im selben Block nicht gleichen („Anna K.“
  // für Anna Kowalski und Anna Krüger): dann steht der volle Name.
  const labelCounts = new Map<string, number>();
  for (const entry of entries) {
    labelCounts.set(entry.label, (labelCounts.get(entry.label) ?? 0) + 1);
  }
  return entries.map((entry) =>
    (labelCounts.get(entry.label) ?? 0) > 1
      ? { ...entry, label: entry.fullName }
      : entry,
  );
}

function describeEntry(entry: BlockStaffEntry): string {
  if (entry.isAbsent) return `${entry.fullName} (abwesend)`;
  if (entry.isSubstitute) return `${entry.fullName} (Ersatz)`;
  return entry.fullName;
}

/** „Raum 104 · Gruppe Sonne“ — Raum und Gruppe, soweit bekannt. */
export function blockPlaceLine(
  instance: Pick<EnrichedInstance, "roomName" | "groupName">,
): string {
  return [instance.roomName, instance.groupName].filter(Boolean).join(" · ");
}

/**
 * Der vollständige Text für Tooltip und Screenreader: Raum und Gruppe mit
 * Bezeichnung, dann alle Fachkräfte mit vollem Namen.
 */
export function blockDetailLines(
  instance: Pick<EnrichedInstance, "roomName" | "groupName">,
  entries: readonly BlockStaffEntry[],
): string[] {
  return [
    instance.roomName ? `Raum: ${instance.roomName}` : null,
    instance.groupName ? `Gruppe: ${instance.groupName}` : null,
    entries.length > 0
      ? `Fachkräfte: ${entries.map(describeEntry).join(", ")}`
      : null,
  ].filter((line): line is string => line !== null);
}

/**
 * Die Namenszeile im Block: abwesende Fachkräfte rot durchgestrichen,
 * Ersatzkräfte grün mit „(Ersatz)“, weitere reguläre Fachkräfte als „+N“.
 * Abweichungen bleiben immer sichtbar. Die Zeile kürzt sich in schmalen
 * Spalten selbst; „+N“ bleibt dabei sichtbar.
 */
export function BlockStaffNames({
  entries,
  className = "",
}: Readonly<{ entries: readonly BlockStaffEntry[]; className?: string }>) {
  if (entries.length === 0) return null;
  const deviations = entries.filter(
    (entry) => entry.isAbsent || entry.isSubstitute,
  );
  const regular = entries.filter(
    (entry) => !entry.isAbsent && !entry.isSubstitute,
  );
  const visible = [
    ...deviations,
    ...regular.slice(0, Math.max(0, BLOCK_STAFF_VISIBLE - deviations.length)),
  ];
  const hidden = entries.length - visible.length;
  return (
    <span className={`flex min-w-0 text-xs text-gray-700 ${className}`}>
      <span className="truncate">
        {visible.map((entry, index) => (
          <span key={entry.staffId}>
            {index > 0 ? ", " : null}
            {entry.isAbsent ? (
              <span className="text-moto-red-strong line-through">
                {entry.label}
                <span className="sr-only"> (abwesend)</span>
              </span>
            ) : entry.isSubstitute ? (
              <span className="text-moto-green-strong font-medium">
                {entry.label} (Ersatz)
              </span>
            ) : (
              entry.label
            )}
          </span>
        ))}
      </span>
      {hidden > 0 ? (
        <span className="shrink-0 pl-1 text-gray-500">
          +{hidden}
          <span className="sr-only"> weitere</span>
        </span>
      ) : null}
    </span>
  );
}
