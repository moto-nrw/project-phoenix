"use client";

/**
 * TimetableAddMenu: the single "+ Neu" entry point in the timetable toolbar.
 */

import { useCallback, useRef, useState } from "react";
import { CalendarPlus, ChevronDown, Plus, Repeat } from "lucide-react";

import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { Button } from "~/components/ui/button";
import { timetablePopoverSurface } from "./timetable-style";

interface TimetableAddMenuProps {
  /** Create a single one-off appointment. */
  onAddInstance: () => void;
  /** Create a recurring series. */
  onAddSeries: () => void;
  /** The visible date must belong to a planning period for one-off events. */
  instanceDisabled?: boolean;
  disabled?: boolean;
}

export function TimetableAddMenu({
  onAddInstance,
  onAddSeries,
  instanceDisabled = false,
  disabled = false,
}: TimetableAddMenuProps) {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  const closeMenu = useCallback(() => setOpen(false), []);
  useClickOutside(containerRef, closeMenu, open);

  return (
    <div className="relative w-full sm:w-auto" ref={containerRef}>
      <Button
        type="button"
        variant="primary"
        size="compact"
        onClick={() => setOpen((v) => !v)}
        disabled={disabled}
        aria-haspopup="menu"
        aria-expanded={open}
        className="w-full rounded-lg sm:w-auto"
      >
        <Plus className="h-3.5 w-3.5" aria-hidden />
        Neu
        <ChevronDown
          className={`h-3.5 w-3.5 transition-transform ${open ? "rotate-180" : ""}`}
          aria-hidden
        />
      </Button>

      {open && (
        <div
          role="menu"
          aria-label="Neu anlegen"
          className={`absolute right-0 z-30 mt-2 w-[min(20rem,calc(100vw-2rem))] ${timetablePopoverSurface}`}
        >
          <p className="px-3 pt-3 pb-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
            Termin anlegen
          </p>

          <button
            type="button"
            role="menuitem"
            disabled={instanceDisabled}
            onClick={() => {
              setOpen(false);
              onAddInstance();
            }}
            className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent"
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white shadow-sm">
              <CalendarPlus className="h-4 w-4 text-gray-700" aria-hidden />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold text-gray-900">
                Einmaliger Termin
              </div>
              <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                {instanceDisabled
                  ? "Wählen Sie einen planbaren Tag."
                  : "Findet nur an diesem Tag statt."}
              </p>
            </div>
          </button>

          <button
            type="button"
            role="menuitem"
            onClick={() => {
              setOpen(false);
              onAddSeries();
            }}
            className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none focus-visible:ring-inset"
          >
            <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-gray-200 bg-white shadow-sm">
              <Repeat className="h-4 w-4 text-gray-700" aria-hidden />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-semibold text-gray-900">
                Regelmäßiger Termin
              </div>
              <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                Für Angebote, die jede Woche oder alle zwei Wochen stattfinden.
              </p>
            </div>
          </button>
        </div>
      )}
    </div>
  );
}
