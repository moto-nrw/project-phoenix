"use client";

import { useEffect, useId, useRef, type ReactNode } from "react";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink, type NavLinkProps } from "~/components/ui/nav-link";
import {
  PORTAL_ICON_CLASSES,
  portalSidebarRowClasses,
} from "~/components/dashboard/sidebar-geometry";
import type { MotoConcept } from "~/lib/moto-concepts";

/** Ab dieser Fensterposition klebt die Leiste; die Klasse `top-[73px]` passt dazu. */
const STICKY_TOP_PX = 73;

/**
 * Hält die Unterkante der Leiste am Fensterrand, auch wenn ein Hinweis über der
 * Kopfzeile sie tiefer beginnen lässt (Testsystem, #5680). Der Hinweis scrollt
 * mit der Seite weg, deshalb folgt die Höhe der Position; CSS allein kennt
 * weder seine Höhe noch die Scrollposition. Ohne Hinweis gilt die Klasse.
 *
 * Gemessen wird im nächsten Frame: Ein Schreibzugriff direkt im
 * ResizeObserver würde Fehler „ResizeObserver loop“ auslösen. Der Beobachter
 * läuft auch, weil der Hinweis erst nach dem Laden des Trägers erscheint und
 * dabei weder Scroll- noch Resize-Ereignis auslöst.
 */
function useFitSidebarToWindow() {
  const asideRef = useRef<HTMLElement>(null);
  const innerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const aside = asideRef.current;
    const inner = innerRef.current;
    if (!aside || !inner) return;

    const fit = () => {
      const { top } = aside.getBoundingClientRect();
      inner.style.height = top > STICKY_TOP_PX ? `calc(100vh - ${top}px)` : "";
    };
    let frame = 0;
    const scheduleFit = () => {
      frame ||= requestAnimationFrame(() => {
        frame = 0;
        fit();
      });
    };

    fit();
    window.addEventListener("scroll", scheduleFit, { passive: true });
    window.addEventListener("resize", scheduleFit);
    const observer = new ResizeObserver(scheduleFit);
    observer.observe(aside);
    observer.observe(document.body);

    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("scroll", scheduleFit);
      window.removeEventListener("resize", scheduleFit);
      observer.disconnect();
    };
  }, []);

  return { asideRef, innerRef };
}

/**
 * Die feste Seitenleiste eines Portals mit eigener Navigation ab 1024 px:
 * oben die Ziele, unten abgetrennt die Hilfe. Das Schul- und das
 * Träger-Portal (#5145) teilen sie; die Ziele, ihre Freigabe und ihren
 * Aktivzustand bestimmt jedes Portal selbst.
 *
 * Unter 1024 px übernimmt die mobile Leiste des Portals; CSS blendet die
 * Spalte dort bereits beim ersten Paint aus, ohne Hydrationssprung.
 *
 * Ein Portal kann den Inhalt an derselben Stelle wechseln (Träger-Portal,
 * #5685): Position und Breite bleiben, nur Einträge und `navLabel` ändern sich.
 */
export function PortalSidebar({
  children,
  navLabel = "Hauptnavigation",
  secondary,
  secondaryLabel = "Hilfe",
}: {
  /** Die Einträge (`PortalSidebarLink`, `PortalSidebarGroup`). */
  readonly children: ReactNode;
  /** Der Name des oberen Blocks für Vorleseprogramme. */
  readonly navLabel?: string;
  /**
   * Die Einträge unter der Trennlinie: die Hilfe, im Träger-Portal auch der
   * Einstieg in die Einstellungen. Ohne Einträge fehlt der Block.
   */
  readonly secondary?: ReactNode;
  /** Der Name des unteren Blocks für Vorleseprogramme. */
  readonly secondaryLabel?: string;
}) {
  const { asideRef, innerRef } = useFitSidebarToWindow();
  return (
    <aside
      ref={asideRef}
      className="hidden w-64 shrink-0 border-r border-gray-200/70 bg-white/95 lg:block"
    >
      <div
        ref={innerRef}
        className="sticky top-[73px] flex h-[calc(100vh-73px)] flex-col"
      >
        <nav
          aria-label={navLabel}
          className="flex-1 overflow-y-auto p-3 lg:p-4 xl:p-3"
        >
          <ul>{children}</ul>
        </nav>

        {secondary ? (
          <nav
            aria-label={secondaryLabel}
            className="border-t border-gray-200 p-3 lg:p-4 xl:p-3"
          >
            <ul>{secondary}</ul>
          </nav>
        ) : null}
      </div>
    </aside>
  );
}

/** Eine benannte Gruppe von Einträgen in der `PortalSidebar`. */
export function PortalSidebarGroup({
  label,
  children,
}: {
  readonly label: string;
  /** Die Einträge der Gruppe (`PortalSidebarLink`). */
  readonly children: ReactNode;
}) {
  const labelId = useId();
  return (
    <li className="mt-4 first:mt-0">
      <span
        id={labelId}
        className="mb-1 block px-3 text-xs font-semibold tracking-wider text-gray-500 uppercase"
      >
        {label}
      </span>
      <ul aria-labelledby={labelId}>{children}</ul>
    </li>
  );
}

type PortalSidebarLinkProps = Omit<
  NavLinkProps,
  "children" | "className" | "aria-current" | "target" | "rel"
> & {
  readonly label: string;
  readonly concept: MotoConcept;
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
        className={portalSidebarRowClasses(active)}
      >
        <MotoNavIcon
          scene
          concept={concept}
          active={active}
          className={`${PORTAL_ICON_CLASSES} ${active ? "" : "text-gray-400"}`}
        />
        <span className="flex flex-1 items-center">
          {label}
          {badge}
        </span>
      </NavLink>
    </li>
  );
}

export interface PortalSettingsNavItem {
  readonly key: string;
  readonly href: string;
  readonly label: string;
  readonly concept: MotoConcept;
  readonly active: boolean;
}

/**
 * Der Inhalt der Seitenleiste in den Einstellungen (#5685, #5750): oben der
 * Rückweg zur App, darunter die Bereiche. Die Leiste behält Position und
 * Breite, nur ihr Inhalt wechselt. `groupLabel` ist die Überschrift der
 * Bereiche, `navItemAttribute` benennt jede Zeile für Touren und Tests.
 */
export function PortalSettingsSidebarEntries({
  back,
  groupLabel,
  items,
  navItemAttribute,
}: {
  readonly back: Pick<
    PortalSettingsNavItem,
    "key" | "href" | "label" | "concept"
  >;
  readonly groupLabel: string;
  readonly items: readonly PortalSettingsNavItem[];
  readonly navItemAttribute: `data-${string}`;
}) {
  return (
    <>
      <PortalSidebarLink
        href={back.href}
        label={back.label}
        concept={back.concept}
        active={false}
        {...{ [navItemAttribute]: back.key }}
      />
      <PortalSidebarGroup label={groupLabel}>
        {items.map((item) => (
          <PortalSidebarLink
            key={item.key}
            href={item.href}
            label={item.label}
            concept={item.concept}
            active={item.active}
            {...{ [navItemAttribute]: item.key }}
          />
        ))}
      </PortalSidebarGroup>
    </>
  );
}
