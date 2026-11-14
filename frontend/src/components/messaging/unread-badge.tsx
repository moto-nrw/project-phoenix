import { NotificationBadge } from "~/components/ui/notification-badge";
import { COUNTER_TONE, type CounterAudience } from "~/lib/counter-tones";

export type UnreadBadgeTone = CounterAudience;

/**
 * The domain-colored unread count shown on parent, feedback and team-chat
 * surfaces. One component keeps size, weight, the 99+ cap, and the accessible
 * label aligned.
 */
export function UnreadBadge({
  count,
  tone = "parents",
  noun = "ungelesene Nachrichten",
  className,
}: Readonly<{
  count: number;
  tone?: UnreadBadgeTone;
  /** Was gezählt wird, für Vorleseprogramme, z. B. „ungelesene Anmeldungen". */
  noun?: string;
  className?: string;
}>) {
  return (
    <NotificationBadge
      count={count}
      tone={COUNTER_TONE[tone]}
      ariaLabel={`${count} ${noun}`}
      className={className}
    />
  );
}
