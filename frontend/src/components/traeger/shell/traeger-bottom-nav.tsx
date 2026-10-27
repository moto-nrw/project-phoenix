"use client";

import { ChevronRight } from "lucide-react";
import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { useState } from "react";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { Button } from "~/components/ui/button";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink } from "~/components/ui/nav-link";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerNavActive } from "./traeger-nav-active";
import {
  TRAEGER_HELP_NAV,
  traegerNavItemsFor,
  type TraegerNavItem,
} from "./traeger-nav-items";

const ITEM =
  "relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full px-4 py-2.5 transition-colors duration-200";
const ITEM_ACTIVE = "bg-gray-100 text-gray-900";
const ITEM_IDLE = "text-gray-500 hover:text-gray-700";
const LABEL =
  "hidden text-sm font-semibold whitespace-nowrap min-[480px]:inline";

/**
 * So viele Felder hat die Leiste höchstens. Fünf Icons passen auch auf ein
 * 320 px breites Telefon; ab sechs Zielen stehen die ersten vier in der
 * Leiste und der Rest hinter „Mehr".
 */
const MAX_SLOTS = 5;

type BottomNavItem = Omit<TraegerNavItem, "right"> & { readonly help?: true };

const HELP_ITEM: BottomNavItem = { ...TRAEGER_HELP_NAV, help: true };

/**
 * Die mobile Hauptnavigation des Träger-Portals (#5145).
 *
 * Wie im Schul-Portal trägt nur das aktive Ziel seinen Namen, die übrigen ihr
 * Icon und ihren Namen als aria-label. Eine Person mit allen Rechten hat
 * acht Seiten und die Hilfe, das passt nicht in eine Zeile: dann stehen die
 * ersten vier Ziele in der Leiste und der Rest hinter „Mehr", wie in der
 * Eltern-App. Ab 1024 px übernimmt die Seitennavigation; CSS blendet diese
 * Leiste dort schon beim ersten Paint aus.
 */
export function TraegerBottomNav() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();
  const [moreOpen, setMoreOpen] = useState(false);

  const items: readonly BottomNavItem[] = [
    ...traegerNavItemsFor(session ?? null),
    HELP_ITEM,
  ];
  const crowded = items.length > MAX_SLOTS;
  const barItems = crowded ? items.slice(0, MAX_SLOTS - 1) : items;
  const moreItems = crowded ? items.slice(MAX_SLOTS - 1) : [];

  const isActive = (item: BottomNavItem) =>
    item.help !== true && isTraegerNavActive(item.href, pathname);
  const href = (item: BottomNavItem) =>
    item.help ? item.href : traegerPath(item.href);
  const newTab = (item: BottomNavItem) =>
    item.help ? { target: "_blank", rel: "noopener noreferrer" } : {};

  // „Mehr" leuchtet auch, wenn eine seiner Seiten offen ist; sonst stünde
  // man auf „Briefkopf" vor einer Leiste, in der nichts markiert ist.
  const moreActive = moreOpen || moreItems.some(isActive);

  return (
    <>
      <nav
        aria-label="Hauptnavigation"
        className="fixed inset-x-0 bottom-0 z-30 lg:hidden"
      >
        {/* Verlauf auf die Seitenfarbe wie in der Leiste des Schul-Portals:
            ohne ihn liefe der Inhalt neben der schwebenden Pille sichtbar
            weiter. */}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-gradient-to-t from-gray-50 via-gray-50/90 to-transparent"
        />
        <div className="relative px-4 pb-4">
          <div className="rounded-full border border-gray-200/50 bg-white/95 px-3 py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md">
            <ul className="flex items-center justify-around gap-0">
              {barItems.map((item) => {
                const active = isActive(item);
                return (
                  <li key={item.key}>
                    <NavLink
                      href={href(item)}
                      data-traeger-nav-item={item.key}
                      data-active={active ? "true" : "false"}
                      aria-current={active ? "page" : undefined}
                      {...newTab(item)}
                      aria-label={item.label}
                      className={`${ITEM} ${active ? ITEM_ACTIVE : ITEM_IDLE}`}
                    >
                      <MotoNavIcon
                        concept={item.concept}
                        active={active}
                        className="h-5 w-5 shrink-0"
                      />
                      {active ? (
                        <span className={LABEL}>{item.label}</span>
                      ) : null}
                    </NavLink>
                  </li>
                );
              })}
              {crowded ? (
                <li>
                  <Button
                    type="button"
                    variant="ghost"
                    size="compact"
                    onClick={() => setMoreOpen(true)}
                    data-traeger-nav-item="more"
                    data-active={moreActive ? "true" : "false"}
                    aria-haspopup="dialog"
                    aria-expanded={moreOpen}
                    aria-label="Mehr"
                    className={`${ITEM} ${moreActive ? ITEM_ACTIVE : ITEM_IDLE}`}
                  >
                    <MotoNavIcon
                      concept="more"
                      active={moreActive}
                      className="h-5 w-5 shrink-0"
                    />
                    {moreActive ? <span className={LABEL}>Mehr</span> : null}
                  </Button>
                </li>
              ) : null}
            </ul>
          </div>
        </div>
        <div className="h-safe-area-inset-bottom bg-transparent" />
      </nav>

      {crowded ? (
        <Drawer open={moreOpen} onOpenChange={setMoreOpen}>
          <DrawerContent className="bg-white">
            <DrawerHeader className="sr-only">
              <DrawerTitle>Mehr</DrawerTitle>
              <DrawerDescription>Weitere Bereiche</DrawerDescription>
            </DrawerHeader>
            {/* Auf einem quer gehaltenen Telefon ist die Schublade niedriger
                als die Liste: die Liste scrollt in ihr, sonst lägen die
                unteren Ziele außer Reichweite. */}
            <ul className="min-h-0 space-y-2 overflow-y-auto px-4 pt-6 pb-[calc(2rem+env(safe-area-inset-bottom))]">
              {moreItems.map((item) => {
                const active = isActive(item);
                return (
                  <li
                    key={item.key}
                    // Die Hilfe ist kein Arbeitsbereich: eine Linie trennt sie
                    // ab, wie unten in der Seitenleiste.
                    className={
                      item.help
                        ? "mt-5 border-t border-gray-200 pt-5"
                        : undefined
                    }
                  >
                    <NavLink
                      href={href(item)}
                      {...newTab(item)}
                      data-traeger-nav-item={item.key}
                      data-active={active ? "true" : "false"}
                      aria-current={active ? "page" : undefined}
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
                      <span className="flex-1">{item.label}</span>
                      <ChevronRight
                        className="h-5 w-5 shrink-0 text-gray-400"
                        aria-hidden="true"
                      />
                    </NavLink>
                  </li>
                );
              })}
            </ul>
          </DrawerContent>
        </Drawer>
      ) : null}
    </>
  );
}
