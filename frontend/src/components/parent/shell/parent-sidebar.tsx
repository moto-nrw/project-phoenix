"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { LogoutModal } from "~/components/ui/logout-modal";
import { NavLink } from "~/components/ui/nav-link";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { parentPath } from "~/lib/parent-url";
import { isParentNavActive } from "./parent-nav-active";
import type { ParentNavCounts } from "./parent-bottom-nav";
import { NavConceptIcon } from "~/components/ui/nav-scene-icon/nav-scene-icon";
import {
  PORTAL_ICON_CLASSES,
  portalSidebarRowClasses,
} from "~/components/dashboard/sidebar-geometry";
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
      <li key={item.key}>
        <NavLink
          href={item.external ? item.href : parentPath(item.href)}
          {...(item.external
            ? { target: "_blank", rel: "noopener noreferrer" }
            : {})}
          data-parent-nav-item={item.key}
          data-active={active ? "true" : "false"}
          aria-current={active ? "page" : undefined}
          className={portalSidebarRowClasses(active)}
        >
          <NavConceptIcon
            concept={item.concept}
            iconConcept={
              item.key === "children" && childCount !== 1
                ? groupsConcept
                : undefined
            }
            active={active}
            className={`${PORTAL_ICON_CLASSES} ${active ? "" : "text-gray-400"}`}
          />
          <span className="flex-1">{label}</span>
          {count > 0 && (
            <NotificationBadge
              count={count}
              tone="parents"
              ariaLabel={t(
                item.badge === "news" ? "openCount" : "unreadCount",
                { count },
              )}
            />
          )}
        </NavLink>
      </li>
    );
  };

  const settingsActive = isParentNavActive("/parents/settings", pathname);
  const enrollActive = isParentNavActive("/parents/anmeldung", pathname);

  return (
    <>
      <aside className="hidden min-h-screen w-64 shrink-0 border-r border-gray-200/70 bg-white/95 lg:block">
        <div
          className={`sticky flex flex-col ${
            eveningRow
              ? "top-[153px] h-[calc(100vh-153px)]"
              : demoBannerShown
                ? "top-[121px] h-[calc(100vh-121px)]"
                : "top-[73px] h-[calc(100vh-73px)]"
          }`}
        >
          <nav
            aria-label={t("mainNav")}
            className="flex-1 overflow-y-auto p-3 lg:p-4 xl:p-3"
          >
            <ul>{PARENT_PRIMARY_NAV.map(renderItem)}</ul>
            {secondary.length > 0 && <ul>{secondary.map(renderItem)}</ul>}
          </nav>

          <nav
            aria-label={t("accountNav")}
            className="border-t border-gray-200 p-3 lg:p-4 xl:p-3"
          >
            <ul className="space-y-1">
              <li>
                {/* Die Anleitung liegt ausserhalb des Portals: eigener Tab,
                    nie ein Aktivzustand. `role=parent` erspart den Eltern
                    die Frage, fuer wen die Anleitung ist. */}
                <NavLink
                  href="/help?role=parent"
                  target="_blank"
                  rel="noopener noreferrer"
                  data-parent-nav-item="help"
                  data-active="false"
                  className={portalSidebarRowClasses(false)}
                >
                  <NavConceptIcon
                    concept={helpConcept}
                    active={false}
                    className={`${PORTAL_ICON_CLASSES} text-gray-400`}
                  />
                  <span className="flex-1">{t("help")}</span>
                </NavLink>
              </li>
              <li>
                <NavLink
                  href={parentPath("/parents/settings")}
                  data-parent-nav-item="settings"
                  data-active={settingsActive ? "true" : "false"}
                  aria-current={settingsActive ? "page" : undefined}
                  className={portalSidebarRowClasses(settingsActive)}
                >
                  <NavConceptIcon
                    concept={settingsConcept}
                    active={settingsActive}
                    className={`${PORTAL_ICON_CLASSES} ${settingsActive ? "" : "text-gray-400"}`}
                  />
                  <span className="flex-1">{t("settings")}</span>
                </NavLink>
              </li>
              <li>
                <NavLink
                  href={parentPath("/parents/anmeldung")}
                  data-parent-nav-item="enroll"
                  data-active={enrollActive ? "true" : "false"}
                  aria-current={enrollActive ? "page" : undefined}
                  className={portalSidebarRowClasses(enrollActive)}
                >
                  <NavConceptIcon
                    concept={enrollmentsConcept}
                    active={enrollActive}
                    className={`${PORTAL_ICON_CLASSES} ${enrollActive ? "" : "text-gray-400"}`}
                  />
                  <span className="flex-1">{t("enroll")}</span>
                </NavLink>
              </li>
              <li>
                <button
                  type="button"
                  data-parent-nav-item="logout"
                  onClick={() => setLogoutModalOpen(true)}
                  className={`${portalSidebarRowClasses(false)} w-full text-left`}
                >
                  <NavConceptIcon
                    concept={logoutConcept}
                    active={false}
                    className={`${PORTAL_ICON_CLASSES} text-gray-400`}
                  />
                  <span className="flex-1">{t("logout")}</span>
                </button>
              </li>
            </ul>
          </nav>
        </div>
      </aside>
      <LogoutModal
        isOpen={logoutModalOpen}
        onClose={() => setLogoutModalOpen(false)}
      />
    </>
  );
}
