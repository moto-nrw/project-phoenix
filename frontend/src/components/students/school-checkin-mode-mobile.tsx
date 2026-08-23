"use client";

import { useEffect } from "react";
import { CheckSquare, Loader2, LogIn, LogOut, X } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { GROUP_ROOM_SHADES } from "~/lib/location-helper";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";

const CHECKIN_BAR_OFFSET_VAR = "--moto-checkin-bar-offset";
const CHECKIN_BAR_OFFSET = "4.5rem";
const CHECKIN_SELECTION_BAR_OFFSET = "6.75rem";

interface SchoolCheckinModeMobileProps {
  /** Whether the page is currently in check-in/out mode. */
  readonly isActive: boolean;
  /** Toggle the mode on/off. */
  readonly onToggle: () => void;
  /** Successful per-session toggles — surfaced on the active sticky bar. */
  readonly successCount: number;
  /** Open API calls in flight — surfaces as a "X laufen" hint. */
  readonly pendingCount: number;
  /** Selection sub-mode on (#2359) — the sticky bar then counts marks, not writes. */
  readonly selectionActive?: boolean;
  readonly onSelectionActiveChange?: (active: boolean) => void;
  /** Currently marked students while the selection sub-mode is on. */
  readonly selectedCount?: number;
  readonly onClearSelection?: () => void;
  readonly onBulkAction?: (action: "in" | "out") => void;
  readonly runningAction?: "in" | "out" | null;
  /**
   * Locks the trigger, e.g. while a bulk request is in flight (#2359): the
   * hook ignores mode exits during a run, so "Fertig" mirrors that as a
   * visible disabled state instead of a dead tap.
   */
  readonly disabled?: boolean;
}

/**
 * Mobile-only (`<md` / <768px) check-in mode trigger.
 *
 * - **OFF**: full-width inline pill rendered in the page flow at the top
 *   of the card list. Scrolls away with the rest of the (non-sticky)
 *   header — re-accessible via scroll-up. Mode activation is a once-per-
 *   shift action so this is fine.
 * - **ON**: fixed sticky bar above the mobile bottom nav. Counter +
 *   "Fertig" button stay in the thumb zone for the duration of the
 *   multi-tap session (iOS Photos / Apple Mail multi-select pattern).
 *
 * Tablet (md..lg) uses the floating FAB; desktop (lg+) uses the inline
 * pill inside the page header's primaryAction slot.
 */
