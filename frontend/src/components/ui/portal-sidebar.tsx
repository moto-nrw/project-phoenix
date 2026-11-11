"use client";

import {
  useEffect,
  useId,
  useRef,
  type ButtonHTMLAttributes,
  type ReactNode,
} from "react";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import type { NavigationLinkProps } from "~/components/ui/navigation-link";
import {
  type PortalSidebarTopOffset,
  SIDEBAR_ICON_CLASSES,
  SIDEBAR_NAV_GAP,
  SIDEBAR_NAV_PADDING,
  SIDEBAR_WIDTH_COLLAPSED,
  SIDEBAR_WIDTH_EXPANDED,
  SIDEBAR_WIDTH_TRANSITION,
} from "~/components/ui/sidebar-geometry";
import {
  SidebarRowButton,
  SidebarRowLink,
  SidebarSectionHeading,
  type SidebarRowBadge,
} from "~/components/ui/sidebar-row";
import type { MotoConcept } from "~/lib/moto-concepts";

const STICKY_POSITION: Readonly<Record<PortalSidebarTopOffset, string>> = {
  73: "top-[73px] h-[calc(100vh-73px)]",
  121: "top-[121px] h-[calc(100vh-121px)]",
  153: "top-[153px] h-[calc(100vh-153px)]",
};

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
function useFitSidebarToWindow(topOffset: PortalSidebarTopOffset) {
  const asideRef = useRef<HTMLElement>(null);
  const innerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const aside = asideRef.current;
    if (!aside) return;

    // Liest über die Refs, damit der Fenster-Listener den Baum nicht festhält.
    const fit = () => {
      const measured = asideRef.current;
      const inner = innerRef.current;
      if (!measured || !inner) return;
      const { top } = measured.getBoundingClientRect();
      inner.style.height = top > topOffset ? `calc(100vh - ${top}px)` : "";
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
  }, [topOffset]);

  return { asideRef, innerRef };
}

/**
 * Die Hülle jeder Seitenleiste: die Spalte ab 1024 px und darin der klebende
 * Bereich. `PortalSidebar` füllt sie mit Listen. Die Tenant-App füllt sie selbst,
 * weil ihre Akkordeons und der Klappzustand am Fachbereich hängen; `collapsed`
 * schaltet dafür die Breite samt Bewegung (#2825, #2923): Hülle und klebender
 * Bereich gleiten gemeinsam, dieselben Zeilen wandern mit der Kante. Die Hülle
 * trägt kein `overflow-hidden`, das hebelte das Kleben aus.
 *
 * Unter 1024 px übernimmt die mobile Leiste des Portals; CSS blendet die
 * Spalte dort bereits beim ersten Paint aus, ohne Hydrationssprung.
 */
export function PortalSidebarFrame({
  children,
  topOffset = 73,
  collapsed,
  className = "",
}: {
  readonly children: ReactNode;
  readonly topOffset?: PortalSidebarTopOffset;
  /** Ohne Wert hat die Leiste eine feste Breite und ist nicht einklappbar. */
  readonly collapsed?: boolean;
  readonly className?: string;
}) {
  const { asideRef, innerRef } = useFitSidebarToWindow(topOffset);
  const collapsible = collapsed !== undefined;
  const width = collapsed ? SIDEBAR_WIDTH_COLLAPSED : SIDEBAR_WIDTH_EXPANDED;
  const motion = collapsible ? SIDEBAR_WIDTH_TRANSITION : "";
  return (
    <aside
      ref={asideRef}
      className={`hidden ${width} shrink-0 border-r border-gray-200/70 bg-white/95 ${motion} lg:block ${className}`}
    >
      <div
        ref={innerRef}
        className={`sticky ${STICKY_POSITION[topOffset]} flex flex-col ${
          collapsible ? `${width} ${motion}` : ""
        }`}
      >
        {children}
      </div>
    </aside>
  );
}

/**
 * Die feste Seitenleiste eines Portals mit eigener Navigation ab 1024 px:
 * oben die Ziele, unten abgetrennt die Hilfe. Schul-, Träger- und Eltern-Portal
 * (#5145, #5603) teilen sie; die Ziele, ihre Freigabe und ihren Aktivzustand
 * bestimmt jedes Portal selbst. Abstände und Zeilen sind die der
 * Tenant-Leiste (#5975), nur ohne Einklappen.
 *
 * Ein Portal kann den Inhalt an derselben Stelle wechseln (Träger-Portal,
 * #5685): Position und Breite bleiben, nur Einträge und `navLabel` ändern sich.
 */
export function PortalSidebar({
  children,
  navLabel = "Hauptnavigation",
  secondary,
  secondaryLabel = "Hilfe",
  topOffset,
  className,
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
  readonly topOffset?: PortalSidebarTopOffset;
  readonly className?: string;
}) {
  return (
    <PortalSidebarFrame topOffset={topOffset} className={className}>
      <nav
        aria-label={navLabel}
        className={`flex-1 overflow-y-auto ${SIDEBAR_NAV_PADDING}`}
      >
        <ul className={SIDEBAR_NAV_GAP}>{children}</ul>
      </nav>

      {secondary ? (
        <nav
          aria-label={secondaryLabel}
          className={`border-t border-gray-200 ${SIDEBAR_NAV_PADDING}`}
        >
          <ul className={SIDEBAR_NAV_GAP}>{secondary}</ul>
        </nav>
      ) : null}
    </PortalSidebarFrame>
  );
}

/**
 * Eine benannte Gruppe von Einträgen in der `PortalSidebar`, mit derselben
 * Überschrift wie die Bereiche der Tenant-Leiste (#5975).
 */
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
    <li className="pt-4 first:pt-0">
      <SidebarSectionHeading id={labelId}>{label}</SidebarSectionHeading>
      <ul aria-labelledby={labelId} className={SIDEBAR_NAV_GAP}>
        {children}
      </ul>
    </li>
  );
}

type PortalSidebarLinkProps = Omit<
  NavigationLinkProps,
  "children" | "className" | "aria-current" | "target" | "rel" | "title"
> & {
  readonly label: string;
  readonly concept: MotoConcept;
  /** Ein anderes Symbol im Ton von `concept`. */
  readonly iconConcept?: MotoConcept;
  readonly active: boolean;
  /** Ziele außerhalb des Portals (die Hilfe) öffnen in einem neuen Tab. */
  readonly newTab?: boolean;
  /** Ein Zähler hinter dem Namen. */
  readonly badge?: SidebarRowBadge | null;
};

/** Ein Eintrag der `PortalSidebar`: Icon des Konzepts, Name, Aktivzustand. */
export function PortalSidebarLink({
  concept,
  iconConcept,
  active,
  ...rowProps
}: PortalSidebarLinkProps) {
  return (
    <li>
      <SidebarRowLink
        {...rowProps}
        active={active}
        icon={
          <MotoNavIcon
            scene
            concept={concept}
            iconConcept={iconConcept}
            active={active}
            className={SIDEBAR_ICON_CLASSES}
          />
        }
      />
    </li>
  );
}

/** Ein Eintrag der `PortalSidebar`, der eine Aktion auslöst (Abmelden). */
export function PortalSidebarButton({
  label,
  concept,
  ...buttonProps
}: Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children" | "className" | "type" | "title"
> & {
  readonly label: string;
  readonly concept: MotoConcept;
}) {
  return (
    <li>
      <SidebarRowButton
        {...buttonProps}
        label={label}
        icon={
          <MotoNavIcon
            scene
            concept={concept}
            active={false}
            className={SIDEBAR_ICON_CLASSES}
          />
        }
      />
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
