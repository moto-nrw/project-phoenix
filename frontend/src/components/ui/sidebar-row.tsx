"use client";

import type { ButtonHTMLAttributes, ReactNode } from "react";
import NavigationLink, {
  type NavigationLinkProps,
} from "~/components/ui/navigation-link";
import {
  NotificationBadge,
  type NotificationBadgeTone,
} from "~/components/ui/notification-badge";
import { SECTION_LABEL_CLASS } from "~/components/ui/section-label";
import {
  SIDEBAR_RAIL_BADGE_CLASSES,
  sidebarChevronClasses,
  sidebarFadeClasses,
  sidebarLabelClasses,
  sidebarRowClasses,
} from "~/components/ui/sidebar-geometry";

/**
 * Die eine Zeile jeder Seitenleiste (#5975): Tenant, Schule, Träger und Eltern
 * teilen Raster, Aktivzustand und Zähler. Das Raster steht in
 * `sidebar-geometry.ts` (40 px auf jeder Breite, #2923).
 *
 * Eine einklappbare Leiste (Tenant) reicht `collapsed`, `labelsMounted` und
 * `labelsVisible` durch, die festen Leisten der Portale lassen sie weg.
 */
export interface SidebarRowCollapseState {
  /** Die Leiste ist der 64-px-Streifen. */
  readonly collapsed?: boolean;
  /** Bezeichnung und Beiwerk stehen im Baum (auch während der Blende). */
  readonly labelsMounted?: boolean;
  /** Bezeichnung und Beiwerk sind eingeblendet. */
  readonly labelsVisible?: boolean;
}

/** Ein Zähler an der Zeile. */
export interface SidebarRowBadge {
  readonly count: number;
  readonly tone: NotificationBadgeTone;
  /** Was gezählt wird, ganz ausgesprochen („3 ungelesene Nachrichten“). */
  readonly ariaLabel: string;
}

/**
 * Ein Sammelzähler aus mehreren Zählern (Kopfzeile einer Gruppe): Die Zahl
 * ist die Summe, vorgelesen wird jeder Teil, damit „5“ nicht als fünf
 * Nachrichten gilt, wenn darin Anmeldungen stecken (#6566).
 */
export function combineSidebarRowBadges(
  badges: readonly (SidebarRowBadge | null)[],
  tone: NotificationBadgeTone,
): SidebarRowBadge | null {
  const parts = badges.filter(
    (badge): badge is SidebarRowBadge => badge != null && badge.count > 0,
  );
  if (parts.length === 0) return null;
  const labels = parts.map((badge) => badge.ariaLabel);
  const last = labels.pop();
  return {
    count: parts.reduce((sum, badge) => sum + badge.count, 0),
    tone,
    ariaLabel: labels.length > 0 ? `${labels.join(", ")} und ${last}` : last!,
  };
}

/**
 * Der Inhalt jeder Zeile der Seitenleiste (#6580): Zeile, Kopfzeile einer
 * Gruppe, Akkordeon-Bereich und Unterpunkt. Reihenfolge: Icon, Bezeichnung,
 * Beiwerk (Schloss, Anwesenheit), Zähler, Chevron. Ausgeklappt steht der
 * Zähler rechts und blendet mit der Bezeichnung aus; im Streifen sitzt er als
 * Punkt auf der Ecke des Icons und blendet gegengleich ein. Für die Dauer der
 * Blende stehen beide im Baum; der ausblendende ist `aria-hidden`, sonst läse
 * ein Vorleseprogramm die Zahl zweimal (#2923).
 *
 * Maße, Farben und Lage stehen in `sidebar-geometry.ts`; die Hülle (Link oder
 * Schalter) bringt jede Zeilenart selbst mit.
 */
export function SidebarRowContent({
  icon,
  label,
  labelClassName,
  trailing,
  badge,
  chevronOpen,
  railBadgeClassName = SIDEBAR_RAIL_BADGE_CLASSES,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
}: SidebarRowCollapseState & {
  /** Fehlt beim Unterpunkt; dann rückt die Bezeichnung nicht ein. */
  readonly icon?: ReactNode;
  readonly label: string;
  /** Schrift der Bezeichnung, wenn sie keine Zeilenschrift ist (Gruppe). */
  readonly labelClassName?: string;
  /** Beiwerk hinter der Bezeichnung, das mit ihr ausblendet. */
  readonly trailing?: ReactNode;
  readonly badge?: SidebarRowBadge | null;
  /** Gesetzt, wenn die Zeile klappt: `true` heißt offen. */
  readonly chevronOpen?: boolean;
  /** Lage des Punkts im Streifen. */
  readonly railBadgeClassName?: string;
}) {
  const showBadge = badge != null && badge.count > 0;
  return (
    <>
      {icon}
      {labelsMounted && (
        <>
          <span
            className={`${sidebarLabelClasses(labelsVisible, icon != null)}${labelClassName ? ` ${labelClassName}` : ""}`}
          >
            {label}
          </span>
          {trailing ? (
            <span
              className={`ml-2 inline-flex shrink-0 ${sidebarFadeClasses(labelsVisible)}`}
            >
              {trailing}
            </span>
          ) : null}
          {showBadge && (
            <span
              aria-hidden={!labelsVisible}
              className={`ml-2 shrink-0 ${sidebarFadeClasses(labelsVisible)}`}
            >
              <NotificationBadge
                count={badge.count}
                tone={badge.tone}
                ariaLabel={badge.ariaLabel}
              />
            </span>
          )}
          {chevronOpen === undefined ? null : (
            <svg
              className={sidebarChevronClasses(chevronOpen, labelsVisible)}
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
              aria-hidden="true"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M19 9l-7 7-7-7"
              />
            </svg>
          )}
        </>
      )}
      {collapsed && showBadge && (
        <span
          aria-hidden={labelsVisible}
          className={`${railBadgeClassName} ${sidebarFadeClasses(!labelsVisible)}`}
        >
          <NotificationBadge
            count={badge.count}
            tone={badge.tone}
            size="sm"
            ariaLabel={badge.ariaLabel}
          />
        </span>
      )}
    </>
  );
}

