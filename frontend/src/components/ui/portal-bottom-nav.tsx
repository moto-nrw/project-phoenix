"use client";

import { ChevronRight } from "lucide-react";
import {
  useEffect,
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type ReactNode,
  type Ref,
  type RefObject,
} from "react";
import { Z_LAYER } from "./overlay-styles";
import { MotoNavIcon } from "~/components/ui/moto-nav-icon";
import { NavLink, type NavLinkProps } from "~/components/ui/nav-link";
import type { MotoConcept } from "~/lib/moto-concepts";

const ITEM =
  "relative z-10 flex min-h-[44px] items-center justify-center gap-2.5 rounded-full py-2.5 transition-colors duration-200";
const ITEM_ACTIVE = "bg-gray-100 text-gray-900";
const ITEM_IDLE = "text-gray-500 hover:text-gray-700";
const ITEM_IDLE_QUIET = "text-gray-400 hover:text-gray-600";

/** Erst messen, dann animieren: Die erste Position springt nicht ins Bild. */
const INDICATOR_ANIMATION_DELAY_MS = 100;

interface IndicatorPosition {
  readonly left: number;
  readonly width: number;
}

export interface PortalBottomNavIndicator {
  readonly listRef: RefObject<HTMLUListElement | null>;
  readonly position: IndicatorPosition | null;
  readonly animated: boolean;
}

/**
 * Misst, wo die gleitende Fläche der `PortalBottomNav` steht. `getTarget`
 * nennt das Feld, unter dem sie liegt, oder `null`, dann fehlt sie. `watch`
 * ändert sich, sobald ein anderes Feld das Ziel sein kann; Breitenänderungen
 * des Ziels und der Leiste misst der Beobachter selbst.
 */
export function usePortalBottomNavIndicator(
  getTarget: () => HTMLElement | null,
  watch: string,
): PortalBottomNavIndicator {
  const listRef = useRef<HTMLUListElement | null>(null);
  const latestGetTarget = useRef(getTarget);
  const [position, setPosition] = useState<IndicatorPosition | null>(null);
  const [animated, setAnimated] = useState(false);

  useEffect(() => {
    latestGetTarget.current = getTarget;
  });

  useEffect(() => {
    const measure = () => {
      const target = latestGetTarget.current();
      // Dieselbe Position behält dasselbe Objekt, sonst rendert jede Messung
      // die Leiste neu (#2938).
      setPosition((previous) => {
        if (!target) return null;
        const { offsetLeft: left, offsetWidth: width } = target;
        return previous?.left === left && previous.width === width
          ? previous
          : { left, width };
      });
    };
    // Die kleine Verzögerung lässt dem Browser Zeit, die Breiten zu setzen.
    const timer = setTimeout(measure, 10);
    const observer = new ResizeObserver(measure);
    const target = latestGetTarget.current();
    if (target) observer.observe(target);
    if (listRef.current) observer.observe(listRef.current);
    return () => {
      clearTimeout(timer);
      observer.disconnect();
    };
  }, [watch]);

  useEffect(() => {
    const timer = setTimeout(
      () => setAnimated(true),
      INDICATOR_ANIMATION_DELAY_MS,
    );
    return () => clearTimeout(timer);
  }, []);

  return { listRef, position, animated };
}

/** Das Bild eines Felds: das Icon des Konzepts oder ein eigenes. */
type PortalNavIconProps =
  | {
      readonly concept: MotoConcept;
      /** Ein anderes Symbol im Ton von `concept`. */
      readonly iconConcept?: MotoConcept;
      readonly icon?: never;
    }
  | {
      /** Für ein Ziel ohne Konzept; ersetzt das Icon vollständig. */
      readonly icon: ReactNode;
      readonly concept?: never;
      readonly iconConcept?: never;
    };

interface PortalBottomNavItemProps {
  readonly label: string;
  readonly active: boolean;
  /** Ein Zähler am Feld. */
  readonly badge?: ReactNode;
  readonly paddingClassName: string;
  /** Wie der Name des aktiven Ziels dasteht, auch ab welcher Breite. */
  readonly labelClassName: string;
  /** Der leisere Ruhezustand der Tenant- und der Eltern-App. */
  readonly quiet?: boolean;
  /** Nur das aktive Feld darf schrumpfen: ein langer Name schiebt sonst die übrigen aus dem Bild (#4975). */
  readonly shrinkActive?: boolean;
}

