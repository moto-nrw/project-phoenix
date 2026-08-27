"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { useSchoolTeamChatUnread } from "~/lib/hooks/use-school-team-chat-unread";
import { schoolPath } from "~/lib/school-url";
import { isSchoolNavActive } from "./school-nav-active";
import {
  SCHOOL_PRIMARY_NAV,
  SCHOOL_SECONDARY_NAV,
  type SchoolNavItem,
} from "./school-nav-items";

const ROW =
  "flex items-center rounded-lg px-3 py-2.5 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none lg:px-4 lg:py-3 lg:text-base xl:px-3 xl:py-2.5 xl:text-sm";
const ROW_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const ROW_IDLE =
  "font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900";
const ICON =
  "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5";

/**
 * Die dauerhafte Seitennavigation des Schul-Portals ab 1024 px.
 *
 * Unter 1024 px übernimmt die mobile Leiste; CSS blendet die Spalte dort
 * bereits beim ersten Paint aus, ohne Hydrationssprung. Abmelden sitzt im
 * Profil-Menü der geteilten Kopfzeile, das auf jeder Breite erreichbar ist.
 */
export function SchoolSidebar() {
  const pathname = usePathname();
  const teamChat = useSchoolTeamChatUnread();

  const renderItem = (item: SchoolNavItem) => {
    // Der Team-Chat ist standardmäßig aus; der Eintrag erscheint erst, wenn
    // die Schule ihn eingeschaltet hat (siehe SchoolNavItem.optional).
    if (item.optional === "teamChat" && teamChat.available !== true) {
      return null;
    }
    const active = isSchoolNavActive(item.href, pathname);
    return (
      <li key={item.key}>
        <Link
          href={item.portalPath ? schoolPath(item.href) : item.href}
          data-school-nav-item={item.key}
          data-active={active ? "true" : "false"}
          aria-current={active ? "page" : undefined}
          {...(item.newTab
            ? { target: "_blank", rel: "noopener noreferrer" }
            : {})}
          className={`${ROW} ${active ? ROW_ACTIVE : ROW_IDLE}`}
        >
          <MotoNavIcon
            concept={item.concept}
            active={active}
            className={`${ICON} ${active ? "" : "text-gray-400"}`}
          />
          <span className="flex flex-1 items-center">
            {item.label}
            {item.badge === "teamChat" && (
              <NotificationBadge
                count={teamChat.unreadCount}
                tone="staff"
                ariaLabel={`${teamChat.unreadCount} ungelesene Nachrichten`}
                className="ml-2"
              />
            )}
          </span>
        </Link>
      </li>
    );
  };

  return (
    <aside className="hidden min-h-screen w-64 shrink-0 border-r border-gray-200/70 bg-white/95 lg:block">
      <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
        <nav
          aria-label="Hauptnavigation"
          className="flex-1 overflow-y-auto p-3 lg:p-4 xl:p-3"
        >
          <ul>{SCHOOL_PRIMARY_NAV.map(renderItem)}</ul>
        </nav>

        <nav
          aria-label="Hilfe"
          className="border-t border-gray-200 p-3 lg:p-4 xl:p-3"
        >
          <ul>{SCHOOL_SECONDARY_NAV.map(renderItem)}</ul>
        </nav>
      </div>
    </aside>
  );
}
