import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { dayStatusHint, type DayStatusKey } from "~/lib/presence-status";

/**
 * A fact about a child that sits next to its location pill: present but not
 * planned for today, or present despite a sick note, class trip or excused
 * absence. Exactly one pill per child (#4371): the pill says where the child
 * is, this line says what else staff need to know. Icon and text stay neutral
 * gray so the card carries one colour, and the label alone carries the
 * meaning; the icon is the concept glyph and decorative. Text, glyph and
 * marker come from the presence status table.
 */
type PresenceHintSize = "sm" | "md" | "lg";

const HINT_TEXT_SIZE: Record<PresenceHintSize, string> = {
  sm: "text-xs",
  md: "text-xs",
  lg: "text-sm",
};

interface PresenceHintProps {
  readonly status: DayStatusKey;
  readonly title?: string;
  readonly size?: PresenceHintSize;
}

export function PresenceHint({
  status,
  title,
  size = "md",
}: PresenceHintProps) {
  const hint = dayStatusHint(status);
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
