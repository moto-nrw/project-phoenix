"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { Button } from "~/components/ui/button";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

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
 * Form: die Leiste ist KEINE eigene Karte mehr. Sie sitzt als Bedienband IN
 * der Kopfkarte von `TenantPage` (Slot `searchSlot`), an derselben Stelle, an
 * der jede andere Seite Suche und Filter trägt. Zwei Karten übereinander —
 * Seitenkopf und Planungsbalken — waren der letzte Ort im Portal, an dem eine
 * Seite zwei Köpfe hatte. Zwei Regeln halten die drei Flächen zusammen:
 *
 * 1. Den Seitennamen trägt die Kopfkarte. Das Datum in der Bedienzeile
 *    beantwortet "was sehe ich gerade".
 * 2. Die Navigation ist EIN Objekt: eine gerahmte Gruppe aus Zurück, Heute und
 *    Weiter. "Heute" ist immer da (deaktiviert, wenn man schon dort steht), weil
 *    ein auftauchender und verschwindender Button die Zeile seitlich springen
 *    ließ.
 * 3. Zeile 2 ist eine ruhige 12px-Kontextzeile, keine Sammlung verschiedener
 *    Pillen. Sie wird immer gerendert, damit der Inhalt darunter beim
 *    Seitenwechsel nicht springt.
 *
 * Mobiles Verhalten: die Bar darf auf einem Handy nicht per `flex-wrap` in vier
 * Zeilen zerfallen — gemessen fraß sie so ein Viertel bis ein Drittel des
 * Viewports, bevor der erste Inhalt kam. Sie ordnet sich deshalb unter md in
 * genau zwei Bedienzeilen (Titel + Navigation / Datum + Umschalter) und die
 * Kontextzeile scrollt horizontal, statt umzubrechen. Beide Layouts rendern
 * DIESELBEN Slot-Knoten — nichts wird für eine Breakpoint-Variante doppelt in
 * den DOM gehängt, sonst kollidieren Radix-IDs und Tests finden jedes Element
 * zweimal.
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
  /** Slot für den Ansichts-Umschalter (`SegmentedControl` vom Aufrufer). */
  readonly viewSwitcher?: ReactNode;
  /** Rechtsbündige Aktionen (Primäraktion etc.). */
  readonly actions?: ReactNode;
  /** Zeile 2, die Kontextzeile. */
  readonly children?: ReactNode;
  /** Für Flächen, die nie eine Kontextzeile haben (Statistik, Zeiterfassung):
   *  lässt die reservierte zweite Zeile ganz weg. */
  readonly withoutContextRow?: boolean;
  /** Setzt den `navigationSlot` ZWISCHEN die Pfeile, statt daneben. Für
   *  Flächen, deren Zeitraum frei wählbar ist (Statistik): der Chip, den man
   *  anklickt, und die Pfeile, die ihn verschieben, sind dann sichtbar EIN
   *  Bedienelement. Der Heute-Knopf wandert dabei aus der Gruppe heraus und
   *  erscheint nur, wenn es etwas zurückzusetzen gibt. */
  readonly navigationInGroup?: boolean;
  readonly className?: string;
}

