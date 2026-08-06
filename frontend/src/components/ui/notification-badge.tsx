export type NotificationBadgeTone = "staff" | "parents" | "feedback";
export type NotificationBadgeSize = "sm" | "md";

const TONE_CLASSES: Record<NotificationBadgeTone, string> = {
  staff: "bg-moto-orange text-white",
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
