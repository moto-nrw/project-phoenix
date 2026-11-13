/**
 * Palette names, not audiences: which audience gets which hue is the
 * caller's decision (#6582, `~/lib/counter-tones`).
 */
export type NotificationBadgeTone = "orange" | "blue" | "coral" | "red";
export type NotificationBadgeSize = "sm" | "md";

// The count is small bold text, so WCAG AA wants 4.5:1 against the pill.
// bg-moto-orange (#F78C10) carries white at 2.41:1 — below even the 3:1
// non-text floor — so the orange tone uses the -strong ramp step (5.63:1).
// See the contrast note on TEXT_SAFE_SHADES in kit-palette.ts.
const TONE_CLASSES: Record<NotificationBadgeTone, string> = {
  orange: "bg-moto-orange-strong text-white",
  blue: "bg-moto-blue-strong text-white",
  coral: "bg-moto-coral-strong text-white",
  red: "bg-moto-red text-white",
};

const SIZE_CLASSES: Record<NotificationBadgeSize, string> = {
  sm: "h-4 min-w-4 px-1 text-xs",
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
