"use client";

import type { ReactNode } from "react";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink, type NavLinkProps } from "~/components/ui/nav-link";
import {
  PORTAL_ICON_CLASSES,
  portalSidebarRowClasses,
} from "~/components/dashboard/sidebar-geometry";
import type { MotoConcept } from "~/lib/moto-concepts";

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
   * Abschnitt „Einstellungen" (`SidebarAccordionSection`).
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
