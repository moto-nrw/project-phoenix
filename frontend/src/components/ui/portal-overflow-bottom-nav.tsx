"use client";

import { useState } from "react";
import {
  Drawer,
  DrawerContent,
  DrawerDescription,
  DrawerHeader,
  DrawerTitle,
} from "~/components/ui/drawer";
import {
  PortalBottomNav,
  PortalBottomNavButton,
  PortalBottomNavLink,
  PortalNavSheetLink,
} from "~/components/ui/portal-bottom-nav";
import { moreConcept } from "~/lib/concepts/more";
import type { MotoConcept } from "~/lib/moto-concepts";

const PADDING = "px-4";
// Das aktive Ziel trägt seinen Namen auf jeder Breite, wie in der Leiste des
// Personal-Portals: es darf schrumpfen, der Name wird notfalls gekürzt.
const LABEL = "truncate text-sm font-semibold whitespace-nowrap";

/**
 * So viele Felder hat die Leiste höchstens. Fünf Icons passen auch auf ein
 * 320 px breites Telefon; ab sechs Zielen stehen die ersten vier in der
 * Leiste und der Rest hinter „Mehr".
 */
const MAX_SLOTS = 5;

export interface PortalOverflowNavItem {
  /** Stabiler Schlüssel für Listen und Tests. */
  readonly key: string;
  /** Die fertige Adresse des Ziels. */
  readonly href: string;
  readonly label: string;
  /**
   * A shorter name for the bar, where the active target's name has about
   * 80 px; the „Mehr“ sheet keeps `label`.
   */
  readonly shortLabel?: string;
  readonly concept: MotoConcept;
  readonly active: boolean;
  /** Ziele außerhalb des Portals (die Hilfe) öffnen in einem neuen Tab. */
  readonly newTab?: boolean;
  /** Eine Linie trennt den Eintrag im „Mehr"-Blatt von den Arbeitsbereichen. */
  readonly separated?: boolean;
}

/**
 * Die mobile Leiste für eine Zielliste, deren Länge nicht feststeht: Passen die
 * Ziele nicht in eine Zeile, stehen die ersten vier in der Leiste und der Rest
 * hinter „Mehr", wie in der Eltern-App. Wie im Schul-Portal trägt nur das aktive
 * Ziel seinen Namen. Das Träger-Portal führt damit seine ganze Navigation, die
 * übrigen Portale ihre Einstellungen (#5750).
 */
export function PortalOverflowBottomNav({
  items,
  navItemAttribute,
  moreLabel = "Mehr",
  moreDescription = "Weitere Bereiche",
  activeInMore = false,
}: {
  readonly items: readonly PortalOverflowNavItem[];
  /** Das Attribut, das jeden Eintrag für Touren und Tests benennt. */
  readonly navItemAttribute: `data-${string}`;
  readonly moreLabel?: string;
  readonly moreDescription?: string;
  /**
   * Opt-in: ist eine Seite hinter „Mehr“ offen, zeigt das Feld „Mehr“ neben
   * seinen Punkten ihren Namen; der zugängliche Name lautet „Mehr: …“. Sonst
   * stünde auf jeder dieser Seiten nur ein graues „Mehr“ markiert, und ohne
   * die Punkte sähe das Feld nicht mehr nach den übrigen Bereichen aus.
   */
  readonly activeInMore?: boolean;
}) {
  const [moreOpen, setMoreOpen] = useState(false);

  const crowded = items.length > MAX_SLOTS;
  const barItems = crowded ? items.slice(0, MAX_SLOTS - 1) : items;
  const moreItems = crowded ? items.slice(MAX_SLOTS - 1) : [];

  const linkProps = (item: PortalOverflowNavItem) => ({
    href: item.href,
    label: item.label,
    concept: item.concept,
    active: item.active,
    newTab: item.newTab === true,
    [navItemAttribute]: item.key,
  });

  // „Mehr" leuchtet auch, wenn eine seiner Seiten offen ist; sonst stünde
  // man auf einer Seite vor einer Leiste, in der nichts markiert ist.
  const moreActive = moreOpen || moreItems.some((item) => item.active);
  const shownInMore = activeInMore
    ? moreItems.find((item) => item.active)
    : undefined;

  return (
    <>
      <PortalBottomNav listClassName="gap-0">
        {barItems.map((item) => (
          <PortalBottomNavLink
            key={item.key}
            {...linkProps(item)}
            shortLabel={item.shortLabel}
            paddingClassName={PADDING}
            labelClassName={LABEL}
            shrinkActive
          />
        ))}
        {crowded ? (
          <PortalBottomNavButton
            label={
              shownInMore ? `${moreLabel}: ${shownInMore.label}` : moreLabel
            }
            shortLabel={
              shownInMore
                ? (shownInMore.shortLabel ?? shownInMore.label)
                : undefined
            }
            // The dots stay: beside the open page's name they keep saying
            // that this field opens the other areas.
            concept={moreConcept}
            active={moreActive}
            onClick={() => setMoreOpen(true)}
            {...{ [navItemAttribute]: "more" }}
            aria-haspopup="dialog"
            aria-expanded={moreOpen}
            paddingClassName={PADDING}
            labelClassName={LABEL}
            shrinkActive
          />
        ) : null}
      </PortalBottomNav>

      {crowded ? (
        <Drawer open={moreOpen} onOpenChange={setMoreOpen}>
          <DrawerContent className="bg-white">
            <DrawerHeader className="sr-only">
              <DrawerTitle>{moreLabel}</DrawerTitle>
              <DrawerDescription>{moreDescription}</DrawerDescription>
            </DrawerHeader>
            {/* Auf einem quer gehaltenen Telefon ist die Schublade niedriger
                als die Liste: die Liste scrollt in ihr, sonst lägen die
                unteren Ziele außer Reichweite. */}
            <ul className="min-h-0 space-y-2 overflow-y-auto px-4 pt-6 pb-[calc(2rem+env(safe-area-inset-bottom))]">
              {moreItems.map((item) => (
                <li
                  key={item.key}
                  className={
                    item.separated
                      ? "mt-5 border-t border-gray-200 pt-5"
                      : undefined
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
