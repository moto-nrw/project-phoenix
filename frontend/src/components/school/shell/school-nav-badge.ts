import type { SchoolStaffNoticesPending } from "~/lib/hooks/use-school-staff-notices-pending";
import type { SchoolTeamChatUnread } from "~/lib/hooks/use-school-team-chat-unread";
import type { SchoolNavItem } from "./school-nav-items";

/** Die Zähler, die die beiden Leisten des Schul-Portals kennen. */
export interface SchoolNavCounts {
  readonly teamChat: SchoolTeamChatUnread;
  readonly notices?: SchoolStaffNoticesPending;
}

/**
 * Welche Zahl mit welcher Beschriftung neben einem Eintrag steht. Ein Ort
 * für beide Leisten, damit "3 ungelesene Nachrichten" und "2 offene
 * Tagesinformationen" nicht zweimal formuliert werden.
 */
export function schoolNavBadge(
  item: SchoolNavItem,
  counts: SchoolNavCounts,
): { count: number; ariaLabel: string } | null {
  switch (item.badge) {
    case "teamChat": {
      const count = counts.teamChat.unreadCount;
      return { count, ariaLabel: `${count} ungelesene Nachrichten` };
    }
    case "notices": {
      const count = counts.notices?.pendingCount ?? 0;
      return { count, ariaLabel: `${count} offene Tagesinformationen` };
    }
    default:
      return null;
  }
}