export function SchoolCheckinModeMobile({
  isActive,
  onToggle,
  successCount,
  pendingCount,
  selectionActive = false,
  onSelectionActiveChange,
  selectedCount = 0,
  onClearSelection,
  onBulkAction,
  runningAction = null,
  disabled = false,
}: SchoolCheckinModeMobileProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();

  useEffect(() => {
    if (!attendanceWebEnabled || !isActive) return;
    const root = document.documentElement;
    const mediaQuery = window.matchMedia("(max-width: 767px)");
    const publishOffset = () => {
      if (mediaQuery.matches) {
        root.style.setProperty(
          CHECKIN_BAR_OFFSET_VAR,
          selectionActive ? CHECKIN_SELECTION_BAR_OFFSET : CHECKIN_BAR_OFFSET,
        );
      } else {
        root.style.removeProperty(CHECKIN_BAR_OFFSET_VAR);
      }
    };
    publishOffset();
    mediaQuery.addEventListener("change", publishOffset);
    return () => {
      mediaQuery.removeEventListener("change", publishOffset);
      root.style.removeProperty(CHECKIN_BAR_OFFSET_VAR);
    };
  }, [attendanceWebEnabled, isActive, selectionActive]);

  if (!attendanceWebEnabled) return null;
  if (!isActive) {
    return (
      <button
        type="button"
        onClick={onToggle}
        aria-pressed={false}
        aria-label="Kinder an- und abmelden"
        className="flex w-full items-center justify-center gap-2.5 rounded-full px-5 py-3 text-base font-semibold transition-colors duration-150 outline-none focus-visible:ring-2 focus-visible:ring-offset-2 active:scale-[0.99]"
        style={{
          backgroundColor: "var(--color-white)",
          color: GROUP_ROOM_SHADES.text,
          boxShadow: `inset 0 0 0 1.5px ${GROUP_ROOM_SHADES.base}, 0 1px 2px rgb(0 0 0 / 0.04)`,
          // @ts-expect-error CSS custom property for focus ring colour
          "--tw-ring-color": `${GROUP_ROOM_SHADES.base}80`,
        }}
      >
        <span
          className="flex size-5 flex-shrink-0 items-center justify-center rounded-full"
          style={{ backgroundColor: `${GROUP_ROOM_SHADES.base}1a` }}
        >
          <CheckSquare className="size-3.5" strokeWidth={2.5} aria-hidden />
        </span>
        <span>Kinder an- und abmelden</span>
      </button>
    );
  }

  // Sticky bar sits above the mobile bottom nav. The nav's actual visible
  // height is ~4.75rem (pill min-h-44px + py-2 + outer pb-4) plus its own
  // safe-area-inset. Offset = 5.5rem (nav + breathing gap) + safe-area so
  // the bar visually floats above the nav with a clear separator.
  return (
    <div
      className="fixed right-0 left-0 z-40 px-3"
      style={{ bottom: "calc(5.5rem + env(safe-area-inset-bottom))" }}
      data-checkin-mode-mobile="active"
    >
      <div
        className="moto-content-surface rounded-2xl border border-gray-200 bg-white/95 p-2 shadow-sm backdrop-blur"
        role="region"
        aria-label="An- und Abmelde-Modus"
        aria-busy={disabled && runningAction !== null}
      >
        <div className="flex items-center gap-2">
          {onSelectionActiveChange ? (
            <SegmentedControl
              ariaLabel="Tipp-Verhalten"
              items={[
                { value: "immediate", label: "Direkt", disabled },
                { value: "select", label: "Mehrere", disabled },
              ]}
              value={selectionActive ? "select" : "immediate"}
              onChange={(next) => onSelectionActiveChange(next === "select")}
              className="shrink-0"
            />
          ) : (
            <CheckSquare
              className="text-moto-green size-4 shrink-0"
              aria-hidden
            />
          )}

          <span className="min-w-0 flex-1 truncate text-xs font-medium text-gray-500 tabular-nums">
            {selectionActive
              ? null
              : successCount === 0
                ? "Bereit"
                : `${successCount} bearbeitet`}
            {!selectionActive && pendingCount > 0
              ? ` · ${pendingCount} laufen`
              : ""}
          </span>

          {pendingCount > 0 ? (
            <Loader2
              className="size-3.5 shrink-0 animate-spin text-gray-500"
              aria-hidden
            />
          ) : null}

          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onToggle}
            disabled={disabled}
            aria-label="Fertig, An- und Abmelde-Modus beenden"
            className="shrink-0 rounded-full shadow-none"
          >
            Fertig
          </Button>
        </div>

        {selectionActive ? (
          <div className="mt-1.5 flex items-center justify-end gap-1.5 border-t border-gray-100 pt-1.5">
            <span
              className="mr-auto text-xs font-semibold text-gray-700 tabular-nums"
              aria-live="polite"
            >
              {selectedCount} ausgewählt
            </span>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="shadow-none"
              onClick={onClearSelection}
              disabled={
                selectedCount === 0 ||
                disabled ||
                onClearSelection === undefined
              }
              aria-label="Auswahl aufheben"
            >
              <X className="size-3.5" aria-hidden />
            </Button>
            <Button
              type="button"
              variant="success"
              size="compact"
              className="rounded-lg text-white shadow-none hover:shadow-none"
              onClick={() => onBulkAction?.("in")}
              disabled={
                selectedCount === 0 || disabled || onBulkAction === undefined
              }
            >
              {runningAction === "in" ? (
                <Loader2 className="size-3.5 animate-spin" aria-hidden />
              ) : (
                <LogIn className="size-3.5" aria-hidden />
              )}
              Anmelden
            </Button>
            <Button
              type="button"
              variant="danger"
              size="compact"
              className="rounded-lg shadow-none hover:shadow-none"
              onClick={() => onBulkAction?.("out")}
              disabled={
                selectedCount === 0 || disabled || onBulkAction === undefined
              }
            >
              {runningAction === "out" ? (
                <Loader2 className="size-3.5 animate-spin" aria-hidden />
              ) : (
                <LogOut className="size-3.5" aria-hidden />
              )}
              Abmelden
            </Button>
          </div>
        ) : null}
      </div>
    </div>
  );
}