export function PlanningContextBar({
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
  withoutContextRow = false,
  navigationInGroup = false,
  className,
}: PlanningContextBarProps) {
  return (
    <div className={`flex flex-col gap-3 ${className ?? ""}`}>
      <div
        className={`flex flex-wrap items-center gap-2 sm:gap-3 ${PRIMARY_ROW_MIN_H}`}
      >
        {/* Datumsblock: unter md füllt er die Zeile links, die Zeitnavigation
            sitzt rechts. Ab md löst `contents` den Wrapper auf und die
            `md:order-*`-Klassen stellen die Zeile als Navigation, Datum,
            Aktionen zusammen. */}
        {!navigationInGroup && (
          <div className="flex min-w-0 flex-1 flex-col md:contents">
            {navigationSlot ??
              (dateLabel && (
                // Unter md darf das Datum umbrechen: abgeschnitten („KW 36 ·
                // 31.08.–04.09....") fehlte auf dem Telefon genau das Jahr.
                <p className="min-w-0 text-sm font-semibold text-gray-900 tabular-nums md:order-2 md:truncate md:text-base">
                  {dateLabel}
                </p>
              ))}
          </div>
        )}

        {/* Eine Gruppe, drei Segmente: die Zeitnavigation liest sich als ein
            Bedienelement statt als zwei schwebende Pfeile mit Text dazwischen.
            Mobil sitzt sie rechts neben dem Datumsblock, ab md rückt sie per
            `order` wieder an den Anfang der Zeile. Mit `navigationInGroup`
            steht der Slot selbst zwischen den Pfeilen und die Gruppe füllt
            unter md die Zeile. */}
        <div
          className={`inline-flex h-9 shrink-0 divide-x divide-gray-200 overflow-hidden rounded-lg border border-gray-200 bg-white md:order-1 ${
            navigationInGroup ? "w-full md:w-auto" : ""
          }`}
        >
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
          {navigationInGroup ? (
            <div className="flex min-w-0 flex-1 items-stretch [&>*]:min-w-0 [&>*]:flex-1">
              {navigationSlot}
            </div>
          ) : (
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
          )}
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

        {/* Außerhalb der Gruppe ist „Heute" kein Dauergast: er steht nur da,
            wenn er etwas tut. Rechts von der Gruppe kann nichts springen. */}
        {navigationInGroup && onToday && (
          <Button
            type="button"
            size="compact"
            variant="ghost"
            className="h-9 px-3 text-gray-600 md:order-2"
            onClick={onToday}
          >
            {todayLabel}
          </Button>
        )}

        {/* Umschalter und Aktionen: unter md eine eigene Zeile über die VOLLE
            Breite (`basis-full`), in der sich der Ansichtsumschalter breit
            macht statt links zu kleben. Vorher standen dort zwei angeschnittene
            Bedienelemente und daneben viel ungenutzte Fläche. Ab md rutscht die
            Gruppe zurück an das rechte Ende derselben Zeile.

            Der Selektor streckt den Tab-Umschalter, ohne dass jede aufrufende
            Fläche daran denken muss: die Kit-Tabs sind `inline-flex`, also
            genau so breit wie ihre Beschriftungen.

            `flex-wrap` ist nötig, weil einzelne Aktionen ihre eigene mobile
            Breite mitbringen: `TimetableAddMenu` ist unter sm `w-full`. Ohne
            Umbruch kämpften zwei Elemente um dieselbe Zeile und der breitere
            drückte den Umschalter auf null (im Betreuungsplan lag der
            "Neu"-Knopf über den Ansichts-Tabs). Mit Umbruch nimmt jeder, was er
            braucht, und beide bleiben vollständig sichtbar. */}
        {(viewSwitcher ?? actions) && (
          <div
            className={`flex basis-full flex-wrap items-center gap-2 md:order-3 md:ml-auto md:basis-auto md:flex-nowrap max-md:[&>*]:min-w-0 max-md:[&>*]:flex-1 ${
              viewSwitcher
                ? // Der Umschalter nimmt unter md die freie Breite, damit er
                  // nicht auf Textbreite links klebt; ab md steht er wieder in
                  // seiner eigenen Breite am rechten Rand.
                  "[&>*:first-child]:min-w-0 [&>*:first-child]:flex-1 md:[&>*:first-child]:flex-none"
                : ""
            }`}
          >
            {viewSwitcher}
            {actions}
          </div>
        )}
      </div>

      {/* Haarlinie trennt Bedienung (oben) von Kontext (unten). Die Zeile
          bleibt auch leer stehen (#2031), damit der Inhalt beim Wechsel
          zwischen Planungsflächen nicht springt -- außer die Fläche hat nie
          Kontext (`withoutContextRow`): dann wäre sie nur ein leerer Streifen
          unter dem Umschalter, auf dem Telefon 40 px hoch. */}
      {!withoutContextRow && (
        <div className="border-t border-gray-100 pt-2">
          {/* Unter sm eine scrollende Zeile statt eines Zeilenumbruchs: eine
            umbrechende Wochenleiste warf die Trennlinie und die Zähler mitten
            in die zweite Zeile und ließ die Bar um zwei Zeilen wachsen. */}
          <div
            className={`flex [scrollbar-width:none] items-center gap-x-3 gap-y-1 overflow-x-auto text-xs text-gray-500 sm:flex-wrap sm:overflow-x-visible [&::-webkit-scrollbar]:hidden [&>*]:shrink-0 ${CONTEXT_ROW_MIN_H}`}
          >
            {children}
          </div>
        </div>
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
  readonly selected?: boolean;
  readonly onClick?: () => void;
  readonly className?: string;
  readonly "aria-label"?: string;
}

/** Punktfarbe des Zählers: dieselbe Lückenfarbe wie im CoverageIndicator. */
const COUNT_DOT_COLOR = MOTO_COLOR_PALETTE.orange.base;

/**
 * PlanningDayChip is the building block of a week strip in the context row
 * (Wochentagskürzel + Datum + optionale Zähler-Ziffer). It holds no date logic
 * of its own — every label and the selection state come from the caller.
 *
 * Einzeilig (#2031): der frühere dreizeilige Chip war so hoch wie die gesamte
 * Bedienzeile und ließ die Leiste in der Vertretung höher werden als in den
 * anderen Flächen. Ohne Zähler bleibt der Chip einfach still: eine Null wird
 * nicht gezeigt (ein Tag ohne offene Lücke ist keine Meldung wert), und ein
 * Tag ohne verfügbare Zahl bekommt auch keinen Platzhalter. Der frühere Strich
 * war nicht deutbar, ohne dass man die Ladelogik dahinter kennt.
 */
export function PlanningDayChip({
  weekdayLabel,
  dateLabel,
  count,
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
      {count !== undefined && count > 0 ? (
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
