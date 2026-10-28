import type { ReactNode } from "react";
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
}: {
  /** Die Einträge der Hauptnavigation (`PortalSidebarLink`). */
  readonly children: ReactNode;
  /** Die Einträge unter der Trennlinie, heute die Hilfe. */
  readonly secondary: ReactNode;
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
          aria-label="Hilfe"
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
