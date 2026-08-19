import { NotificationBadge } from "~/components/ui/notification-badge";

export type UnreadBadgeTone = "parents";

/**
 * The domain-colored unread count shown on parent surfaces. One component
 * keeps size, weight, the 99+ cap, and the accessible label aligned.
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
