"use client";

import { usePathname } from "next/navigation";
import { useSession } from "next-auth/react";
import { useState } from "react";
import { Button } from "~/components/ui/button";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import {
  PortalBottomNav,
  PortalBottomNavLink,
  PortalNavSheetLink,
  portalBottomNavItemClassName,
} from "~/components/ui/portal-bottom-nav";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerNavActive } from "./traeger-nav-active";
import {
  TRAEGER_HELP_NAV,
  traegerNavItemsFor,
  traegerSettingsNavItemsFor,
  type TraegerNavItem,
} from "./traeger-nav-items";
import { moreConcept } from "~/lib/concepts/more";

const PADDING = "px-4";
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
 * Die mobile Hauptnavigation des Träger-Portals (#5145): die geteilte
 * `PortalBottomNav` des Schul-Portals mit den Zielen, die die Rechte der
 * Person öffnen. Die Unterpunkte der Einstellungen, die die Seitenleiste
 * unten einklappt (#5242), stehen hier hinter den Arbeitsbereichen.
 *
 * Wie im Schul-Portal trägt nur das aktive Ziel seinen Namen. Eine Person mit
 * allen Rechten hat acht Seiten und die Hilfe, das passt nicht in eine
 * Zeile: dann stehen die ersten vier Ziele in der Leiste und der Rest hinter
 * „Mehr", wie in der Eltern-App.
 */
export function TraegerBottomNav() {
  const pathname = usePathname() ?? "";
  const { data: session } = useSession();
  const [moreOpen, setMoreOpen] = useState(false);

  const items: readonly BottomNavItem[] = [
    ...traegerNavItemsFor(session ?? null),
    ...traegerSettingsNavItemsFor(session ?? null),
    HELP_ITEM,
  ];
  const crowded = items.length > MAX_SLOTS;
  const barItems = crowded ? items.slice(0, MAX_SLOTS - 1) : items;
  const moreItems = crowded ? items.slice(MAX_SLOTS - 1) : [];

  const isActive = (item: BottomNavItem) =>
    item.help !== true && isTraegerNavActive(item.href, pathname);
  const linkProps = (item: BottomNavItem) => ({
    href: item.help ? item.href : traegerPath(item.href),
    label: item.label,
    concept: item.concept,
    active: isActive(item),
    newTab: item.help === true,
    "data-traeger-nav-item": item.key,
  });

  // „Mehr" leuchtet auch, wenn eine seiner Seiten offen ist; sonst stünde
  // man auf „Briefkopf" vor einer Leiste, in der nichts markiert ist.
  const moreActive = moreOpen || moreItems.some(isActive);

  return (
    <>
      <PortalBottomNav listClassName="gap-0">
        {barItems.map((item) => (
          <PortalBottomNavLink
            key={item.key}
            {...linkProps(item)}
            paddingClassName={PADDING}
            labelClassName={LABEL}
          />
        ))}
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
              className={portalBottomNavItemClassName(moreActive, PADDING)}
            >
              <MotoNavIcon
                concept={moreConcept}
                active={moreActive}
                className="h-5 w-5 shrink-0"
              />
              {moreActive ? <span className={LABEL}>Mehr</span> : null}
            </Button>
          </li>
        ) : null}
      </PortalBottomNav>

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
              {moreItems.map((item) => (
                <li
                  key={item.key}
                  // Die Hilfe ist kein Arbeitsbereich: eine Linie trennt sie
                  // ab, wie unten in der Seitenleiste.
                  className={
                    item.help ? "mt-5 border-t border-gray-200 pt-5" : undefined
                  }
                >
                  <PortalNavSheetLink
                    {...linkProps(item)}
                    onClick={() => setMoreOpen(false)}
                  />
                </li>
              ))}
            </ul>
          </DrawerContent>
        </Drawer>
      ) : null}
    </>
  );
}
