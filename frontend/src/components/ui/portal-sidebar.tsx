"use client";

import { CaretDownIcon } from "@phosphor-icons/react";
import { useId, useState, type ReactNode } from "react";
import { Button } from "~/components/ui/button";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink, type NavLinkProps } from "~/components/ui/nav-link";
import type { MotoConceptKey } from "~/lib/moto-concepts";

const ROW =
  "flex items-center rounded-lg px-3 py-2.5 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none lg:px-4 lg:py-3 lg:text-base xl:px-3 xl:py-2.5 xl:text-sm";
const ROW_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const ROW_IDLE =
  "font-medium text-gray-600 hover:bg-gray-50 hover:text-gray-900";
const ICON =
  "mr-3 h-5 w-5 shrink-0 lg:mr-3.5 lg:h-[22px] lg:w-[22px] xl:mr-3 xl:h-5 xl:w-5";

/**
 * Die feste Seitenleiste eines Portals mit eigener Navigation ab 1024 px:
 * oben die Ziele, unten abgetrennt die Hilfe. Das Schul- und das
 * Träger-Portal (#5145) teilen sie; die Ziele, ihre Freigabe und ihren
 * Aktivzustand bestimmt jedes Portal selbst.
 *
 * Unter 1024 px übernimmt die mobile Leiste des Portals; CSS blendet die
 * Spalte dort bereits beim ersten Paint aus, ohne Hydrationssprung.
 */
export function PortalSidebar({
  children,
  secondary,
  secondaryLabel = "Hilfe",
}: {
  /** Die Einträge der Hauptnavigation (`PortalSidebarLink`). */
  readonly children: ReactNode;
  /**
   * Die Einträge unter der Trennlinie: die Hilfe, im Träger-Portal auch der
   * Abschnitt „Einstellungen" (`PortalSidebarSection`).
   */
  readonly secondary: ReactNode;
  /** Der Name des unteren Blocks für Vorleseprogramme. */
  readonly secondaryLabel?: string;
}) {
  return (
    <aside className="hidden min-h-screen w-64 shrink-0 border-r border-gray-200/70 bg-white/95 lg:block">
      <div className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col">
        <nav
          aria-label="Hauptnavigation"
          className="flex-1 overflow-y-auto p-3 lg:p-4 xl:p-3"
        >
          <ul>{children}</ul>
        </nav>

        <nav
          aria-label={secondaryLabel}
          className="border-t border-gray-200 p-3 lg:p-4 xl:p-3"
        >
          <ul>{secondary}</ul>
        </nav>
      </div>
    </aside>
  );
}

type PortalSidebarLinkProps = Omit<
  NavLinkProps,
  "children" | "className" | "aria-current" | "target" | "rel"
> & {
  readonly label: string;
  readonly concept: MotoConceptKey;
  readonly active: boolean;
  /** Ziele außerhalb des Portals (die Hilfe) öffnen in einem neuen Tab. */
  readonly newTab?: boolean;
  /** Ein Zähler hinter dem Namen. */
  readonly badge?: ReactNode;
};

