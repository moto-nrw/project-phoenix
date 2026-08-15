"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useState } from "react";
import { LanguageSwitcher } from "~/components/parent/language-switcher";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { parentPath } from "~/lib/parent-url";
import { useShellAuth } from "~/lib/shell-auth-context";
import { isParentNavActive } from "./parent-nav-active";
import {
  PARENT_ICON_WEIGHT,
  PARENT_ICON_WEIGHT_ACTIVE,
  PARENT_MORE_BADGE_SOURCES,
  PARENT_MORE_ENTRY,
  PARENT_MORE_NAV,
  PARENT_PRIMARY_NAV,
  type ParentNavBadgeSource,
  type ParentNavGate,
} from "./parent-nav-items";

export interface ParentNavCounts {
  readonly badges: Readonly<Record<ParentNavBadgeSource, number>>;
  readonly gates: Readonly<Record<ParentNavGate, boolean>>;
}

// Aktiv: gefuelltes Icon, gruener Balken ueber dem Ziel, halbfette Beschriftung.
// Das Gruen ist moto-green-vivid (#5F9F1B) statt #83CD2D: die helle Marken-
// stufe erreicht auf Weiss nur 1,9:1 und verschwindet auf einem Handy in der
// Sonne. Der Balken darueber traegt das reine Markengruen.
const ICON_ACTIVE = "text-moto-green-vivid";
const ICON_IDLE = "text-gray-500";

function NavBadge({ count }: { readonly count: number }) {
  return (
    <NotificationBadge
      count={count}
      tone="parents"
      size="sm"
      ariaLabel={`${count} ungelesen`}
      className="absolute -top-1 -right-2"
    />
  );
}

/**
 * Die mobile Hauptnavigation der Eltern-App: vier Alltagsziele und "Mehr".
 *
 * Ab 1024 px uebernimmt die Seitennavigation, deshalb verschwindet diese
 * Leiste dort vollstaendig aus dem Dokument statt nur ausgeblendet zu werden.
 * Beides gleichzeitig im Baum wuerde jede Beschriftung doppelt vorlesen
 * lassen und die Tabulator-Reihenfolge durch eine unsichtbare Kopie fuehren.
 */