function bottomNavItemClassName({
  active,
  paddingClassName,
  quiet = false,
  shrinkActive = false,
}: Pick<
  PortalBottomNavItemProps,
  "active" | "paddingClassName" | "quiet" | "shrinkActive"
>): string {
  const idle = quiet ? ITEM_IDLE_QUIET : ITEM_IDLE;
  return `${ITEM} ${paddingClassName} ${active ? ITEM_ACTIVE : idle} ${
    shrinkActive ? bottomNavItemShrink(active) : ""
  }`;
}

function bottomNavItemShrink(active: boolean): string {
  return active ? "min-w-0" : "shrink-0";
}

function PortalNavIcon({
  concept,
  iconConcept,
  icon,
  active,
  className,
}: PortalNavIconProps & {
  readonly active: boolean;
  readonly className: string;
}) {
  if (!concept) return icon;
  return (
    <MotoNavIcon
      concept={concept}
      iconConcept={iconConcept}
      active={active}
      className={className}
    />
  );
}

function BottomNavItemContent({
  label,
  active,
  badge,
  labelClassName,
  ...iconProps
}: Pick<
  PortalBottomNavItemProps,
  "label" | "active" | "badge" | "labelClassName"
> &
  PortalNavIconProps) {
  return (
    <>
      <span className="relative inline-flex">
        <PortalNavIcon
          {...iconProps}
          active={active}
          className="h-5 w-5 shrink-0"
        />
        {badge}
      </span>
      {active ? <span className={labelClassName}>{label}</span> : null}
    </>
  );
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
  label = "Hauptnavigation",
  barClassName = "px-3",
  fade = true,
  indicator,
  className = "",
}: {
  /** Die Felder (`PortalBottomNavLink`, `PortalBottomNavButton`), jedes in einem `<li>`. */
  readonly children: ReactNode;
  /** Abstand der Felder, je nachdem, wie voll die Leiste ist. */
  readonly listClassName: string;
  /** Der Name der Leiste für Vorleseprogramme. */
  readonly label?: string;
  /** Der seitliche Innenabstand der Pille. */
  readonly barClassName?: string;
  /** Der Verlauf hinter der Leiste, der den Inhalt ausblendet. */
  readonly fade?: boolean;
  /** Die gleitende Fläche aus `usePortalBottomNavIndicator`. */
  readonly indicator?: PortalBottomNavIndicator;
  readonly className?: string;
}) {
  return (
    <nav
      aria-label={label}
      className={`fixed inset-x-0 bottom-0 ${Z_LAYER.nav} lg:hidden print:hidden ${className}`}
    >
      {/* Die Leiste schwebt, ihr Rand ist also durchsichtig — ohne diesen
          Verlauf laeuft der Inhalt daneben und darunter sichtbar weiter und
          endet mitten in einer Zeile. Der Verlauf geht auf die Seitenfarbe
          (gray-50) und nicht auf Weiss, damit er auf dem gepunkteten
          Hintergrund keine Kante zieht. */}
      {fade ? (
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-x-0 bottom-0 h-28 bg-gradient-to-t from-gray-50 via-gray-50/90 to-transparent"
        />
      ) : null}
      <div className="relative px-4 pb-4">
        <div
          className={`rounded-full border border-gray-200/50 bg-white/95 ${barClassName} py-2 shadow-[0_-2px_20px_rgba(0,0,0,0.08)] backdrop-blur-md`}
        >
          <ul
            ref={indicator?.listRef}
            className={`relative flex items-center justify-around ${listClassName}`}
          >
            {indicator?.position ? (
              <li
                aria-hidden="true"
                className={`absolute top-0 h-full rounded-full bg-gray-100 ${
                  indicator.animated
                    ? "transition-[left,width] duration-150 ease-out"
                    : ""
                }`}
                style={{
                  left: `${indicator.position.left}px`,
                  width: `${indicator.position.width}px`,
                }}
              />
            ) : null}
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
> &
  PortalNavIconProps & {
    readonly label: string;
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
  iconConcept,
  icon,
  active,
  newTab = false,
  badge,
  paddingClassName,
  labelClassName,
  quiet,
  shrinkActive,
  ...linkProps
}: PortalNavLinkProps & PortalBottomNavItemProps) {
  return (
    <li className={shrinkActive ? bottomNavItemShrink(active) : undefined}>
      <NavLink
        {...linkProps}
        data-active={active ? "true" : "false"}
        aria-current={active ? "page" : undefined}
        {...(newTab ? { target: "_blank", rel: "noopener noreferrer" } : {})}
        aria-label={label}
        className={bottomNavItemClassName({
          active,
          paddingClassName,
          quiet,
          shrinkActive,
        })}
      >
        <BottomNavItemContent
          {...({ concept, iconConcept, icon } as PortalNavIconProps)}
          label={label}
          active={active}
          badge={badge}
          labelClassName={labelClassName}
        />
      </NavLink>
    </li>
  );
}

/**
 * Ein Feld der `PortalBottomNav`, das kein Ziel hat, sondern etwas öffnet (das
 * „Mehr"). Navigationszeile, kein Kit-Knopf (#5536): Form und Zustand sind die
 * der übrigen Felder.
 */
export function PortalBottomNavButton({
  label,
  concept,
  iconConcept,
  icon,
  active,
  badge,
  paddingClassName,
  labelClassName,
  quiet,
  shrinkActive,
  buttonRef,
  ...buttonProps
}: Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children" | "className" | "aria-label" | "type"
> &
  PortalNavIconProps &
  PortalBottomNavItemProps & {
    readonly buttonRef?: Ref<HTMLButtonElement>;
  }) {
  return (
    <li className={shrinkActive ? bottomNavItemShrink(active) : undefined}>
      <button
        {...buttonProps}
        ref={buttonRef}
        type="button"
        data-active={active ? "true" : "false"}
        aria-label={label}
        className={bottomNavItemClassName({
          active,
          paddingClassName,
          quiet,
          shrinkActive,
        })}
      >
        <BottomNavItemContent
          {...({ concept, iconConcept, icon } as PortalNavIconProps)}
          label={label}
          active={active}
          badge={badge}
          labelClassName={labelClassName}
        />
      </button>
    </li>
  );
}

const SHEET_ROW =
  "flex items-center gap-3 rounded-xl px-4 py-3 text-base font-medium transition-colors";
const SHEET_ROW_IDLE =
  "bg-gray-50 text-gray-900 hover:bg-gray-100 active:bg-gray-200";

/**
 * Eine Zeile im „Mehr"-Blatt der mobilen Leiste (Tenant- und Eltern-App,
 * Träger-Portal): Icon, Name, Zähler, Pfeil. Das `<li>` und eine Gruppentrennung
 * legt die Leiste selbst darum. `plain` lässt den Pfeil weg und hält den Namen
 * auch aktiv im normalen Gewicht, wie im Menü der Tenant-App.
 */
export function PortalNavSheetLink({
  label,
  concept,
  iconConcept,
  icon,
  active,
  newTab = false,
  badge,
  plain = false,
  ...linkProps
}: PortalNavLinkProps & { readonly plain?: boolean }) {
  return (
    <NavLink
      {...linkProps}
      {...(newTab ? { target: "_blank", rel: "noopener noreferrer" } : {})}
      data-active={active ? "true" : "false"}
      aria-current={active ? "page" : undefined}
      className={`${SHEET_ROW} ${
        active ? "bg-gray-100 font-semibold text-gray-900" : SHEET_ROW_IDLE
      }`}
    >
      <PortalNavIcon
        {...({ concept, iconConcept, icon } as PortalNavIconProps)}
        active={active}
        className={`h-5 w-5 ${active ? "" : "text-gray-600"}`}
      />
      <span className={plain ? "flex-1 font-medium" : "flex-1"}>{label}</span>
      {badge}
      {plain ? null : (
        <ChevronRight
          className="h-5 w-5 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      )}
    </NavLink>
  );
}

/** Eine Zeile im „Mehr"-Blatt, die eine Aktion auslöst (Abmelden): ohne Pfeil. */
export function PortalNavSheetButton({
  label,
  concept,
  ...buttonProps
}: Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children" | "className" | "type"
> & {
  readonly label: string;
  readonly concept: MotoConcept;
}) {
  return (
    <button
      {...buttonProps}
      type="button"
      className={`${SHEET_ROW} w-full text-left ${SHEET_ROW_IDLE}`}
    >
      <MotoNavIcon
        concept={concept}
        active={false}
        className="h-5 w-5 text-gray-600"
      />
      {label}
    </button>
  );
}
