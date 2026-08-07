export type NotificationBadgeTone = "staff" | "parents" | "feedback";
export type NotificationBadgeSize = "sm" | "md";

// The count is small bold text, so WCAG AA wants 4.5:1 against the pill.
// bg-moto-orange (#F78C10) carries white at 2.41:1 — below even the 3:1
// non-text floor — so the staff tone uses the -strong ramp step (5.63:1).
// See the contrast note on ACCESSIBLE_TEXT_COLORS in lib/location-helper.ts.
const TONE_CLASSES: Record<NotificationBadgeTone, string> = {
  staff: "bg-moto-orange-strong text-white",
  parents: "bg-moto-blue text-white",
  feedback: "bg-moto-coral text-white",
};

const SIZE_CLASSES: Record<NotificationBadgeSize, string> = {
  sm: "h-4 min-w-4 px-1 text-[10px]",
  md: "h-5 min-w-5 px-1.5 text-xs",
};

export function NotificationBadge({
  count,
  tone,
  ariaLabel,
  size = "md",
  className,
}: Readonly<{
  count: number;
  tone: NotificationBadgeTone;
  ariaLabel: string;
  size?: NotificationBadgeSize;
  className?: string;
}>) {
  if (count <= 0) return null;

  return (
    <span
      className={`${TONE_CLASSES[tone]} ${SIZE_CLASSES[size]} inline-flex items-center justify-center rounded-full font-bold ${className ?? ""}`}
      aria-label={ariaLabel}
    >
      {count > 99 ? "99+" : count}
    </span>
  );
}