/** Ein Eintrag der `PortalSidebar`: Icon des Konzepts, Name, Aktivzustand. */
export function PortalSidebarLink({
  label,
  concept,
  active,
  newTab = false,
  badge,
  ...linkProps
}: PortalSidebarLinkProps) {
  return (
    <li>
      <NavLink
        {...linkProps}
        data-active={active ? "true" : "false"}
        aria-current={active ? "page" : undefined}
        {...(newTab ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        className={`${ROW} ${active ? ROW_ACTIVE : ROW_IDLE}`}
      >
        <MotoNavIcon
          concept={concept}
          active={active}
          className={`${ICON} ${active ? "" : "text-gray-400"}`}
        />
        <span className="flex flex-1 items-center">
          {label}
          {badge}
        </span>
      </NavLink>
    </li>
  );
}

/**
 * Unterpunkte stehen unter dem Namen der Kopfzeile, nicht unter ihrem Icon:
 * Zeilenabstand + Icon + Abstand zum Namen, je Breite wie in `ROW` und `ICON`
 * (12 + 20 + 12, ab 1024 px 16 + 22 + 14, ab 1280 px wieder 12 + 20 + 12).
 */
const SUB_ROW =
  "flex items-center rounded-lg py-1.5 pr-3 pl-11 text-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none lg:pl-13 xl:pl-11";
const SUB_ROW_ACTIVE = "bg-gray-100 font-semibold text-gray-900";
const SUB_ROW_IDLE =
  "font-medium text-gray-500 hover:bg-gray-50 hover:text-gray-700";

export interface PortalSidebarSectionItem {
  readonly key: string;
  readonly href: string;
  readonly label: string;
  readonly active: boolean;
}

/**
 * Ein einklappbarer Abschnitt der `PortalSidebar` (#5242), gebaut wie der
 * Bereich „Einstellungen" unten in der Seitenleiste des Schul-Portals
 * (`dashboard/sidebar-accordion-section.tsx`): eine Kopfzeile mit Chevron,
 * darunter die Unterpunkte. Kopfzeile und Unterpunkte nehmen das Raster von
 * `PortalSidebarLink`, damit der Abschnitt neben der Hilfe auf jeder Breite
 * gleich hoch steht.
 *
 * Liegt die offene Seite im Abschnitt, klappt er auf, auch beim Wechsel aus
 * einem anderen Bereich dorthin. Zuklappen bleibt möglich; dann trägt die
 * Kopfzeile die Markierung, sonst stünde man auf einer Seite des Abschnitts
 * vor einer Leiste ohne Markierung.
 */
export function PortalSidebarSection({
  label,
  concept,
  items,
  "data-portal-nav-section": dataSection,
}: {
  readonly label: string;
  readonly concept: MotoConceptKey;
  /** Die Unterpunkte, die die Person sehen darf; leer heißt: kein Abschnitt. */
  readonly items: readonly PortalSidebarSectionItem[];
  readonly "data-portal-nav-section"?: string;
}) {
  const containsActive = items.some((item) => item.active);
  const [open, setOpen] = useState(containsActive);
  // Beim Wechsel auf eine Seite des Abschnitts aufklappen, ohne Effekt:
  // React rendert sofort neu, bevor etwas auf dem Bildschirm steht.
  const [wasActive, setWasActive] = useState(containsActive);
  if (containsActive !== wasActive) {
    setWasActive(containsActive);
    if (containsActive) setOpen(true);
  }
  const bodyId = useId();

  if (items.length === 0) return null;

  const headActive = containsActive && !open;

  return (
    <li>
      <Button
        type="button"
        variant="ghost"
        size="md"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        aria-controls={bodyId}
        data-portal-nav-section={dataSection}
        data-active={headActive ? "true" : "false"}
        className={`${ROW} ${headActive ? ROW_ACTIVE : ROW_IDLE} w-full justify-start text-left`}
      >
        <MotoNavIcon
          concept={concept}
          active={containsActive}
          className={`${ICON} ${containsActive ? "" : "text-gray-400"}`}
        />
        <span className="flex-1">{label}</span>
        <CaretDownIcon
          aria-hidden="true"
          className={`ml-2 size-4 shrink-0 text-gray-400 motion-safe:transition-transform motion-safe:duration-200 ${open ? "rotate-180" : ""}`}
        />
      </Button>

      {/* Höhenwechsel über CSS-Grid; inert hält den Fokus aus dem
          zugeklappten Abschnitt heraus. */}
      <div
        className={`grid motion-safe:transition-[grid-template-rows] motion-safe:duration-200 motion-safe:ease-in-out ${open ? "grid-rows-[1fr]" : "grid-rows-[0fr]"}`}
      >
        <div id={bodyId} className="overflow-hidden" inert={!open}>
          <ul className="space-y-0.5 py-0.5">
            {items.map((item) => (
              <li key={item.key}>
                <NavLink
                  href={item.href}
                  data-portal-nav-item={item.key}
                  data-active={item.active ? "true" : "false"}
                  aria-current={item.active ? "page" : undefined}
                  className={`${SUB_ROW} ${item.active ? SUB_ROW_ACTIVE : SUB_ROW_IDLE}`}
                >
                  {item.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </div>
      </div>
    </li>
  );
}
