"use client";

/**
 * TimetableToolbar — view switcher, range navigator, density picker, and
 * primary add actions in a single horizontal bar.
 *
 * Visual style follows shadcn/Linear conventions: ghost-style ghost buttons,
 * 1px borders only on segmented controls, no shadows on chrome, brand
 * accent reserved for currently-active states. The whole row is intended
 * to fit on one line at desktop widths.
 */

import type { ReactNode } from "react";
import { CalendarRange, ChevronLeft, ChevronRight, Plus } from "lucide-react";

import { Button } from "~/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { timetableSurface } from "./timetable-style";

export type TimetableView = "week" | "month" | "year" | "series";

/**
 * Three discrete zoom levels for the week grid. Pixel-per-hour values are
 * mapped from these — never expose raw pixels in the UI; that pattern was
 * called out as "horrible UX" and replaced with semantic labels matching
 * the Apple/Google/Outlook convention (zoom by intent, not magnitude).
 */
export type WeekDensity = "compact" | "normal" | "comfortable";

export const DENSITY_TO_HOUR_HEIGHT_PX: Record<WeekDensity, number> = {
  compact: 60,
  normal: 90,
  comfortable: 120,
};

const DENSITY_OPTIONS: Array<{ value: WeekDensity; label: string }> = [
  { value: "compact", label: "Kompakt" },
  { value: "normal", label: "Normal" },
  { value: "comfortable", label: "Komfortabel" },
];

interface TimetableToolbarProps {
  view: TimetableView;
  onViewChange: (next: TimetableView) => void;
  /** "KW 18 · Mo 28.04 – Fr 02.05.2026" or "April 2026". */
  rangeLabel: string;
  onPrev: () => void;
  onNext: () => void;
  onToday: () => void;
  /** Hide the Today pill when already on the current week/month. */
  isOnToday?: boolean;
  /** Disables prev/next/today + density picker when irrelevant. */
  navDisabled?: boolean;
  /** Currently active week density. Only rendered in week view. */
  density?: WeekDensity;
  onDensityChange?: (next: WeekDensity) => void;
  /** Period selector — folded into the toolbar so it isn't a dead top row. */
  periodSwitcher?: ReactNode;
  /** Primary add actions live in the toolbar to keep chrome on one row. */
  onAddInstance?: () => void;
  planWeekAction?: ReactNode;
  /**
   * Opens period management ("Schuljahre & Ferien") from the overflow
   * menu. Rendered in every view so management stays reachable even when
   * the period pill is hidden.
   */
  onManagePeriods?: () => void;
}

const VIEW_TABS: Array<{ id: TimetableView; label: string }> = [
  { id: "week", label: "Woche" },
  { id: "month", label: "Monat" },
  { id: "year", label: "Jahr" },
  { id: "series", label: "Regeltermine" },
];

export function TimetableToolbar({
  view,
  onViewChange,
  rangeLabel,
  onPrev,
  onNext,
  onToday,
  isOnToday = false,
  navDisabled = false,
  density,
  onDensityChange,
  periodSwitcher,
  onAddInstance,
  planWeekAction,
  onManagePeriods,
}: TimetableToolbarProps) {
  const showRangeNav = view !== "series";
  const showDensity = view === "week" && density && onDensityChange;
  const showOverflow = Boolean(showDensity) || Boolean(onManagePeriods);

  return (
    <div
      className={`${timetableSurface} flex flex-col gap-3 px-4 py-3 sm:px-6 lg:flex-row lg:items-center lg:py-2.5`}
    >
      {/* Segmented view tabs — reuses the shared kit Tabs (default/pill variant) */}
      <Tabs
        value={view}
        onValueChange={(next) => onViewChange(next as TimetableView)}
        className="w-full sm:w-auto lg:shrink-0 lg:self-auto"
      >
        <TabsList
          aria-label="Ansicht wählen"
          className="grid w-full grid-cols-4 sm:inline-flex sm:w-auto"
        >
          {VIEW_TABS.map((tab) => (
            <TabsTrigger key={tab.id} value={tab.id}>
              {tab.label}
            </TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {showRangeNav && (
        <div className="grid min-w-0 grid-cols-[2rem_minmax(0,1fr)_2rem] items-center gap-x-2 gap-y-2 sm:flex sm:gap-3">
          <div className="hidden h-6 w-px bg-gray-200 lg:block" aria-hidden />

          {/* Date navigator — ghost buttons, no borders */}
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onPrev}
            disabled={navDisabled}
            className="order-1"
            aria-label="Vorheriger Zeitraum"
          >
            <ChevronLeft className="h-4 w-4" />
          </Button>

          <span className="order-2 min-w-0 text-center text-sm leading-tight font-semibold text-gray-900 tabular-nums sm:order-4 sm:truncate sm:text-left">
            {rangeLabel}
          </span>

          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onNext}
            disabled={navDisabled}
            className="order-3 sm:order-2"
            aria-label="Nächster Zeitraum"
          >
            <ChevronRight className="h-4 w-4" />
          </Button>

          {/* Today pill — only when not already on today */}
          {!isOnToday && (
            <Button
              type="button"
              variant="outline"
              size="compact"
              onClick={onToday}
              disabled={navDisabled}
              className="order-4 col-span-3 justify-self-center sm:order-3"
            >
              Heute
            </Button>
          )}
        </div>
      )}

      {(periodSwitcher || onAddInstance || planWeekAction || showOverflow) && (
        <div className="flex w-full flex-wrap items-stretch gap-2 sm:w-auto sm:items-center lg:ml-auto lg:shrink-0 lg:flex-nowrap">
          {periodSwitcher}
          {planWeekAction && (
            <div className="min-w-0 flex-1 sm:flex-none">{planWeekAction}</div>
          )}

          {onAddInstance && (
            <Button
              type="button"
              variant="primary"
              size="compact"
              onClick={onAddInstance}
            >
              <Plus className="h-3.5 w-3.5" />
              Termin
            </Button>
          )}

          {showOverflow && (
            <OverflowMenu
              ariaLabel="Weitere Optionen"
              items={buildToolbarOverflowItems({
                density: showDensity ? density : undefined,
                onDensityChange: showDensity ? onDensityChange : undefined,
                onManagePeriods,
              })}
            />
          )}
        </div>
      )}
    </div>
  );
}

/**
 * Builds the toolbar's overflow-menu entries ("Weitere Optionen"). Carries
 * the week-view density picker (where it applies) and the Verwaltung
 * section that opens period management from every view.
 */
function buildToolbarOverflowItems({
  density,
  onDensityChange,
  onManagePeriods,
}: {
  density?: WeekDensity;
  onDensityChange?: (next: WeekDensity) => void;
  onManagePeriods?: () => void;
}): OverflowMenuEntry[] {
  const entries: OverflowMenuEntry[] = [];

  if (density && onDensityChange) {
    entries.push({ kind: "header", label: "Zeilenhöhe" });
    for (const opt of DENSITY_OPTIONS) {
      entries.push({
        kind: "radio",
        label: opt.label,
        checked: density === opt.value,
        onClick: () => onDensityChange(opt.value),
      });
    }
  }

  if (onManagePeriods) {
    entries.push({ kind: "header", label: "Verwaltung" });
    entries.push({
      label: "Schuljahre & Ferien",
      icon: <CalendarRange className="h-4 w-4" aria-hidden />,
      onClick: onManagePeriods,
    });
  }

  return entries;
}
