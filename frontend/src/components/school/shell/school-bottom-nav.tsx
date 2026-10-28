"use client";

import { usePathname } from "next/navigation";
import { NotificationBadge } from "~/components/ui/notification-badge";
import {
  PortalBottomNav,
  PortalBottomNavLink,
} from "~/components/ui/portal-bottom-nav";
import type { SchoolStaffNoticesPending } from "~/lib/hooks/use-school-staff-notices-pending";
import type { SchoolTeamChatUnread } from "~/lib/hooks/use-school-team-chat-unread";
import { schoolPath } from "~/lib/school-url";
import { isSchoolNavActive } from "./school-nav-active";
import { schoolNavBadge } from "./school-nav-badge";
import { SCHOOL_PRIMARY_NAV, SCHOOL_SECONDARY_NAV } from "./school-nav-items";

const SCHOOL_MOBILE_NAV = [...SCHOOL_PRIMARY_NAV, ...SCHOOL_SECONDARY_NAV];

/**
 * Die mobile Hauptnavigation des Schul-Portals.
 *
 * Beschriftet ist nur das aktive Ziel, die übrigen tragen ihr Icon und ihren
 * Namen als aria-label — dasselbe Muster wie die Eltern-App. Solange es zwei
 * Ziele waren, standen alle Beschriftungen dauerhaft da; mit dem dritten
 * (#2527) passte das auf einem 390 px breiten Gerät nicht mehr in die Pille:
 * die Zeile brauchte 447 px bei 324 px Platz, "Hilfe" lag komplett ausserhalb
 * des Bildschirms und "Meine Aufsichten" ragte über die Pille hinaus.
 *
 * Mit dem Team-Chat (#2208) sind es vier Ziele, und die längste Beschriftung
 * ("Meine Aufsichten") braucht dann rund 404 px Fensterbreite. Darunter — also
 * auf jedem gängigen Telefon — zeigt die Leiste in dieser Besetzung nur die
 * Icons; das aktive Ziel bleibt an seiner gefüllten Pille erkennbar, der Name
 * steht weiter im aria-label. Bei drei Zielen bleibt die Beschriftung wie
 * bisher immer sichtbar. Mit den Tagesinformationen (#2208) sind es ohne
 * Team-Chat vier und mit ihm fünf Ziele; die Regel bleibt dieselbe.
 *
 * Kein "Mehr"-Menü und kein gleitender Indikator — dafür sind es zu wenige
 * Ziele. Ab 1024 px übernimmt die Seitennavigation; CSS blendet diese Leiste
 * dort schon beim ersten Paint aus.
 */
export function SchoolBottomNav({
  teamChat,
  notices,
}: {
  readonly teamChat: SchoolTeamChatUnread;
  /** Offene Tagesinformationen (#2208); ohne Wert steht keine Zahl. */
  readonly notices?: SchoolStaffNoticesPending;
}) {
  const pathname = usePathname();
  const items = SCHOOL_MOBILE_NAV.filter(
    (item) => item.optional !== "teamChat" || teamChat.available === true,
  );
  // Ab dem vierten Ziel passt die längste Beschriftung erst ab 420 px in die
  // Pille. Schmalere Geräte bekommen die Icon-Zeile, statt dass ein Ziel aus
  // der Leiste geschoben wird. Seit den Tagesinformationen (#2208) sind es
  // immer mindestens vier Ziele, die Beschriftung ist also stets so geregelt.
  //
  // Mit den Elternbriefen (#4800) sind es sechs, mit Team-Chat sieben Ziele. Dann
  // rücken die Icons enger zusammen, damit die Zeile auf 360 px passt, und
  // die Beschriftung des aktiven Ziels kommt erst ab 480 px dazu.
  // Gemessen auf 360 px: sieben Icons à 40 px ohne Lücke passen in die
  // 294 px der Pille, mit px-3 und gap-1 wären es 332 px.
  const crowded = items.length >= 6;
  const itemPadding = crowded ? "px-2.5" : "px-4";
  const listGap = crowded ? "gap-0" : "gap-1";
  const labelClass = crowded
    ? "hidden text-sm font-semibold whitespace-nowrap min-[480px]:inline"
    : "hidden text-sm font-semibold whitespace-nowrap min-[420px]:inline";

  return (
    <PortalBottomNav listClassName={listGap}>
      {items.map((item) => {
        const badge = schoolNavBadge(item, { teamChat, notices });
        return (
          <PortalBottomNavLink
            key={item.key}
            href={item.portalPath ? schoolPath(item.href) : item.href}
            label={item.label}
            concept={item.concept}
            active={isSchoolNavActive(item.href, pathname)}
            newTab={item.newTab}
            data-school-nav-item={item.key}
            paddingClassName={itemPadding}
            labelClassName={labelClass}
            badge={
              badge && badge.count > 0 ? (
                <NotificationBadge
                  count={badge.count}
                  tone="staff"
                  size="sm"
                  ariaLabel={badge.ariaLabel}
                  className="absolute -top-2 -right-3"
                />
              ) : null
            }
          />
        );
      })}
    </PortalBottomNav>
  );
}
