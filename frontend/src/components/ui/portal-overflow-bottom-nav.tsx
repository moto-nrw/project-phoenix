"use client";

import type { ReactNode } from "react";

import { useState } from "react";
import {
  PortalBottomNav,
  PortalBottomNavLink,
  PortalNavSheetLink,
} from "~/components/ui/portal-bottom-nav";
import { PortalMoreBottomNav } from "~/components/ui/portal-more-bottom-nav";
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
  moreConcept,
  moreLabel = "Mehr",
  moreDescription = "Weitere Bereiche",
  activeInMore = false,
  moreIcon,
}: {
  readonly items: readonly PortalOverflowNavItem[];
  /** Das Attribut, das jeden Eintrag für Touren und Tests benennt. */
  readonly navItemAttribute: `data-${string}`;
  /** Das Bild des Felds „Mehr“ (`moreConcept` aus `~/lib/concepts/more`). */
  readonly moreConcept: MotoConcept;
  readonly moreLabel?: string;
  readonly moreDescription?: string;
  /**
   * Opt-in: ist eine Seite hinter „Mehr“ offen, zeigt das Feld „Mehr“ neben
   * seinen Punkten ihren Namen; der zugängliche Name lautet „Mehr: …“. Sonst
   * stünde auf jeder dieser Seiten nur ein graues „Mehr“ markiert, und ohne
   * die Punkte sähe das Feld nicht mehr nach den übrigen Bereichen aus.
   */
  readonly activeInMore?: boolean;
  /**
   * Opt-in: ein eigenes Symbol für „Mehr“ statt der Punkte, etwa das
   * Listen-Symbol des OGS-Portals. Ohne Angabe bleiben die Punkte.
   */
  readonly moreIcon?: ReactNode;
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

  const barLinks = barItems.map((item) => (
    <PortalBottomNavLink
      key={item.key}
      {...linkProps(item)}
      shortLabel={item.shortLabel}
      paddingClassName={PADDING}
      labelClassName={LABEL}
      shrinkActive
    />
  ));

  const moreAttribute: Record<`data-${string}`, string> = {
    [navItemAttribute]: "more",
  };

  if (!crowded) {
    return <PortalBottomNav listClassName="gap-0">{barLinks}</PortalBottomNav>;
  }

  const shownInMore = activeInMore
    ? moreItems.find((item) => item.active)
    : undefined;

  return (
    <PortalMoreBottomNav
      listClassName="gap-0"
      open={moreOpen}
      onOpenChange={setMoreOpen}
      moreItemActive={moreItems.some((item) => item.active)}
      more={{
        ...moreAttribute,
        label: shownInMore ? `${moreLabel}: ${shownInMore.label}` : moreLabel,
        shortLabel: shownInMore
          ? (shownInMore.shortLabel ?? shownInMore.label)
          : undefined,
        // The dots stay: beside the open page's name they keep saying that
        // this field opens the other areas.
        ...(moreIcon ? { icon: moreIcon } : { concept: moreConcept }),
        paddingClassName: PADDING,
        labelClassName: LABEL,
        shrinkActive: true,
      }}
      sheetTitle={moreLabel}
      sheetDescription={moreDescription}
      sheet={
        // Auf einem quer gehaltenen Telefon ist die Schublade niedriger als
        // die Liste: die Liste scrollt in ihr, sonst lägen die unteren Ziele
        // außer Reichweite.
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
      }
    >
      {barLinks}
    </PortalMoreBottomNav>
  );
}
