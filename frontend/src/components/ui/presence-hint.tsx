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
export type PresenceHintSize = "sm" | "md" | "lg";

const HINT_TEXT_SIZE: Record<PresenceHintSize, string> = {
  sm: "text-[11px]",
  md: "text-xs",
  lg: "text-sm",
};

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

interface PresenceHintProps {
  readonly kind: PresenceHintKind;
  readonly title?: string;
  readonly size?: PresenceHintSize;
}

export function PresenceHint({
  kind,
  title,
  size = "md",
}: PresenceHintProps) {
  const hint = HINTS[kind];
  return (
    <span
      className={`mt-1 inline-flex items-center gap-1 font-medium whitespace-nowrap text-gray-600 ${HINT_TEXT_SIZE[size]}`}
      title={title}
      {...{ [hint.dataAttr]: "true" }}
    >
      <MotoConceptIcon concept={hint.concept} tone="neutral" size={14} />
      {hint.label}
    </span>
  );
}
