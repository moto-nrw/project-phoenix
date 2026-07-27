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
 *
 * Form (#2031): ein eigener Balken, damit die Leiste auf den ersten Blick als
 * Kopfbereich lesbar ist und nicht wie freistehende Bedienelemente über dem
 * Raster wirkt. Drei Regeln halten die drei Flächen zusammen:
 *
 * 1. Der Seitenname steht bereits in der App-Kopfzeile und wird hier nur noch
 *    für Screenreader gerendert. Die sichtbare Überschrift ist das Datum, also
 *    die Antwort auf "was sehe ich gerade".
 * 2. Die Navigation ist EIN Objekt: eine gerahmte Gruppe aus Zurück, Heute und
 *    Weiter. "Heute" ist immer da (deaktiviert, wenn man schon dort steht), weil
 *    ein auftauchender und verschwindender Button die Zeile seitlich springen
 *    ließ.
 * 3. Zeile 2 ist eine ruhige 12px-Kontextzeile, keine Sammlung verschiedener
 *    Pillen. Sie wird immer gerendert, damit der Inhalt darunter beim
 *    Seitenwechsel nicht springt.
 */

/**
 * Feste Mindesthöhen beider Zeilen (#2031). Sie sind der einzige Grund, warum
 * Betreuungsplan, Dienstplan und Vertretung gleich hoch beginnen: die Zeilen
 * behalten ihre Höhe unabhängig davon, was eine Fläche einfüllt (ein Datum,
 * eine Wochenleiste, gar nichts). Jedes Element, das in eine Zeile einzieht,
 * muss in diese Höhe passen — siehe PlanningDayChip.
 */
const PRIMARY_ROW_MIN_H = "min-h-9";
const CONTEXT_ROW_MIN_H = "min-h-8";

interface PlanningContextBarProps {
  /** Seitentitel, z. B. "Vertretung" oder "Dienstplan". Auf kleinen
   *  Ansichten sichtbar, ab md steht er in der App-Kopfzeile. */
  readonly title: string;
  readonly onPrevious?: () => void;
  readonly onNext?: () => void;
  readonly previousLabel?: string;
  readonly nextLabel?: string;
  /** Sichtbare Überschrift der Fläche, z. B. "KW 31 · 27.07.–02.08.2026". */
  readonly dateLabel?: string;
  /** Beliebiger Inhalt anstelle des Datums, falls eine Fläche etwas anderes
   *  als Überschrift braucht. */
  readonly navigationSlot?: ReactNode;
  /** Callback des "Heute"-Buttons. Ohne Callback bleibt der Button sichtbar,
   *  aber deaktiviert (feste Geometrie, siehe Regel 2 oben). */
  readonly onToday?: () => void;
  readonly todayLabel?: string;
  /** Slot für den Ansichts-Umschalter (`ui/Tabs` kommt vom Aufrufer). */
  readonly viewSwitcher?: ReactNode;
  /** Rechtsbündige Aktionen (Primäraktion etc.). */
  readonly actions?: ReactNode;
  /** Zeile 2, die Kontextzeile. */
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
      className={`moto-content-surface flex flex-col gap-2 rounded-2xl border px-4 py-3 ${className ?? ""}`}
    >
      <h1 className="text-lg font-semibold text-gray-900 md:sr-only">
        {title}
      </h1>

      <div className={`flex flex-wrap items-center gap-3 ${PRIMARY_ROW_MIN_H}`}>
        {/* Eine Gruppe, drei Segmente: die Zeitnavigation liest sich als ein
            Bedienelement statt als zwei schwebende Pfeile mit Text dazwischen. */}
        <div className="inline-flex h-8 shrink-0 divide-x divide-gray-200 overflow-hidden rounded-lg border border-gray-200 bg-white">
          <Button
            type="button"
            size="icon"
            variant="ghost"
            className="rounded-none"
            onClick={onPrevious}
            disabled={!onPrevious}
            aria-label={previousLabel}
          >
            <ChevronLeft className="h-4 w-4" aria-hidden="true" />
          </Button>
          <Button
            type="button"
            size="compact"
            variant="ghost"
            className="rounded-none px-3"
            onClick={onToday}
            disabled={!onToday}
          >
            {todayLabel}
          </Button>
          <Button
            type="button"
            size="icon"
            variant="ghost"
            className="rounded-none"
            onClick={onNext}
            disabled={!onNext}
            aria-label={nextLabel}
          >
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>

        {navigationSlot ??
          (dateLabel && (
            <p className="text-base font-semibold text-gray-900 tabular-nums">
              {dateLabel}
            </p>
          ))}

        {(viewSwitcher ?? actions) && (
          <div className="ml-auto flex items-center gap-2">
            {viewSwitcher}
            {actions}
          </div>
        )}
      </div>

      {/* Haarlinie trennt Bedienung (oben) von Kontext (unten) INNERHALB des
          Balkens: zwei Zonen, eine Fläche. */}
      <div className="border-t border-gray-100 pt-2">
        <div
          className={`flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-gray-500 ${CONTEXT_ROW_MIN_H}`}
        >
          {children}
        </div>
      </div>
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

/** Punktfarbe des Zählers: dieselbe Lückenfarbe wie im CoverageIndicator. */
const COUNT_DOT_COLOR = "#F78C10";

/**
 * PlanningDayChip is the building block of a week strip in the context row
 * (Wochentagskürzel + Datum + optionale Zähler-Ziffer). It holds no date logic
 * of its own — every label and the selection state come from the caller.
 *
 * Einzeilig (#2031): der frühere dreizeilige Chip war so hoch wie die gesamte
 * Bedienzeile und ließ die Leiste in der Vertretung höher werden als in den
 * anderen Flächen. Eine Null wird nicht gezeigt: ein Tag ohne offene Lücke
 * bleibt ruhig, sichtbar ist nur, wo etwas offen ist.
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
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={ariaLabel}
      className={[
        "inline-flex h-8 shrink-0 items-center gap-1 rounded-md px-2 text-xs focus-visible:ring-2 focus-visible:ring-gray-900 focus-visible:outline-none",
        selected
          ? "bg-gray-900 text-white"
          : "text-gray-600 hover:bg-gray-100 hover:text-gray-900",
        className ?? "",
      ]
        .filter(Boolean)
        .join(" ")}
    >
      <span className="font-medium">{weekdayLabel}</span>
      <span className="tabular-nums">{dateLabel}</span>
      {showPlaceholder ? (
        <span
          className={`tabular-nums ${selected ? "text-white/60" : "text-gray-400"}`}
        >
          –
        </span>
      ) : count !== undefined && count > 0 ? (
        <span className="inline-flex items-center gap-1 font-semibold tabular-nums">
          <span
            aria-hidden
            className="h-1.5 w-1.5 rounded-full"
            style={{ backgroundColor: COUNT_DOT_COLOR }}
          />
          {count}
        </span>
      ) : null}
    </button>
  );
}
