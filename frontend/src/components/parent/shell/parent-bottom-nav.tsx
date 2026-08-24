"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useEffect, useRef, useState } from "react";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { LogoutModal } from "~/components/ui/logout-modal";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { parentPath } from "~/lib/parent-url";
import { ChevronRight } from "lucide-react";
import { isParentNavActive } from "./parent-nav-active";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import {
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
  readonly childCount: number;
}

/** Wie in dashboard/mobile-bottom-nav.tsx: erst messen, dann animieren. */
const INITIAL_MOUNT_DELAY_MS = 100;

const ITEM =
  "relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-3 py-2.5 transition-colors duration-200";
const ITEM_ACTIVE = "bg-gray-100 text-gray-900";
const ITEM_IDLE = "text-gray-400 hover:text-gray-600";

function NavBadge({
  count,
  ariaLabel,
}: {
  readonly count: number;
  readonly ariaLabel: string;
}) {
  return (
    <NotificationBadge
      count={count}
      tone="parents"
      size="sm"
      ariaLabel={ariaLabel}
      className="absolute -top-1 -right-2"
    />
  );
}

/**
 * Die mobile Hauptnavigation der Eltern-App: vier Alltagsziele und "Mehr".
 *
 * Ab 1024 px uebernimmt die Seitennavigation. CSS blendet diese Leiste dort
 * bereits beim ersten Paint aus, damit beim Hydrieren keine Navigation
 * sichtbar hinein- oder herausspringt.
 */
