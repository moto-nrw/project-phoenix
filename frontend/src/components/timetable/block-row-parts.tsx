// Gemeinsame Teile der Blockzeilen im Tagesbetrieb (#4350): Tagesplan,
// "Mein Tag", "Ablauf des Tages" und "Als Nächstes" in der Aufsicht zeigen
// denselben Block, also mit derselben Markierung für den eigenen Block und
// denselben Symbolen für Raum und Personal. Die farbige Kante tragen die
// Zeilen; die Karte in "Als Nächstes" ist nur getönt (#4413).

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
 * Die Tönung des eigenen Blocks in seiner Farbe, dieselbe wie bei PlanBlock
 * (ui/plan-block.tsx); eine Absage bleibt ungetönt. `mine` setzt nur, wer
 * eigene und fremde Blöcke nebeneinander zeigt: wo alles eigen ist, sagte
 * die Tönung nichts. Ohne Kante für Karten wie „Als Nächstes“ (#4413).
 */
export function blockTintStyle(
  block: BlockColorFields,
  mine: boolean,
): CSSProperties {
  return mine && block.status !== "cancelled"
    ? { backgroundColor: `${blockEdgeColor(block)}1A` }
    : {};
}

/** Linke Kante in der Farbe des Blocks und die Tönung des eigenen Blocks. */
export function blockRowStyle(
  block: BlockColorFields,
  mine: boolean,
): CSSProperties {
  return {
    borderLeftColor: blockEdgeColor(block),
    ...blockTintStyle(block, mine),
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
