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

import { useRef, useState } from "react";
import type { ReactNode } from "react";
import {
  Check,
  ChevronLeft,
  ChevronRight,
  MoreVertical,
  Plus,
} from "lucide-react";

import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { Button } from "~/components/ui/button";
import { Tabs, TabsList, TabsTrigger } from "~/components/ui/tabs";

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
}

const VIEW_TABS: Array<{ id: TimetableView; label: string }> = [
  { id: "week", label: "Woche" },
  { id: "month", label: "Monat" },
  { id: "year", label: "Jahr" },
  { id: "series", label: "Serien" },
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
}: TimetableToolbarProps) {
  const showRangeNav = view !== "series";
  const showDensity = view === "week" && density && onDensityChange;

  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-gray-200 bg-white px-4 py-3 shadow-sm sm:px-6 lg:flex-row lg:items-center lg:py-2.5">
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
              variant="ghost"
              size="compact"
              onClick={onToday}
              disabled={navDisabled}
              className="order-4 col-span-3 justify-self-center border border-gray-200 sm:order-3"
            >
              Heute
            </Button>
          )}
        </div>
      )}

      {(periodSwitcher || onAddInstance || planWeekAction || showDensity) && (
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

          {showDensity && (
            <DensityMenu density={density} onDensityChange={onDensityChange} />
          )}
        </div>
      )}
    </div>
  );
}

function DensityMenu({
  density,
  onDensityChange,
}: {
  density: WeekDensity;
  onDensityChange: (next: WeekDensity) => void;
}) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useClickOutside(containerRef, () => setOpen(false), open);

  return (
    <div className="relative" ref={containerRef}>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Weitere Optionen"
      >
        <MoreVertical className="h-4 w-4" />
      </Button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 z-30 mt-2 w-56 overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg"
        >
          <div className="border-b border-gray-100 px-3 py-2">
            <p className="text-[10px] font-semibold tracking-wider text-gray-500 uppercase">
              Zeilenhöhe
            </p>
          </div>
          {DENSITY_OPTIONS.map((opt) => {
            const isActive = density === opt.value;
            return (
              <button
                key={opt.value}
                type="button"
                role="menuitemradio"
                aria-checked={isActive}
                onClick={() => {
                  onDensityChange(opt.value);
                  setOpen(false);
                }}
                className={`flex w-full items-center justify-between px-3 py-2 text-xs font-medium transition-colors hover:bg-gray-50 ${
                  isActive ? "text-gray-900" : "text-gray-600"
                }`}
              >
                <span>{opt.label}</span>
                {isActive && (
                  <Check className="h-4 w-4 text-gray-900" aria-hidden />
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
