"use client";

import { BELOW_MD } from "~/lib/hooks/use-media-query";
import { useEffect } from "react";
import { CheckSquare, LogIn, LogOut, X } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SelectionBar } from "~/components/ui/selection-bar";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useAttendanceWebEnabled } from "~/lib/tenant-context";
import {
  CHECKIN_HINTS,
  leftOutLabel,
  selectAllBlockedReason,
  type SelectAllControl,
} from "~/components/students/school-checkin-select-all";
import { SpinnerIcon } from "~/components/ui/icons";

const CHECKIN_BAR_OFFSET_VAR = "--moto-checkin-bar-offset";
// Measured bar height plus the install hint's clearance (#4358): 106 px in
// "Direkt", 151 px in "Mehrere" at 360 to 390 px viewport width. The line
// naming left-out children (#4762) wraps to two rows there: 187 px.
const CHECKIN_BAR_OFFSET = "7.25rem";
const CHECKIN_SELECTION_BAR_OFFSET = "9.5rem";
const CHECKIN_SELECTION_LEFT_OUT_BAR_OFFSET = "11.75rem";

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
  /** „Alle N auswählen“ for the children on screen (#4762). */
  readonly selectAll?: SelectAllControl;
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
  selectAll,
  disabled = false,
}: SchoolCheckinModeMobileProps) {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const leftOut =
    selectionActive && selectAll ? leftOutLabel(selectAll.leftOutCount) : null;
  const selectionOffset = leftOut
    ? CHECKIN_SELECTION_LEFT_OUT_BAR_OFFSET
    : CHECKIN_SELECTION_BAR_OFFSET;

  useEffect(() => {
    if (!attendanceWebEnabled || !isActive) return;
    const root = document.documentElement;
    const mediaQuery = window.matchMedia(BELOW_MD);
    const publishOffset = () => {
      if (mediaQuery.matches) {
        root.style.setProperty(
          CHECKIN_BAR_OFFSET_VAR,
          selectionActive ? selectionOffset : CHECKIN_BAR_OFFSET,
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
  }, [attendanceWebEnabled, isActive, selectionActive, selectionOffset]);

  if (!attendanceWebEnabled) return null;
  if (!isActive) {
    return (
      <Button
        type="button"
        onClick={onToggle}
        aria-pressed={false}
        aria-label="Kinder an- und abmelden"
        tooltip={CHECKIN_HINTS.start}
        variant="surface"
        size="base"
        className="w-full gap-2"
      >
        <CheckSquare className="size-4" strokeWidth={2.5} aria-hidden />
        <span>Kinder an- und abmelden</span>
      </Button>
    );
  }

  // Before the first tap the line stays empty: the mode line above already
  // says what is going on, and a lone „Bereit“ next to the switcher read like
  // a third option (#4358).
  const statusText = selectionActive
    ? ""
    : [
        successCount > 0 ? `${successCount} bearbeitet` : null,
        pendingCount > 0 ? `${pendingCount} laufen` : null,
      ]
        .filter(Boolean)
        .join(" · ");
  const showStatusRow =
    onSelectionActiveChange !== undefined ||
    statusText !== "" ||
    pendingCount > 0;

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
      <SelectionBar
        aria-label="An- und Abmelde-Modus"
        placement="inline"
        className="rounded-2xl p-2"
        count={selectedCount}
        busy={disabled && runningAction !== null}
        label="An- und Abmelde-Modus aktiv"
        description={
          selectionActive ? null : "Jeder Tipp meldet sofort an oder ab."
        }
        actions={
          <Button
            type="button"
            variant="primary"
            size="compact"
            onClick={onToggle}
            disabled={disabled}
            aria-label="Fertig, An- und Abmelde-Modus beenden"
            tooltip={CHECKIN_HINTS.finish}
            className="shrink-0"
          >
            Fertig
          </Button>
        }
      >
        {showStatusRow ? (
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
            ) : null}

            <span className="min-w-0 flex-1 truncate text-xs font-medium text-gray-500 tabular-nums">
              {statusText}
            </span>

            {pendingCount > 0 ? (
              <SpinnerIcon className="size-3.5 shrink-0 text-gray-500" />
            ) : null}

            {selectionActive && selectAll ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="shrink-0"
                onClick={selectAll.onSelectAll}
                disabled={
                  selectAll.count === 0 || selectAll.allSelected || disabled
                }
                tooltip={CHECKIN_HINTS.selectAll}
                disabledReason={selectAllBlockedReason(selectAll)}
              >
                Alle {selectAll.count} auswählen
              </Button>
            ) : null}
          </div>
        ) : null}

        {selectionActive ? (
          <div className="mt-1.5 flex items-center justify-end gap-1.5 border-t border-gray-100 pt-1.5 max-[359px]:grid max-[359px]:grid-cols-2">
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
              className="max-[359px]:justify-self-end"
              onClick={onClearSelection}
              disabled={
                selectedCount === 0 ||
                disabled ||
                onClearSelection === undefined
              }
              aria-label="Auswahl aufheben"
              tooltip={CHECKIN_HINTS.clearSelection}
              disabledReason={
                selectedCount === 0
                  ? "Es ist noch kein Kind ausgewählt."
                  : undefined
              }
            >
              <X className="size-3.5" aria-hidden />
            </Button>
            <Button
              type="button"
              variant="success"
              size="compact"
              className="max-[359px]:w-full"
              onClick={() => onBulkAction?.("in")}
              disabled={
                selectedCount === 0 || disabled || onBulkAction === undefined
              }
              tooltip={CHECKIN_HINTS.checkIn}
              disabledReason={
                selectedCount === 0 ? CHECKIN_HINTS.noneSelected : undefined
              }
            >
              {runningAction === "in" ? (
                <SpinnerIcon className="size-3.5" />
              ) : (
                <LogIn className="size-3.5" aria-hidden />
              )}
              Anmelden
            </Button>
            <Button
              type="button"
              variant="danger"
              size="compact"
              className="max-[359px]:w-full"
              onClick={() => onBulkAction?.("out")}
              disabled={
                selectedCount === 0 || disabled || onBulkAction === undefined
              }
              tooltip={CHECKIN_HINTS.checkOut}
              disabledReason={
                selectedCount === 0 ? CHECKIN_HINTS.noneSelected : undefined
              }
            >
              {runningAction === "out" ? (
                <SpinnerIcon className="size-3.5" />
              ) : (
                <LogOut className="size-3.5" aria-hidden />
              )}
              Abmelden
            </Button>
          </div>
        ) : null}

        {leftOut ? (
          <p className="mt-1 text-xs text-gray-600">{leftOut}</p>
        ) : null}
      </SelectionBar>
    </div>
  );
}
