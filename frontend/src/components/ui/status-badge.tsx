// Tinted status pill with dot, the enrollment-page badge recipe (#1629).
// Tones map to the brand hexes from LOCATION_COLORS with matching tinted
// backgrounds and darkened label colors. Use this whenever a status is one of
// a fixed set of semantic outcomes; for data-driven colors (a raw hex from
// LOCATION_COLORS) use StatusDotBadge instead.
export type StatusBadgeTone = "blue" | "green" | "orange" | "red" | "gray";

const TONES: Record<
  StatusBadgeTone,
  { bg: string; dot: string; text: string }
> = {
  blue: { bg: "#EEF3FF", dot: "#5080D8", text: "#355A9A" },
  green: { bg: "#83CD2D1A", dot: "#83CD2D", text: "#5A8B1F" },
  orange: { bg: "#FFF4E6", dot: "#F78C10", text: "#8A5600" },
  red: { bg: "#FF31301A", dot: "#FF3130", text: "#9F1F1E" },
  gray: { bg: "#F3F4F6", dot: "#9CA3AF", text: "#4B5563" },
};

export function StatusBadge({
  label,
  tone,
  title,
}: {
  readonly label: string;
  readonly tone: StatusBadgeTone;
  /**
   * Native tooltip on the badge itself. Use when the short label needs the
   * full context on hover (e.g. "Feiertag" → the holiday's name). Set it here
   * rather than on a wrapper so the tooltip belongs to the element that
   * carries the text.
   */
  readonly title?: string;
}) {
  const styles = TONES[tone];
  return (
    <span
      title={title}
      className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      <span
        className="h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: styles.dot }}
        aria-hidden="true"
      />
      {label}
    </span>
  );
}