export function ParentBottomNav({
  badges,
  gates,
  childCount,
}: ParentNavCounts) {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const isCompact = useMediaQuery(BELOW_LG);
  const [moreOpen, setMoreOpen] = useState(false);
  const [logoutModalOpen, setLogoutModalOpen] = useState(false);

  const navRefs = useRef<(HTMLAnchorElement | null)[]>([]);
  const moreButtonRef = useRef<HTMLButtonElement | null>(null);
  const [indicatorStyle, setIndicatorStyle] = useState({ width: 0, left: 0 });
  const [indicatorVisible, setIndicatorVisible] = useState(false);
  const isInitialMount = useRef(true);

  const moreCount = PARENT_MORE_BADGE_SOURCES.reduce(
    (sum, source) => sum + (badges[source] ?? 0),
    0,
  );
  const moreItems = PARENT_MORE_NAV.filter(
    (item) => item.kind === "action" || !item.gate || gates[item.gate],
  );

  // "Mehr" leuchtet auch, wenn eine seiner Unterseiten offen ist. Ohne das
  // steht ein Elternteil bei den Elternbriefen oder dem Essensplan vor einer
  // Navigation, in der nichts markiert ist, und weiss nicht mehr, wo es ist.
  const moreActive =
    moreOpen ||
    moreItems.some(
      (item) => item.kind === "link" && isParentNavActive(item.href, pathname),
    );

  // Den Indikator unter das aktive Feld schieben. Mechanik woertlich aus
  // dashboard/mobile-bottom-nav.tsx, inklusive der kleinen Verzoegerung, die
  // dem Browser Zeit zum Setzen der Breiten laesst.
  useEffect(() => {
    const timer = setTimeout(() => {
      const activeIndex = PARENT_PRIMARY_NAV.findIndex((item) =>
        isParentNavActive(item.href, pathname),
      );
      const target =
        activeIndex === -1
          ? moreButtonRef.current
          : navRefs.current[activeIndex];
      if (target) {
        setIndicatorStyle({
          left: target.offsetLeft,
          width: target.offsetWidth,
        });
        setIndicatorVisible(true);
      } else {
        setIndicatorVisible(false);
      }
    }, 10);
    return () => clearTimeout(timer);
  }, [pathname, isCompact, moreActive]);

  // Uebergaenge erst zulassen, wenn die erste Position steht.
  useEffect(() => {
    const timer = setTimeout(() => {
      isInitialMount.current = false;
    }, INITIAL_MOUNT_DELAY_MS);
    return () => clearTimeout(timer);
  }, []);

  return (
    <>
      <nav
        aria-label={t("mainNav")}
        className="fixed inset-x-0 bottom-0 z-30 translate-y-0 transition-transform duration-150 ease-in-out lg:hidden"
      >
        <div className="px-4 pb-4">
          <div className="rounded-full border border-gray-200/50 bg-white/95 px-3 py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md">
            <ul className="relative flex items-center justify-around gap-1">
              {indicatorVisible && (
                <li
                  aria-hidden="true"
                  className={`absolute top-0 h-full rounded-full bg-gray-100 ${
                    isInitialMount.current
                      ? ""
                      : "transition-[left,width] duration-150 ease-out"
                  }`}
                  style={{
                    left: `${indicatorStyle.left}px`,
                    width: `${indicatorStyle.width}px`,
                  }}
                />
              )}

              {PARENT_PRIMARY_NAV.map((item, index) => {
                const active = isParentNavActive(item.href, pathname);
                const count = item.badge ? (badges[item.badge] ?? 0) : 0;
                const label =
                  item.key === "children"
                    ? t(childCount === 1 ? "childSingle" : "childrenMultiple")
                    : t(item.tKey);
                return (
                  <li key={item.key}>
                    <Link
                      href={parentPath(item.href)}
                      ref={(el) => {
                        navRefs.current[index] = el;
                      }}
                      data-parent-nav-item={item.key}
                      data-active={active ? "true" : "false"}
                      aria-current={active ? "page" : undefined}
                      aria-label={label}
                      className={`${ITEM} ${active ? ITEM_ACTIVE : ITEM_IDLE}`}
                    >
                      <span className="relative">
                        <MotoNavIcon
                          concept={item.concept}
                          iconConcept={
                            item.key === "children" && childCount !== 1
                              ? "groups"
                              : undefined
                          }
                          active={active}
                          className="h-5 w-5 shrink-0"
                        />
                        <NavBadge
                          count={count}
                          ariaLabel={t("unreadCount", { count })}
                        />
                      </span>
                      {active && (
                        <span className="text-sm font-semibold whitespace-nowrap">
                          {label}
                        </span>
                      )}
                    </Link>
                  </li>
                );
              })}

              <li>
                <button
                  type="button"
                  ref={moreButtonRef}
                  onClick={() => setMoreOpen(true)}
                  data-parent-nav-item={PARENT_MORE_ENTRY.key}
                  data-active={moreActive ? "true" : "false"}
                  aria-haspopup="dialog"
                  aria-expanded={moreOpen}
                  aria-label={t(PARENT_MORE_ENTRY.tKey)}
                  className={`${ITEM} ${moreActive ? ITEM_ACTIVE : ITEM_IDLE}`}
                >
                  <span className="relative">
                    <MotoNavIcon
                      concept={PARENT_MORE_ENTRY.concept}
                      active={moreActive}
                      className="h-5 w-5 shrink-0"
                    />
                    <NavBadge
                      count={moreCount}
                      ariaLabel={t("noticeCount", { count: moreCount })}
                    />
                  </span>
                  {moreActive && (
                    <span className="text-sm font-semibold whitespace-nowrap">
                      {t(PARENT_MORE_ENTRY.tKey)}
                    </span>
                  )}
                </button>
              </li>
            </ul>
          </div>
        </div>
        <div className="h-safe-area-inset-bottom bg-transparent" />
      </nav>

      {!logoutModalOpen && (
        <Drawer open={moreOpen} onOpenChange={setMoreOpen}>
          <DrawerContent className="bg-white">
            <DrawerHeader className="sr-only">
              <DrawerTitle>{t("more")}</DrawerTitle>
              <DrawerDescription>{t("more")}</DrawerDescription>
            </DrawerHeader>
            <ul className="space-y-2 px-4 pt-6 pb-[calc(2rem+env(safe-area-inset-bottom))]">
              {moreItems.map((item) => {
                if (item.kind === "action") {
                  return (
                    <li key={item.key}>
                      <button
                        type="button"
                        data-parent-nav-item={item.key}
                        onClick={() => {
                          setMoreOpen(false);
                          setLogoutModalOpen(true);
                        }}
                        className="flex w-full items-center gap-3 rounded-xl bg-gray-50 px-4 py-3 text-left text-base font-medium text-gray-900 hover:bg-gray-100 active:bg-gray-200"
                      >
                        <MotoNavIcon
                          concept={item.concept}
                          active={false}
                          className="h-5 w-5 text-gray-600"
                        />
                        {t(item.tKey)}
                      </button>
                    </li>
                  );
                }

                const active = isParentNavActive(item.href, pathname);
                const count = item.badge ? (badges[item.badge] ?? 0) : 0;
                return (
                  <li
                    key={item.key}
                    data-parent-nav-group={
                      item.key === "settings" ? "account" : undefined
                    }
                    className={
                      item.key === "settings"
                        ? "mt-5 border-t border-gray-200 pt-5"
                        : undefined
                    }
                  >
                    <Link
                      href={parentPath(item.href)}
                      data-parent-nav-item={item.key}
                      data-active={active ? "true" : "false"}
                      onClick={() => setMoreOpen(false)}
                      className={`flex items-center gap-3 rounded-xl px-4 py-3 text-base font-medium transition-colors ${
                        active
                          ? "bg-gray-100 font-semibold text-gray-900"
                          : "bg-gray-50 text-gray-900 hover:bg-gray-100 active:bg-gray-200"
                      }`}
                    >
                      <MotoNavIcon
                        concept={item.concept}
                        active={active}
                        className={`h-5 w-5 ${active ? "" : "text-gray-600"}`}
                      />
                      <span className="flex-1">{t(item.tKey)}</span>
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
                      <ChevronRight
                        className="h-5 w-5 shrink-0 text-gray-400"
                        aria-hidden="true"
                      />
                    </Link>
                  </li>
                );
              })}
            </ul>
          </DrawerContent>
        </Drawer>
      )}
      <LogoutModal
        isOpen={logoutModalOpen}
        onClose={() => {
          setLogoutModalOpen(false);
          requestAnimationFrame(() => moreButtonRef.current?.focus());
        }}
      />
    </>
  );
}