export function ParentBottomNav({ badges, gates }: ParentNavCounts) {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const isCompact = useMediaQuery(BELOW_LG);
  const { logout } = useShellAuth();
  const [moreOpen, setMoreOpen] = useState(false);

  if (!isCompact) return null;

  const moreCount = PARENT_MORE_BADGE_SOURCES.reduce(
    (sum, source) => sum + (badges[source] ?? 0),
    0,
  );
  const moreItems = PARENT_MORE_NAV.filter(
    (item) => item.kind === "action" || !item.gate || gates[item.gate],
  );

  return (
    <>
      <nav
        aria-label={t("mainNav")}
        className="fixed inset-x-0 bottom-0 z-40 border-t border-gray-200 bg-white pb-[env(safe-area-inset-bottom)]"
      >
        <ul className="mx-auto flex max-w-2xl">
          {PARENT_PRIMARY_NAV.map((item) => {
            const active = isParentNavActive(item.href, pathname);
            const Icon = item.icon;
            const count = item.badge ? (badges[item.badge] ?? 0) : 0;
            return (
              <li key={item.key} className="flex-1">
                <Link
                  href={parentPath(item.href)}
                  data-parent-nav-item={item.key}
                  data-active={active ? "true" : "false"}
                  aria-current={active ? "page" : undefined}
                  className="relative flex min-h-14 flex-col items-center justify-center gap-1 px-1 pt-1.5 pb-1"
                >
                  <span
                    aria-hidden="true"
                    className={`absolute inset-x-3 top-0 h-[3px] rounded-b-full ${
                      active ? "bg-moto-green" : "bg-transparent"
                    }`}
                  />
                  <span className="relative">
                    <Icon
                      size={26}
                      weight={
                        active ? PARENT_ICON_WEIGHT_ACTIVE : PARENT_ICON_WEIGHT
                      }
                      className={active ? ICON_ACTIVE : ICON_IDLE}
                      aria-hidden="true"
                    />
                    <NavBadge count={count} />
                  </span>
                  <span
                    className={`text-center text-[12px] leading-4 ${
                      active
                        ? "font-semibold text-gray-900"
                        : "font-normal text-gray-600"
                    }`}
                  >
                    {t(item.tKey)}
                  </span>
                </Link>
              </li>
            );
          })}

          <li className="flex-1">
            <button
              type="button"
              onClick={() => setMoreOpen(true)}
              data-parent-nav-item={PARENT_MORE_ENTRY.key}
              data-active={moreOpen ? "true" : "false"}
              aria-haspopup="dialog"
              aria-expanded={moreOpen}
              className="relative flex min-h-14 w-full flex-col items-center justify-center gap-1 px-1 pt-1.5 pb-1"
            >
              <span
                aria-hidden="true"
                className={`absolute inset-x-3 top-0 h-[3px] rounded-b-full ${
                  moreOpen ? "bg-moto-green" : "bg-transparent"
                }`}
              />
              <span className="relative">
                <PARENT_MORE_ENTRY.icon
                  size={26}
                  weight={
                    moreOpen ? PARENT_ICON_WEIGHT_ACTIVE : PARENT_ICON_WEIGHT
                  }
                  className={moreOpen ? ICON_ACTIVE : ICON_IDLE}
                  aria-hidden="true"
                />
                <NavBadge count={moreCount} />
              </span>
              <span
                className={`text-center text-[12px] leading-4 ${
                  moreOpen
                    ? "font-semibold text-gray-900"
                    : "font-normal text-gray-600"
                }`}
              >
                {t(PARENT_MORE_ENTRY.tKey)}
              </span>
            </button>
          </li>
        </ul>
      </nav>

      <Drawer open={moreOpen} onOpenChange={setMoreOpen}>
        <DrawerContent className="bg-white">
          <DrawerHeader className="sr-only">
            <DrawerTitle>{t("more")}</DrawerTitle>
            <DrawerDescription>{t("more")}</DrawerDescription>
          </DrawerHeader>
          <ul className="space-y-1 px-4 pt-2 pb-[calc(1.5rem+env(safe-area-inset-bottom))]">
            {moreItems.map((item) => {
              const Icon = item.icon;
              if (item.kind === "action" && item.action === "language") {
                return (
                  <li
                    key={item.key}
                    data-parent-nav-item={item.key}
                    className="flex min-h-12 items-center justify-between gap-3 rounded-xl px-3"
                  >
                    <span className="flex items-center gap-3 text-[17px] text-gray-900">
                      <Icon size={22} weight={PARENT_ICON_WEIGHT} />
                      {t(item.tKey)}
                    </span>
                    <LanguageSwitcher compact />
                  </li>
                );
              }

              if (item.kind === "action") {
                return (
                  <li key={item.key}>
                    <button
                      type="button"
                      data-parent-nav-item={item.key}
                      onClick={() => void logout()}
                      className="flex min-h-12 w-full items-center gap-3 rounded-xl px-3 text-left text-[17px] text-gray-900 active:bg-gray-100"
                    >
                      <Icon
                        size={22}
                        weight={PARENT_ICON_WEIGHT}
                        className="text-gray-500"
                        aria-hidden="true"
                      />
                      {t(item.tKey)}
                    </button>
                  </li>
                );
              }

              const active = isParentNavActive(item.href, pathname);
              const count = item.badge ? (badges[item.badge] ?? 0) : 0;
              return (
                <li key={item.key}>
                  <Link
                    href={parentPath(item.href)}
                    data-parent-nav-item={item.key}
                    data-active={active ? "true" : "false"}
                    onClick={() => setMoreOpen(false)}
                    className={`flex min-h-12 items-center gap-3 rounded-xl px-3 text-[17px] ${
                      active
                        ? "bg-gray-100 font-semibold text-gray-900"
                        : "text-gray-900"
                    }`}
                  >
                    <Icon
                      size={22}
                      weight={
                        active ? PARENT_ICON_WEIGHT_ACTIVE : PARENT_ICON_WEIGHT
                      }
                      className={active ? ICON_ACTIVE : "text-gray-500"}
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
            })}
          </ul>
        </DrawerContent>
      </Drawer>
    </>
  );
}
