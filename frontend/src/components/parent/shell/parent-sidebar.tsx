"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { parentPath } from "~/lib/parent-url";
import { useShellAuth } from "~/lib/shell-auth-context";
import { SignOut, UserCircle } from "./parent-icons";
import { isParentNavActive } from "./parent-nav-active";
import type { ParentNavCounts } from "./parent-bottom-nav";
import {
  PARENT_ICON_WEIGHT,
  PARENT_ICON_WEIGHT_ACTIVE,
  PARENT_MORE_NAV,
  PARENT_PRIMARY_NAV,
  type ParentNavItem,
} from "./parent-nav-items";

const ROW =
  "flex min-h-12 items-center gap-3 rounded-xl px-3 text-[17px] transition-colors";

/**
 * Die dauerhafte Seitennavigation ab 1024 px.
 *
 * Auf dieser Breite ist Platz fuer alle Ziele, also stehen auch die Eintraege
 * hier offen, die auf dem Handy hinter "Mehr" liegen. Unter 1024 px
 * uebernimmt die Bottom-Navigation und diese Spalte verlaesst das Dokument.
 */
export function ParentSidebar({ badges, gates }: ParentNavCounts) {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const isCompact = useMediaQuery(BELOW_LG);
  const { profileUrl, logout } = useShellAuth();

  if (isCompact) return null;

  // Die Handlungen "Sprache" und "Abmelden" haben in der Seitenspalte eigene
  // Plaetze: die Sprache sitzt in der Kopfzeile, Abmelden unten angeheftet.
  const secondary = PARENT_MORE_NAV.filter(
    (item): item is { kind: "link" } & ParentNavItem => item.kind === "link",
  ).filter((item) => !item.gate || gates[item.gate]);

  const renderItem = (item: ParentNavItem) => {
    const active = isParentNavActive(item.href, pathname);
    const Icon = item.icon;
    const count = item.badge ? (badges[item.badge] ?? 0) : 0;
    return (
      <li key={item.key}>
        <Link
          href={parentPath(item.href)}
          data-parent-nav-item={item.key}
          data-active={active ? "true" : "false"}
          aria-current={active ? "page" : undefined}
          className={`${ROW} ${
            active
              ? "bg-gray-100 font-semibold text-gray-900"
              : "text-gray-700 hover:bg-gray-50"
          }`}
        >
          <Icon
            size={22}
            weight={active ? PARENT_ICON_WEIGHT_ACTIVE : PARENT_ICON_WEIGHT}
            className={active ? "text-moto-green-vivid" : "text-gray-500"}
            aria-hidden="true"
          />
          <span className="flex-1">{t(item.tKey)}</span>
          {count > 0 && (
            <NotificationBadge
              count={count}
              tone="parents"
              ariaLabel={`${count} ungelesen`}
            />
          )}
        </Link>
      </li>
    );
  };

  return (
    <aside className="w-[264px] shrink-0 border-r border-gray-200 bg-white">
      <div className="sticky top-16 flex h-[calc(100vh-4rem)] flex-col">
        <nav aria-label={t("mainNav")} className="flex-1 overflow-y-auto p-3">
          <ul className="space-y-1">{PARENT_PRIMARY_NAV.map(renderItem)}</ul>
          {secondary.length > 0 && (
            <ul className="mt-3 space-y-1 border-t border-gray-200 pt-3">
              {secondary.map(renderItem)}
            </ul>
          )}
        </nav>

        <div className="space-y-1 border-t border-gray-200 p-3">
          {profileUrl && (
            <Link
              href={profileUrl}
              className={`${ROW} text-gray-700 hover:bg-gray-50`}
            >
              <UserCircle
                size={22}
                weight={PARENT_ICON_WEIGHT}
                className="text-gray-500"
                aria-hidden="true"
              />
              <span>{t("settings")}</span>
            </Link>
          )}
          <button
            type="button"
            onClick={() => void logout()}
            className={`${ROW} w-full text-left text-gray-700 hover:bg-gray-50`}
          >
            <SignOut
              size={22}
              weight={PARENT_ICON_WEIGHT}
              className="text-gray-500"
              aria-hidden="true"
            />
            <span>{t("logout")}</span>
          </button>
        </div>
      </div>
    </aside>
  );
}
