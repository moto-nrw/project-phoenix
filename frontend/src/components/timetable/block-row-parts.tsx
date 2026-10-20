// Gemeinsame Teile der Blockzeilen im Tagesbetrieb (#4350): Tagesplan,
// "Mein Tag", "Ablauf des Tages" und "Als Nächstes" in der Aufsicht zeigen
// denselben Block, also mit derselben Kante, derselben Markierung für den
// eigenen Block und denselben Symbolen für Raum und Personal.

import type { CSSProperties } from "react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TIMETABLE_UNTYPED_EDGE_COLOR } from "~/components/timetable/timetable-style";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";

type BlockColorFields = Pick<
  PlannedTimetableInstance,
  "status" | "planningTrackColor"
>;

/** Kantenfarbe eines Blocks: rot bei Absage, sonst die Planungsspur. */
function blockEdgeColor(block: BlockColorFields): string {
  if (block.status === "cancelled") return LOCATION_COLORS.DANGER;
  return block.planningTrackColor ?? TIMETABLE_UNTYPED_EDGE_COLOR;
}

/**
 * Linke Kante und, für den eigenen Block, die Tönung in seiner Farbe. Die
 * Tönung ist dieselbe wie bei PlanBlock (ui/plan-block.tsx); eine Absage
 * bleibt ungetönt. `mine` setzt nur, wer eigene und fremde Blöcke
 * nebeneinander zeigt: wo alles eigen ist, sagte die Tönung nichts.
 */
export function blockRowStyle(
  block: BlockColorFields,
  mine: boolean,
): CSSProperties {
  const edge = blockEdgeColor(block);
  return {
    borderLeftColor: edge,
    ...(mine && block.status !== "cancelled"
      ? { backgroundColor: `${edge}1A` }
      : {}),
  };
}

/**
 * Die Tönung ist nur Farbe; Vorleseprogramme bekommen das Wort dazu. In den
 * Titel des Blocks setzen.
 */
export function OwnBlockHint({ mine }: Readonly<{ mine: boolean }>) {
  return mine ? <span className="sr-only"> · Mein Block</span> : null;
}

// Die Symbole sind Schmuck: der Text daneben sagt schon, was gemeint ist.
const META_ICON = {
  size: 14,
  className: "mr-1 inline-block align-[-2px]",
} as const;

/** Raumsymbol vor dem Raumnamen, inline in einer Textzeile. */
export function BlockRoom({ room }: Readonly<{ room: string }>) {
  return (
    <>
      <MotoConceptIcon concept="rooms" {...META_ICON} />
      {room}
    </>
  );
}

/** Personalsymbol vor den Namen, inline in einer Textzeile. */
export function BlockStaff({ names }: Readonly<{ names: string }>) {
  return (
    <>
      <MotoConceptIcon concept="staff" {...META_ICON} />
      {names}
    </>
  );
}

/** Eingeteiltes Personal als eine Zeile, Vertretungen benannt. */
export function blockStaffNames(
  block: Pick<PlannedTimetableInstance, "staffNames">,
): string | null {
  const names = block.staffNames ?? [];
  if (names.length === 0) return null;
  return names
    .map((entry) =>
      entry.isSubstitute
        ? `${entry.displayName} (Vertretung)`
        : entry.displayName,
    )
    .join(", ");
}
