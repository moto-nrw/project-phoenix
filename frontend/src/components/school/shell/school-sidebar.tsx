"use client";

import { usePathname } from "next/navigation";
import { NotificationBadge } from "~/components/ui/notification-badge";
import {
  PortalSidebar,
  PortalSidebarLink,
} from "~/components/ui/portal-sidebar";
import type { SchoolStaffNoticesPending } from "~/lib/hooks/use-school-staff-notices-pending";
import type { SchoolTeamChatUnread } from "~/lib/hooks/use-school-team-chat-unread";
import { schoolPath } from "~/lib/school-url";
import { isSchoolNavActive } from "./school-nav-active";
import { schoolNavBadge } from "./school-nav-badge";
import {
  SCHOOL_PRIMARY_NAV,
  SCHOOL_SECONDARY_NAV,
  type SchoolNavItem,
} from "./school-nav-items";

/**
 * Die dauerhafte Seitennavigation des Schul-Portals ab 1024 px.
 *
 * Unter 1024 px übernimmt die mobile Leiste; CSS blendet die Spalte dort
 * bereits beim ersten Paint aus, ohne Hydrationssprung. Abmelden sitzt im
 * Profil-Menü der geteilten Kopfzeile, das auf jeder Breite erreichbar ist.
 */
export function SchoolSidebar({
  teamChat,
  notices,
}: {
  readonly teamChat: SchoolTeamChatUnread;
  /** Offene Tagesinformationen (#2208); ohne Wert steht keine Zahl. */
  readonly notices?: SchoolStaffNoticesPending;
}) {
  const pathname = usePathname();

  const renderItem = (item: SchoolNavItem) => {
    // Der Team-Chat ist standardmäßig an (#3254), die Schule kann ihn
    // ausschalten; der Eintrag erscheint erst, wenn feststeht, dass er an ist
    // (siehe SchoolNavItem.optional).
    if (item.optional === "teamChat" && teamChat.available !== true) {
      return null;
    }
    const badge = schoolNavBadge(item, { teamChat, notices });
    return (
      <PortalSidebarLink
        key={item.key}
        href={item.portalPath ? schoolPath(item.href) : item.href}
        label={item.label}
        concept={item.concept}
        active={isSchoolNavActive(item.href, pathname)}
        newTab={item.newTab}
        data-school-nav-item={item.key}
        badge={
          badge && (
            <NotificationBadge
              count={badge.count}
              tone="staff"
              ariaLabel={badge.ariaLabel}
              className="ml-2"
            />
          )
        }
      />
    );
  };

  return (
    <PortalSidebar secondary={SCHOOL_SECONDARY_NAV.map(renderItem)}>
      {SCHOOL_PRIMARY_NAV.map(renderItem)}
    </PortalSidebar>
  );
}
