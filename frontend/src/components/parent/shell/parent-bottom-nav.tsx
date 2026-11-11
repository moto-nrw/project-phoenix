"use client";

import { usePathname } from "next/navigation";
import { useTranslations } from "next-intl";
import { useRef, useState } from "react";
import { LogoutModal } from "~/components/auth/logout-modal";
import { NotificationBadge } from "~/components/ui/notification-badge";
import {
  PortalBottomNavLink,
  PortalNavSheetButton,
  PortalNavSheetLink,
  usePortalBottomNavIndicator,
} from "~/components/ui/portal-bottom-nav";
import { PortalMoreBottomNav } from "~/components/ui/portal-more-bottom-nav";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { parentPath } from "~/lib/parent-url";
import { isParentNavActive, isParentSettingsPath } from "./parent-nav-active";
import { ParentSettingsBottomNav } from "./parent-settings-nav";
import {
  PARENT_MORE_BADGE_SOURCES,
  PARENT_MORE_ENTRY,
  PARENT_MORE_NAV,
  PARENT_PRIMARY_NAV,
  type ParentNavBadgeSource,
  type ParentNavGate,
} from "./parent-nav-items";
import { groupsConcept } from "~/lib/concepts/groups";

export interface ParentNavCounts {
  readonly badges: Readonly<Record<ParentNavBadgeSource, number>>;
  readonly gates: Readonly<Record<ParentNavGate, boolean>>;
  readonly childCount: number;
}

const ITEM_PADDING = "px-3";
const ITEM_LABEL = "text-sm font-semibold whitespace-nowrap";

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
/**
 * Ab diesem Eintrag laeuft eine Trennlinie: darunter steht, was nicht zum
 * Alltag gehoert. Die Desktop-Seitenleiste zieht dieselbe Grenze -- dort
 * beginnt bei der Hilfe die Region `Kontonavigation`.
 */
const PINNED_GROUP_START = "help";

export function ParentBottomNav(props: ParentNavCounts) {
  const pathname = usePathname();
  // In den Einstellungen folgt die Leiste der Seitenleiste (#5750).
  return isParentSettingsPath(pathname) ? (
    <ParentSettingsBottomNav />
  ) : (
    <ParentMainBottomNav {...props} />
  );
}

function ParentMainBottomNav({ badges, gates, childCount }: ParentNavCounts) {
  const t = useTranslations("parentNav");
  const pathname = usePathname();
  const isCompact = useMediaQuery(BELOW_LG);
  const [moreOpen, setMoreOpen] = useState(false);
  const [logoutModalOpen, setLogoutModalOpen] = useState(false);

  const navRefs = useRef<(HTMLAnchorElement | null)[]>([]);
  const moreButtonRef = useRef<HTMLButtonElement | null>(null);

  const moreCount = PARENT_MORE_BADGE_SOURCES.reduce(
    (sum, source) => sum + (badges[source] ?? 0),
    0,
  );
  const moreItems = PARENT_MORE_NAV.filter(
    (item) => item.kind === "action" || !item.gate || gates[item.gate],
  );

  // Eine Unterseite von „Mehr“ ist offen (Elternbriefe, Essensplan): dann
  // leuchtet „Mehr“, sonst wüsste ein Elternteil nicht mehr, wo es ist.
  const moreItemActive = moreItems.some(
    (item) => item.kind === "link" && isParentNavActive(item.href, pathname),
  );

  // Die Fläche liegt unter dem aktiven Feld, ohne aktives Feld unter „Mehr".
  const indicator = usePortalBottomNavIndicator(() => {
    const activeIndex = PARENT_PRIMARY_NAV.findIndex((item) =>
      isParentNavActive(item.href, pathname),
    );
    return activeIndex === -1
      ? moreButtonRef.current
      : (navRefs.current[activeIndex] ?? null);
  }, `${pathname}|${isCompact}|${moreOpen}|${moreItemActive}`);

  return (
    <>
      <PortalMoreBottomNav
        label={t("mainNav")}
        listClassName="gap-1"
        fade={false}
        indicator={indicator}
        open={moreOpen}
        onOpenChange={setMoreOpen}
        moreItemActive={moreItemActive}
        more={{
          ref: moreButtonRef,
          label: t(PARENT_MORE_ENTRY.tKey),
          concept: PARENT_MORE_ENTRY.concept,
          "data-parent-nav-item": PARENT_MORE_ENTRY.key,
          paddingClassName: ITEM_PADDING,
          labelClassName: ITEM_LABEL,
          quiet: true,
          badge: (
            <NavBadge
              count={moreCount}
              ariaLabel={t("noticeCount", { count: moreCount })}
            />
          ),
        }}
        sheetTitle={t("more")}
        sheetDescription={t("more")}
        sheetMounted={!logoutModalOpen}
        sheet={
          <ul className="space-y-2 px-4 pt-6 pb-[calc(2rem+env(safe-area-inset-bottom))]">
            {moreItems.map((item) => {
              if (item.kind === "action") {
                return (
                  <li key={item.key}>
                    <PortalNavSheetButton
                      label={t(item.tKey)}
                      concept={item.concept}
                      data-parent-nav-item={item.key}
                      onClick={() => {
                        setMoreOpen(false);
                        setLogoutModalOpen(true);
                      }}
                    />
                  </li>
                );
              }

              const active =
                !item.external && isParentNavActive(item.href, pathname);
              const count = item.badge ? (badges[item.badge] ?? 0) : 0;
              return (
                <li
                  key={item.key}
                  data-parent-nav-group={
                    item.key === PINNED_GROUP_START ? "account" : undefined
                  }
                  className={
                    item.key === PINNED_GROUP_START
                      ? "mt-5 border-t border-gray-200 pt-5"
                      : undefined
                  }
                >
                  <PortalNavSheetLink
                    href={item.external ? item.href : parentPath(item.href)}
                    label={t(item.tKey)}
                    concept={item.concept}
                    active={active}
                    newTab={item.external}
                    data-parent-nav-item={item.key}
                    onClick={() => setMoreOpen(false)}
                    badge={
                      count > 0 ? (
                        <NotificationBadge
                          count={count}
                          tone="parents"
                          ariaLabel={t(
                            item.badge === "news" ? "openCount" : "unreadCount",
                            { count },
                          )}
                        />
                      ) : null
                    }
                  />
                </li>
              );
            })}
          </ul>
        }
      >
        {PARENT_PRIMARY_NAV.map((item, index) => {
          const count = item.badge ? (badges[item.badge] ?? 0) : 0;
          const label =
            item.key === "children"
              ? t(childCount === 1 ? "childSingle" : "childrenMultiple")
              : t(item.tKey);
          return (
            <PortalBottomNavLink
              key={item.key}
              href={parentPath(item.href)}
              ref={(el: HTMLAnchorElement | null) => {
                navRefs.current[index] = el;
              }}
              data-parent-nav-item={item.key}
              label={label}
              concept={item.concept}
              iconConcept={
                item.key === "children" && childCount !== 1
                  ? groupsConcept
                  : undefined
              }
              active={isParentNavActive(item.href, pathname)}
              paddingClassName={ITEM_PADDING}
              labelClassName={ITEM_LABEL}
              quiet
              badge={
                <NavBadge
                  count={count}
                  ariaLabel={t("unreadCount", { count })}
                />
              }
            />
          );
        })}
      </PortalMoreBottomNav>
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