type SidebarRowLinkProps = Omit<
  NavigationLinkProps,
  "children" | "className" | "aria-current" | "target" | "rel" | "title"
> &
  SidebarRowCollapseState & {
    readonly icon: ReactNode;
    readonly label: string;
    readonly active: boolean;
    /** Ziele außerhalb der App (die Hilfe) öffnen in einem neuen Tab. */
    readonly newTab?: boolean;
    readonly trailing?: ReactNode;
    readonly badge?: SidebarRowBadge | null;
    /** Der Name im Streifen, wenn er mehr sagt als `label` („…, nicht gebucht“). */
    readonly collapsedLabel?: string;
  };

/** Eine Zeile, die auf eine Seite führt. */
export function SidebarRowLink({
  icon,
  label,
  active,
  newTab = false,
  trailing,
  badge,
  collapsedLabel,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
  ...linkProps
}: SidebarRowLinkProps) {
  return (
    <NavigationLink
      {...linkProps}
      data-active={active ? "true" : "false"}
      aria-current={active ? "page" : undefined}
      className={sidebarRowClasses({ isActive: active })}
      {...(collapsed
        ? { title: label, "aria-label": collapsedLabel ?? label }
        : {})}
      {...(newTab ? { target: "_blank", rel: "noopener noreferrer" } : {})}
    >
      <SidebarRowContent
        icon={icon}
        label={label}
        trailing={trailing}
        badge={badge}
        collapsed={collapsed}
        labelsMounted={labelsMounted}
        labelsVisible={labelsVisible}
      />
    </NavigationLink>
  );
}

/**
 * Eine Zeile ohne Ziel, die ein kommendes Angebot ankündigt. Sie ist gesperrt
 * und zeigt `hint` erst beim Überfahren; im Streifen sagt sie nichts aus und
 * fehlt deshalb, sobald die Bezeichnungen ausgeblendet sind.
 */
export function SidebarRowPlaceholder({
  icon,
  label,
  hint,
  title,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
}: SidebarRowCollapseState & {
  readonly icon: ReactNode;
  readonly label: string;
  /** Kurzer Hinweis beim Überfahren („Bald“). */
  readonly hint: string;
  /** Die Erklärung als Tooltip. */
  readonly title: string;
}) {
  if (collapsed && !labelsMounted) return null;
  return (
    <div className={sidebarRowClasses({ isDisabled: true })} title={title}>
      <SidebarRowContent
        icon={icon}
        label={label}
        labelsMounted={labelsMounted}
        labelsVisible={labelsVisible}
        trailing={
          <span className="rounded bg-gray-100 px-2 py-0.5 text-xs text-gray-500 opacity-0 transition-opacity group-hover:opacity-100">
            {hint}
          </span>
        }
      />
    </div>
  );
}

/** Eine Zeile, die eine Aktion auslöst (Abmelden). */
export function SidebarRowButton({
  icon,
  label,
  collapsed = false,
  labelsMounted = true,
  labelsVisible = true,
  ...buttonProps
}: Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children" | "className" | "type" | "title"
> &
  SidebarRowCollapseState & {
    readonly icon: ReactNode;
    readonly label: string;
  }) {
  return (
    <button
      {...buttonProps}
      type="button"
      className={`${sidebarRowClasses()} cursor-pointer`}
      {...(collapsed ? { title: label, "aria-label": label } : {})}
    >
      <SidebarRowContent
        icon={icon}
        label={label}
        collapsed={collapsed}
        labelsMounted={labelsMounted}
        labelsVisible={labelsVisible}
      />
    </button>
  );
}

/**
 * Die Überschrift über einer Gruppe von Zeilen, die nicht klappt (Bereiche der
 * Einstellungen, Operator-Navigation, Träger-Gruppen). Im Streifen behält sie
 * ihre Höhe und blendet nur aus: so bleibt die Gruppierung als Abstand
 * erhalten und die Icons darunter springen beim Klappen nicht nach oben.
 */
export function SidebarSectionHeading({
  id,
  children,
  collapsed = false,
  labelsVisible = true,
}: {
  readonly id?: string;
  readonly children: ReactNode;
  readonly collapsed?: boolean;
  readonly labelsVisible?: boolean;
}) {
  return (
    <span
      id={id}
      aria-hidden={collapsed || undefined}
      className={`mb-1.5 block truncate px-3 ${SECTION_LABEL_CLASS} ${sidebarFadeClasses(labelsVisible)}`}
    >
      {children}
    </span>
  );
}
