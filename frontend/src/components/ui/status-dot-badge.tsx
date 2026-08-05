import { getLocationBadgeTone } from "@/lib/location-helper";

export function StatusDotBadge({
  label,
  color,
}: {
  readonly label: string;
  readonly color: string;
}) {
  const tone = getLocationBadgeTone(color);

  return (
    <span
      className="inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-medium"
      style={{
        backgroundColor: tone.backgroundColor,
        color: tone.textColor,
      }}
    >
      <span
        aria-hidden
        className="inline-block h-1.5 w-1.5 rounded-full"
        style={{ backgroundColor: tone.dotColor }}
      />
      <span>{label}</span>
    </span>
  );
}
