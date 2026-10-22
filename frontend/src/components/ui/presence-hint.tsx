import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { LOCATION_STATUSES } from "~/lib/location-helper";

/**
 * A fact about a child that sits next to its location pill: present but not
 * planned for today, or present despite a sick note, class trip or excused
 * absence. Exactly one pill per child (#4371): the pill says where the child
 * is, this line says what else staff need to know. Icon and text stay neutral
 * gray so the card carries one colour, and the label alone carries the
 * meaning; the icon is the concept glyph and decorative. "Ungeplant anwesend"
 * borrows the glyph of the plan it contradicts ("Kommt heute nicht").
 */
export type PresenceHintKind = "notArrival" | "sick" | "classTrip" | "excused";

const HINTS = {
  notArrival: {
    label: LOCATION_STATUSES.UNPLANNED_PRESENT,
    concept: "notArrival",
    dataAttr: "data-not-arrival-indicator",
  },
  sick: {
    label: LOCATION_STATUSES.SICK,
    concept: "sick",
    dataAttr: "data-sick-indicator",
  },
  classTrip: {
    label: LOCATION_STATUSES.CLASS_TRIP,
    concept: "classTrip",
    dataAttr: "data-class-trip-indicator",
  },
  excused: {
    label: LOCATION_STATUSES.EXCUSED,
    concept: "excused",
    dataAttr: "data-excused-indicator",
  },
} as const;

export function PresenceHint({
  kind,
  title,
}: Readonly<{ kind: PresenceHintKind; title?: string }>) {
  const hint = HINTS[kind];
  return (
    <span
      className="mt-1 inline-flex items-center gap-1 text-xs font-medium whitespace-nowrap text-gray-600"
      title={title}
      {...{ [hint.dataAttr]: "true" }}
    >
      <MotoConceptIcon concept={hint.concept} tone="neutral" size={14} />
      {hint.label}
    </span>
  );
}
