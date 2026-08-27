import { NotificationBadge } from "~/components/ui/notification-badge";

// "staff" is the Mitarbeitende hue already used for staff counts in the sidebar
// and on the staff list; the internal Team-Chat badge reuses it rather than
// introducing a fourth colour for the same audience.
export type UnreadBadgeTone = "parents" | "feedback" | "staff";

/**
 * The domain-colored unread count shown on parent, feedback and team-chat
 * surfaces. One component keeps size, weight, the 99+ cap, and the accessible
 * label aligned.
 */
export function UnreadBadge({
  count,
  tone = "parents",
  className,
}: Readonly<{
  count: number;
  tone?: UnreadBadgeTone;
  className?: string;
}>) {
  return (
    <NotificationBadge
      count={count}
      tone={tone}
      ariaLabel={`${count} ungelesene Nachrichten`}
      className={className}
    />
  );
}
