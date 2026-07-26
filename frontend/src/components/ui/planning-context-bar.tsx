"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "~/components/ui/button";

/**
 * PlanningContextBar is the shared header of all three planning surfaces
 * (Betreuungsplan, Dienstplan, Vertretung) —
 * docs/planung-redesign/docs/04-designsprache.md Abschnitt 6.2. It carries
 * no domain logic, no date math, and no data fetching: every value it shows
 * is a prop, and every navigation step is a callback the caller resolves.
 * The bar is generic on purpose (see Akzeptanzkriterium 11 in 04-designsprache.md,
 * "Y7"); domain-specific content (a week strip of day chips, a Lückenzähler
 * chip, …) is composed by the caller via `navigationSlot`/`viewSwitcher`/
 * `actions`/`children`.
 */

interface PlanningContextBarProps {
  /** Seitentitel, z. B. "Vertretung" oder "Dienstplan". */
  readonly title: string;
  readonly onPrevious?: () => void;
  readonly onNext?: () => void;
  readonly previousLabel?: string;
  readonly nextLabel?: string;
  /** Anzeigelabel zwischen den Pfeilen, z. B. "KW 29" oder "Mo 13.07.". Wird
   *  ignoriert, wenn `navigationSlot` gesetzt ist. */
  readonly dateLabel?: string;
  /** Beliebiger Inhalt zwischen den Pfeilen statt des Anzeigelabels — z. B.
   *  die Wochenleiste aus sieben `PlanningDayChip`s in der Vertretung. */
  readonly navigationSlot?: ReactNode;
  /** Callback des "Heute"-Buttons. Der Button wird nur gerendert, wenn diese
   *  Prop gesetzt ist. */
  readonly onToday?: () => void;
  readonly todayLabel?: string;
  /** Slot für den Ansichts-Umschalter (`ui/Tabs` kommt vom Aufrufer). */
  readonly viewSwitcher?: ReactNode;
  /** Rechtsbündige Aktionen (Primäraktion etc.). */
  readonly actions?: ReactNode;
  /** Zeile 2, die Kontextzeile — nur gerendert, wenn vorhanden. */
  readonly children?: ReactNode;
  readonly className?: string;
}

export function PlanningContextBar({
  title,
  onPrevious,
  onNext,
  previousLabel = "Zurück",
  nextLabel = "Weiter",
  dateLabel,
  navigationSlot,
  onToday,
  todayLabel = "Heute",
  viewSwitcher,
  actions,
  children,
  className,
}: PlanningContextBarProps) {
  return (
    <div
      className={`moto-content-surface rounded-2xl border p-3 sm:p-4 ${className ?? ""}`}
    >
      <div className="flex flex-wrap items-center gap-2">
        <h1 className="text-lg font-semibold text-gray-900">{title}</h1>

        <div className="flex items-center gap-1">
          <Button
            type="button"
            size="icon"
            variant="ghost"
            onClick={onPrevious}
            disabled={!onPrevious}
            aria-label={previousLabel}
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          </Button>
          {navigationSlot ??
            (dateLabel && (
              <span className="text-sm font-medium text-gray-900 tabular-nums">
                {dateLabel}
              </span>
            ))}
          <Button
            type="button"
            size="icon"
            variant="ghost"
            onClick={onNext}
            disabled={!onNext}
            aria-label={nextLabel}
          >
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>

        {onToday && (
          <Button type="button" variant="outline" size="md" onClick={onToday}>
            {todayLabel}
          </Button>
        )}

        {viewSwitcher}

        {actions && (
          <div className="ml-auto flex items-center gap-2">{actions}</div>
        )}
      </div>

      {children && (
        <div className="flex flex-wrap items-center gap-2 pt-2">{children}</div>
      )}
    </div>
  );
}

interface PlanningDayChipProps {
  /** Wochentagskürzel, z. B. "Mo". */
  readonly weekdayLabel: string;
  /** Tagesdatum, z. B. "14.07.". */
  readonly dateLabel: string;
  /** Kleine Zähler-Ziffer, z. B. Anzahl offener Lücken. */
  readonly count?: number;
  /** Zeigt einen Strich statt einer Ziffer, wenn kein Zähler verfügbar ist
   *  (z. B. vergangene Tage oder ein fehlgeschlagener Abruf). */
  readonly showPlaceholder?: boolean;
  readonly selected?: boolean;
  readonly onClick?: () => void;
  readonly className?: string;
  readonly "aria-label"?: string;
}

/**
 * PlanningDayChip is the building block of a week strip inside
 * `navigationSlot` (Wochentagskürzel + Datum + optionale Zähler-Ziffer). It
 * holds no date logic of its own — every label and the selection state come
 * from the caller.
 */
export function PlanningDayChip({
  weekdayLabel,
  dateLabel,
  count,
  showPlaceholder = false,
  selected = false,
  onClick,
  className,
  "aria-label": ariaLabel,
}: PlanningDayChipProps) {
  const countClassName = selected ? "text-white" : "text-gray-900";
  const placeholderClassName = selected ? "text-white/70" : "text-gray-400";

  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={ariaLabel}
      className={[
        "flex flex-col items-center rounded-md px-1 py-1 text-center focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none sm:px-2",
        selected
          ? "bg-gray-900 text-white"
          : "text-gray-600 hover:bg-gray-100 hover:text-gray-900",
        className ?? "",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <span className="text-[11px] font-medium">{weekdayLabel}</span>
      <span className="text-xs tabular-nums">{dateLabel}</span>
      {showPlaceholder ? (
        <span className={`text-[11px] tabular-nums ${placeholderClassName}`}>
          –
        </span>
      ) : count !== undefined ? (
        <span
          className={`text-[11px] font-semibold tabular-nums ${countClassName}`}
        >
          {count}
        </span>
      ) : null}
    </button>
  );
}
