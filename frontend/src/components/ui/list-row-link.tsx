"use client";

import { ChevronRight } from "lucide-react";
import type { CSSProperties, MouseEventHandler, ReactNode } from "react";
import NavigationLink from "~/components/ui/navigation-link";
import { cn } from "~/lib/utils";
import { listRowLinkStateClasses } from "./list-row-styles";

type ListRowLinkTarget =
  | {
      readonly href: string;
      readonly onClick?: never;
      readonly disabled?: never;
    }
  | {
      readonly href?: never;
      readonly onClick?: MouseEventHandler<HTMLButtonElement>;
      readonly disabled?: boolean;
    };

type ListRowLinkProps = ListRowLinkTarget & {
  /** Vor dem Text: Symbol-Kachel, Avatar oder eine Zeitspalte. */
  readonly icon?: ReactNode;
  readonly title: ReactNode;
  readonly subtitle?: ReactNode;
  /**
   * Kleine Zeile über dem Titel, etwa „Für Baran Amini“. Mit `overline` oder
   * `meta` wird die Zeile dreizeilig wie eine Unterhaltung: größerer Titel,
   * Vorschau als `subtitle`, Pille rechts neben dem Titel.
   */
  readonly overline?: ReactNode;
  /** Rechts neben `overline`, etwa der Zeitpunkt der letzten Nachricht. */
  readonly meta?: ReactNode;
  /** Vorlesetext für `meta`, wenn die sichtbare Form gekürzt ist. */
  readonly metaLabel?: string;
  /** Vor dem Pfeil, etwa eine Pille; dreizeilig rechts neben dem Titel. */
  readonly badge?: ReactNode;
  /**
   * Die Zeile ist die offene Auswahl, etwa das Objekt im Bereich daneben:
   * grün hinterlegt, Titel fett, Pfeil grün. Setzt `aria-current`, denn Link
   * und Knopf kennen kein `aria-selected` (das gilt nur für option, tab, row).
   */
  readonly selected?: boolean;
  /**
   * Farbige Kante links als Hex (`#RRGGBB`), etwa `LOCATION_COLORS.DANGER`
   * oder die Farbe einer Planungsspur. Rein schmückend: der Text nennt, was
   * sie bedeutet.
   */
  readonly accent?: string;
  /** Tönt die Zeile zusätzlich leicht in der Farbe von `accent`. */
  readonly accentTint?: boolean;
  /**
   * Ungelesen: Titel fett, `meta` blau, Vorschau dunkler. Die Zahl dazu kommt
   * als `NotificationBadge` in `badge`; einen Punkt gibt es nicht, `StatusDot`
   * bleibt Legenden und Plänen vorbehalten (ADR 0149).
   */
  readonly unread?: boolean;
  /** Als einzelne Karte statt als Zeile einer `FramedList`. */
  readonly framed?: boolean;
  /** Führt die Zeile nicht auf eine andere Seite, entfällt der Pfeil. */
  readonly chevron?: boolean;
  readonly className?: string;
  readonly "aria-label"?: string;
};

function accentStyle(
  accent: string | undefined,
  tint: boolean,
): CSSProperties | undefined {
  if (!accent) return undefined;
  return {
    borderLeftColor: accent,
    // Dieselbe Tönung wie ein eigener Block im Plan.
    ...(tint ? { backgroundColor: `${accent}1A` } : {}),
  };
}

/**
 * Eine Zeile, die woanders hinführt: Symbol, Titel, Untertitel, Pille, Pfeil.
 * In einer `FramedList` oder als einzelne Karte mit `framed`. Zustände:
 * `selected` (offene Auswahl), `accent` (farbige Kante) und dreizeilig mit
 * `overline`, `meta` und `unread` (Unterhaltungen).
 */
export function ListRowLink({
  icon,
  title,
  subtitle,
  overline,
  meta,
  metaLabel,
  badge,
  href,
  onClick,
  disabled = false,
  selected = false,
  accent,
  accentTint = false,
  unread = false,
  framed = false,
  chevron = true,
  className,
  "aria-label": ariaLabel,
}: ListRowLinkProps) {
  const stacked = overline != null || meta != null;
  const rowClass = cn(
    "group flex w-full items-center gap-3 text-left transition-colors focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-gray-400",
    framed ? "rounded-2xl p-4" : "px-4 py-3",
    accent && "border-l-4",
    disabled && "cursor-not-allowed opacity-60",
    !disabled && !framed && listRowLinkStateClasses(selected),
    !framed && className,
  );
  const marker = badge ? (
    <span className="flex shrink-0 items-center gap-2">{badge}</span>
  ) : null;

  const text = stacked ? (
    <span className="min-w-0 flex-1 space-y-0.5">
      <span className="flex min-w-0 items-center justify-between gap-2">
        <span className="min-w-0 truncate text-xs font-medium text-gray-500">
          {overline}
        </span>
        {meta != null ? (
          <span
            aria-label={metaLabel}
            className={cn(
              "shrink-0 text-xs tabular-nums",
              unread ? "text-moto-blue font-semibold" : "text-gray-500",
            )}
          >
            {meta}
          </span>
        ) : null}
      </span>
      <span className="flex min-w-0 items-center justify-between gap-2">
        <span
          className={cn(
            "min-w-0 truncate text-base text-gray-900",
            unread || selected ? "font-bold" : "font-semibold",
          )}
        >
          {title}
        </span>
        {marker}
      </span>
      {subtitle ? (
        <span
          className={cn(
            "block min-w-0 truncate text-sm",
            unread ? "text-gray-700" : "text-gray-500",
          )}
        >
          {subtitle}
        </span>
      ) : null}
    </span>
  ) : (
    <span className="min-w-0 flex-1">
      <span
        className={cn(
          "block text-sm text-gray-900",
          framed ? "font-semibold" : "truncate",
          !framed && (selected || unread ? "font-semibold" : "font-medium"),
        )}
      >
        {title}
      </span>
      {subtitle ? (
        <span
          className={cn(
            "block text-xs",
            unread ? "text-gray-700" : "text-gray-500",
            framed && "mt-1",
          )}
        >
          {subtitle}
        </span>
      ) : null}
    </span>
  );

  const body = (
    <>
      {icon}
      {text}
      {stacked ? null : marker}
      {chevron ? (
        <ChevronRight
          className={cn(
            "shrink-0",
            stacked ? "h-5 w-5" : "h-4 w-4",
            selected ? "text-moto-green" : "text-gray-400",
            framed && "group-hover:text-gray-700",
          )}
          aria-hidden="true"
        />
      ) : null}
    </>
  );

  const style = accentStyle(accent, accentTint);
  const current = selected ? ("true" as const) : undefined;
  const row = href ? (
    <NavigationLink
      href={href}
      aria-label={ariaLabel}
      aria-current={current}
      className={rowClass}
      style={style}
    >
      {body}
    </NavigationLink>
  ) : (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-label={ariaLabel}
      aria-current={current}
      className={rowClass}
      style={style}
    >
      {body}
    </button>
  );
  if (!framed) return row;
  return (
    <div className={cn("moto-content-surface rounded-2xl border", className)}>
      {row}
    </div>
  );
}
