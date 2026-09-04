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
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import {
  PARENT_MORE_NAV,
  PARENT_PRIMARY_NAV,
  type ParentNavItem,
} from "./parent-nav-items";

const ROW =
  "flex items-center rounded-lg px-3 py-2.5 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none lg:px-4 lg:py-3 lg:text-base xl:px-3 xl:py-2.5 xl:text-sm";
const ROW_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const ROW_IDLE =
  "font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900";
const ICON =
  "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5";

/**
 * Die dauerhafte Seitennavigation ab 1024 px.
 *
 * Auf dieser Breite ist Platz fuer alle Ziele, also stehen auch die Eintraege
 * hier offen, die auf dem Handy hinter "Mehr" liegen. Unter 1024 px blendet
 * CSS die Spalte bereits beim ersten Paint aus, ohne Hydrationssprung.
 */
export function ParentSidebar({ badges, gates, childCount }: ParentNavCounts) {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const [logoutModalOpen, setLogoutModalOpen] = useState(false);

  const secondary = PARENT_MORE_NAV.filter(
    (item): item is { kind: "link" } & ParentNavItem => item.kind === "link",
  ).filter(
    (item) =>
      item.key !== "settings" &&
      item.key !== "enroll" &&
      (!item.gate || gates[item.gate]),
  );

  const renderItem = (item: ParentNavItem) => {
    const active = isParentNavActive(item.href, pathname);
    const count = item.badge ? (badges[item.badge] ?? 0) : 0;
    const label =
      item.key === "children"
        ? t(childCount === 1 ? "childSingle" : "childrenMultiple")
        : t(item.tKey);
    return (
      <li key={item.key}>
        <NavLink
          href={parentPath(item.href)}
          data-parent-nav-item={item.key}
          data-active={active ? "true" : "false"}
          aria-current={active ? "page" : undefined}
          className={`${ROW} ${active ? ROW_ACTIVE : ROW_IDLE}`}
        >
          <MotoNavIcon
            concept={item.concept}
            iconConcept={
              item.key === "children" && childCount !== 1 ? "groups" : undefined
            }
            active={active}
            className={`${ICON} ${active ? "" : "text-gray-400"}`}
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
  const enrollActive = isParentNavActive("/parents/enroll", pathname);

  return (
    <>
      <aside className="hidden min-h-screen w-64 shrink-0 border-r border-gray-200/70 bg-white/95 lg:block">
        <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
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
                <NavLink
                  href={parentPath("/parents/settings")}
                  data-parent-nav-item="settings"
                  data-active={settingsActive ? "true" : "false"}
                  aria-current={settingsActive ? "page" : undefined}
                  className={`${ROW} ${settingsActive ? ROW_ACTIVE : ROW_IDLE}`}
                >
                  <MotoNavIcon
                    concept="settings"
                    active={settingsActive}
                    className={`${ICON} ${settingsActive ? "" : "text-gray-400"}`}
                  />
                  <span className="flex-1">{t("settings")}</span>
                </NavLink>
              </li>
              <li>
                <NavLink
                  href={parentPath("/parents/enroll")}
                  data-parent-nav-item="enroll"
                  data-active={enrollActive ? "true" : "false"}
                  aria-current={enrollActive ? "page" : undefined}
                  className={`${ROW} ${enrollActive ? ROW_ACTIVE : ROW_IDLE}`}
                >
                  <MotoNavIcon
                    concept="enrollments"
                    active={enrollActive}
                    className={`${ICON} ${enrollActive ? "" : "text-gray-400"}`}
                  />
                  <span className="flex-1">{t("enroll")}</span>
                </NavLink>
              </li>
              <li>
                <button
                  type="button"
                  data-parent-nav-item="logout"
                  onClick={() => setLogoutModalOpen(true)}
                  className={`${ROW} ${ROW_IDLE} w-full text-left`}
                >
                  <MotoNavIcon
                    concept="logout"
                    active={false}
                    className={`${ICON} text-gray-400`}
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
