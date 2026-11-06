"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { LogoutModal } from "~/components/auth/logout-modal";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { parentPath } from "~/lib/parent-url";
import { isParentNavActive, isParentSettingsPath } from "./parent-nav-active";
import { ParentSettingsSidebarEntries } from "./parent-settings-nav";
import type { ParentNavCounts } from "./parent-bottom-nav";
import {
  PortalSidebar,
  PortalSidebarButton,
  PortalSidebarLink,
} from "~/components/ui/portal-sidebar";
import { portalSidebarTopOffset } from "~/components/ui/sidebar-geometry";
import {
  PARENT_MORE_NAV,
  PARENT_PRIMARY_NAV,
  type ParentNavItem,
} from "./parent-nav-items";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { helpConcept } from "~/lib/concepts/help";
import { logoutConcept } from "~/lib/concepts/logout";
import { settingsConcept } from "~/lib/concepts/settings";
import { groupsConcept } from "~/lib/concepts/groups";

/**
 * Die dauerhafte Seitennavigation ab 1024 px.
 *
 * Auf dieser Breite ist Platz fuer alle Ziele, also stehen auch die Eintraege
 * hier offen, die auf dem Handy hinter "Mehr" liegen. Unter 1024 px blendet
 * CSS die Spalte bereits beim ersten Paint aus, ohne Hydrationssprung.
 */
export function ParentSidebar({
  badges,
  gates,
  childCount,
  demoBannerShown = false,
  eveningRow = false,
}: ParentNavCounts & {
  /** The demo banner (#3468) sits above the header; the sidebar moves with it. */
  readonly demoBannerShown?: boolean;
  /** Hinweiszeile der Demo unter dem Streifen (#3921), weitere 32 px. */
  readonly eveningRow?: boolean;
}) {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const [logoutModalOpen, setLogoutModalOpen] = useState(false);

  const secondary = PARENT_MORE_NAV.filter(
    (item): item is { kind: "link" } & ParentNavItem => item.kind === "link",
  ).filter(
    (item) =>
      item.key !== "settings" &&
      item.key !== "enroll" &&
      item.key !== "help" &&
      (!item.gate || gates[item.gate]),
  );

  // `external` wird hier mitgetragen, obwohl der einzige heutige Eintrag
  // dieser Art -- die Hilfe -- unten von Hand steht: der Typ lässt externe
  // Ziele in jeder Liste zu, und ein nächster soll nicht still über
  // `parentPath` laufen.
  const renderItem = (item: ParentNavItem) => {
    const active = !item.external && isParentNavActive(item.href, pathname);
    const count = item.badge ? (badges[item.badge] ?? 0) : 0;
    const label =
      item.key === "children"
        ? t(childCount === 1 ? "childSingle" : "childrenMultiple")
        : t(item.tKey);
    return (
      <PortalSidebarLink
        key={item.key}
        href={item.external ? item.href : parentPath(item.href)}
        label={label}
        concept={item.concept}
        iconConcept={
          item.key === "children" && childCount !== 1
            ? groupsConcept
            : undefined
        }
        active={active}
        newTab={item.external}
        data-parent-nav-item={item.key}
        badge={
          count > 0 ? (
            <NotificationBadge
              count={count}
              tone="parents"
              ariaLabel={t(
                item.badge === "news" ? "openCount" : "unreadCount",
                { count },
              )}
              className="ml-auto"
            />
          ) : null
        }
      />
    );
  };

  // In den Einstellungen wechselt der Inhalt derselben Leiste (#5750).
  const inSettings = isParentSettingsPath(pathname);
  const enrollActive = isParentNavActive("/parents/anmeldung", pathname);

  return (
    <>
      <PortalSidebar
        topOffset={portalSidebarTopOffset(demoBannerShown, eveningRow)}
        className="min-h-screen"
        navLabel={inSettings ? t("settings") : t("mainNav")}
        secondaryLabel={t("accountNav")}
        secondaryListClassName="space-y-1"
        secondary={
          inSettings ? null : (
            <>
              {/* Die Anleitung liegt ausserhalb des Portals: eigener Tab,
                  nie ein Aktivzustand. `role=parent` erspart den Eltern
                  die Frage, fuer wen die Anleitung ist. */}
              <PortalSidebarLink
                href="/help?role=parent"
                label={t("help")}
                concept={helpConcept}
                active={false}
                newTab
                data-parent-nav-item="help"
              />
              <PortalSidebarLink
                href={parentPath("/parents/settings")}
                label={t("settings")}
                concept={settingsConcept}
                active={false}
                data-parent-nav-item="settings"
              />
              <PortalSidebarLink
                href={parentPath("/parents/anmeldung")}
                label={t("enroll")}
                concept={enrollmentsConcept}
                active={enrollActive}
                data-parent-nav-item="enroll"
              />
              <PortalSidebarButton
                label={t("logout")}
                concept={logoutConcept}
                data-parent-nav-item="logout"
                onClick={() => setLogoutModalOpen(true)}
              />
            </>
          )
        }
      >
        {inSettings ? (
          <ParentSettingsSidebarEntries />
        ) : (
          <>
            {PARENT_PRIMARY_NAV.map(renderItem)}
            {secondary.map(renderItem)}
          </>
        )}
      </PortalSidebar>
      <LogoutModal
        isOpen={logoutModalOpen}
        onClose={() => setLogoutModalOpen(false)}
      />
    </>
  );
}
