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

import { useEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, MoreVertical, Plus } from "lucide-react";

export type TimetableView = "week" | "month" | "series";

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
  /** Primary add actions live in the toolbar to keep chrome on one row. */
  onAddInstance?: () => void;
}

const VIEW_TABS: Array<{ id: TimetableView; label: string }> = [
  { id: "week", label: "Woche" },
  { id: "month", label: "Monat" },
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
  onAddInstance,
}: TimetableToolbarProps) {
  const showRangeNav = view !== "series";
  const showDensity = view === "week" && density && onDensityChange;

  return (
    <div className="flex flex-wrap items-center gap-3 border-b border-slate-200 bg-white px-6 py-2.5">
      {/* Segmented view tabs */}
      <div
        role="tablist"
        aria-label="Ansicht wählen"
        className="inline-flex items-center rounded-md bg-slate-100 p-0.5"
      >
        {VIEW_TABS.map((tab) => {
          const isActive = view === tab.id;
          return (
            <button
              key={tab.id}
              role="tab"
              type="button"
              aria-selected={isActive}
              onClick={() => onViewChange(tab.id)}
              className={`rounded px-3 py-1 text-[13px] font-medium transition-colors ${
                isActive
                  ? "bg-white text-slate-900 shadow-sm"
                  : "text-slate-600 hover:text-slate-900"
              }`}
            >
              {tab.label}
            </button>
          );
        })}
      </div>

      {showRangeNav && (
        <>
          <div className="h-6 w-px bg-slate-200" aria-hidden />

          {/* Date navigator — ghost buttons, no borders */}
          <div className="flex items-center gap-0.5">
            <button
              type="button"
              onClick={onPrev}
              disabled={navDisabled}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
              aria-label="Vorheriger Zeitraum"
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <button
              type="button"
              onClick={onNext}
              disabled={navDisabled}
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-40"
              aria-label="Nächster Zeitraum"
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>

          {/* Today pill — only when not already on today */}
          {!isOnToday && (
            <button
              type="button"
              onClick={onToday}
              disabled={navDisabled}
              className="inline-flex h-8 items-center rounded-md border border-slate-200 px-2.5 text-[12px] font-medium text-slate-700 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Heute
            </button>
          )}

          <span className="text-sm font-semibold text-slate-900 tabular-nums">
            {rangeLabel}
          </span>
        </>
      )}

      {/* Spacer pushes everything below to the right */}
      <div className="flex-1" />

      {showDensity && (
        <DensityMenu density={density} onDensityChange={onDensityChange} />
      )}

      {onAddInstance && (
        <div className="flex items-center gap-1.5">
          <button
            type="button"
            onClick={onAddInstance}
            className="inline-flex h-8 items-center gap-1 rounded-md bg-slate-900 px-2.5 text-[12px] font-medium text-white transition-colors hover:bg-slate-700"
          >
            <Plus className="h-3.5 w-3.5" />
            Termin
          </button>
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

  useEffect(() => {
    if (!open) return;
    function onDocClick(event: MouseEvent) {
      if (
        containerRef.current &&
        event.target instanceof Node &&
        !containerRef.current.contains(event.target)
      ) {
        setOpen(false);
      }
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDocClick);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocClick);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  return (
    <div className="relative" ref={containerRef}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label="Weitere Optionen"
        className="inline-flex h-8 w-8 items-center justify-center rounded-md text-slate-500 transition-colors hover:bg-slate-100 hover:text-slate-900"
      >
        <MoreVertical className="h-4 w-4" />
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 z-30 mt-2 w-56 overflow-hidden rounded-lg border border-slate-200 bg-white shadow-md"
        >
          <div className="border-b border-slate-100 px-3 py-2">
            <p className="text-[10px] font-semibold tracking-wider text-slate-500 uppercase">
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
                className={`flex w-full items-center justify-between px-3 py-2 text-[12px] font-medium transition-colors hover:bg-slate-50 ${
                  isActive ? "text-slate-900" : "text-slate-600"
                }`}
              >
                <span>{opt.label}</span>
                {isActive && (
                  <span className="text-[12px] text-slate-900" aria-hidden>
                    ✓
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
