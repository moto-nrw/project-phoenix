import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink, type NavLinkProps } from "~/components/ui/nav-link";
import type { MotoConcept } from "~/lib/moto-concepts";

const ITEM =
  "relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full py-2.5 transition-colors duration-200";
const ITEM_ACTIVE = "bg-gray-100 text-gray-900";
const ITEM_IDLE = "text-gray-500 hover:text-gray-700";

/**
 * Die Klassen eines Felds der schwebenden Leiste. Ein Feld, das kein Link
 * ist (das „Mehr" des Träger-Portals), nimmt sie für denselben Look.
 */
export function portalBottomNavItemClassName(
  active: boolean,
  paddingClassName: string,
): string {
  return `${ITEM} ${paddingClassName} ${active ? ITEM_ACTIVE : ITEM_IDLE}`;
}

/**
 * Die schwebende mobile Leiste eines Portals mit eigener Navigation unter
 * 1024 px, geteilt vom Schul- und vom Träger-Portal (#5145). Ab 1024 px
 * übernimmt die Seitenleiste; CSS blendet die Leiste dort schon beim ersten
 * Paint aus.
 */
export function PortalBottomNav({
  children,
  listClassName,
}: {
  /** Die Felder (`PortalBottomNavLink`), jedes in einem `<li>`. */
  readonly children: ReactNode;
  /** Abstand der Felder, je nachdem, wie voll die Leiste ist. */
  readonly listClassName: string;
}) {
  return (
    <nav
      aria-label="Hauptnavigation"
      className="fixed inset-x-0 bottom-0 z-30 lg:hidden"
    >
      {/* Die Leiste schwebt, ihr Rand ist also durchsichtig — ohne diesen
          Verlauf laeuft der Inhalt daneben und darunter sichtbar weiter und
          endet mitten in einer Zeile. Der Verlauf geht auf die Seitenfarbe
          (gray-50) und nicht auf Weiss, damit er auf dem gepunkteten
          Hintergrund keine Kante zieht. */}
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-gradient-to-t from-gray-50 via-gray-50/90 to-transparent"
      />
      <div className="relative px-4 pb-4">
        <div className="rounded-full border border-gray-200/50 bg-white/95 px-3 py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md">
          <ul className={`flex items-center justify-around ${listClassName}`}>
            {children}
          </ul>
        </div>
      </div>
      <div className="h-safe-area-inset-bottom bg-transparent" />
    </nav>
  );
}

type PortalNavLinkProps = Omit<
  NavLinkProps,
  "children" | "className" | "aria-current" | "aria-label" | "target" | "rel"
> & {
  readonly label: string;
  readonly concept: MotoConcept;
  readonly active: boolean;
  /** Ziele außerhalb des Portals (die Hilfe) öffnen in einem neuen Tab. */
  readonly newTab?: boolean;
  /** Ein Zähler am Feld. */
  readonly badge?: ReactNode;
};

/**
 * Ein Feld der `PortalBottomNav`: nur das aktive Ziel trägt seinen Namen,
 * die übrigen ihr Icon und den Namen als aria-label.
 */
export function PortalBottomNavLink({
  label,
  concept,
  active,
  newTab = false,
  badge,
  paddingClassName,
  labelClassName,
  ...linkProps
}: PortalNavLinkProps & {
  readonly paddingClassName: string;
  /** Ab welcher Breite der Name des aktiven Ziels dasteht. */
  readonly labelClassName: string;
}) {
  return (
    <li>
      <NavLink
        {...linkProps}
        data-active={active ? "true" : "false"}
        aria-current={active ? "page" : undefined}
        {...(newTab ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        aria-label={label}
        className={portalBottomNavItemClassName(active, paddingClassName)}
      >
        <span className="relative inline-flex">
          <MotoNavIcon
            concept={concept}
            active={active}
            className="h-5 w-5 shrink-0"
          />
          {badge}
        </span>
        {active ? <span className={labelClassName}>{label}</span> : null}
      </NavLink>
    </li>
  );
}

/**
 * Eine Zeile im „Mehr"-Blatt der mobilen Leiste (Eltern-App, Träger-Portal):
 * Icon, Name, Zähler, Pfeil. Das `<li>` und eine Gruppentrennung legt die
 * Leiste selbst darum.
 */
export function PortalNavSheetLink({
  label,
  concept,
  active,
  newTab = false,
  badge,
  ...linkProps
}: PortalNavLinkProps) {
  return (
    <NavLink
      {...linkProps}
      {...(newTab ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      data-active={active ? "true" : "false"}
      aria-current={active ? "page" : undefined}
      className={`flex items-center gap-3 rounded-xl px-4 py-3 text-base font-medium transition-colors ${
        active
          ? "bg-gray-100 font-semibold text-gray-900"
          : "bg-gray-50 text-gray-900 hover:bg-gray-100 active:bg-gray-200"
      }`}
    >
      <MotoNavIcon
        concept={concept}
        active={active}
        className={`h-5 w-5 ${active ? "" : "text-gray-600"}`}
      />
      <span className="flex-1">{label}</span>
      {badge}
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </NavLink>
  );
}
