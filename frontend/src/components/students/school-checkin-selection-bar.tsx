"use client";

import { LogIn, LogOut } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { SelectionBar } from "~/components/ui/selection-bar";
import {
  CHECKIN_HINTS,
  selectAllBlockedReason,
} from "~/components/students/school-checkin-hints";
import {
  leftOutLabel,
  type SelectAllControl,
} from "~/components/students/school-checkin-select-all";
import { SpinnerIcon } from "~/components/ui/icons";

interface SchoolCheckinSelectionBarProps {
  /** Whether the selection sub-mode is on ("Mehrere") vs. immediate taps ("Direkt"). */
  readonly selectionActive: boolean;
  readonly onSelectionActiveChange: (active: boolean) => void;
  /** Number of currently marked students. */
  readonly selectedCount: number;
  readonly onClearSelection: () => void;
  /** Fires the bulk action for all marked students. */
  readonly onBulkAction: (action: "in" | "out") => void;
  readonly onFinish: () => void;
  /** True while a bulk API call is in flight — actions are locked then. */
  readonly isRunning: boolean;
  /** The action currently running, so only its button shows a spinner. */
  readonly runningAction?: "in" | "out" | null;
  /** „Alle N auswählen“ for the children on screen (#4762). */
  readonly selectAll?: SelectAllControl;
}

/**
 * Sub-mode bar of the check-in/out mode on the student search page (#2359).
 *
 * Rendered only while the An-/Abmelde-Modus is active. Offers the switch
 * between the door-operation "Direkt" behavior (every tap fires immediately,
 * unchanged since #2220) and the "Mehrere" behavior, where taps only mark
 * children and the explicit Anmelden/Abmelden buttons here execute the batch.
 *
 * Sticky at the viewport top so the actions stay reachable while scrolling
 * a long list — the selection use case (Verabschiedungskreis) is exactly the
 * one where the user walks through many cards before acting.
 */
export function SchoolCheckinSelectionBar({
  selectionActive,
  onSelectionActiveChange,
  selectedCount,
  onClearSelection,
  onBulkAction,
  onFinish,
  isRunning,
  runningAction = null,
  selectAll,
}: SchoolCheckinSelectionBarProps) {
  const noneSelected = selectedCount === 0;
  const leftOut = selectAll ? leftOutLabel(selectAll.leftOutCount) : null;

  const segmentedControl = (
    <SegmentedControl
      ariaLabel="Tipp-Verhalten"
      items={[
        // Locked while a batch is in flight: switching to "Direkt" clears
        // the selection, which would discard the failed IDs the response
        // is about to keep marked for a one-tap retry (review #2372).
        { value: "immediate", label: "Direkt", disabled: isRunning },
        { value: "select", label: "Mehrere", disabled: isRunning },
      ]}
      value={selectionActive ? "select" : "immediate"}
      onChange={(next) => onSelectionActiveChange(next === "select")}
    />
  );
  const finishButton = (
    <Button
      type="button"
      variant="primary"
      size="compact"
      className={selectionActive ? "ml-1" : undefined}
      onClick={onFinish}
      disabled={isRunning}
      tooltip={CHECKIN_HINTS.finish}
    >
      Fertig
    </Button>
  );

  return (
    <SelectionBar
      aria-label="An- und Abmelde-Modus"
      data-checkin-selection-bar="true"
      className="hidden md:block"
      count={selectedCount}
      busy={isRunning || runningAction !== null}
      label="An- und Abmelde-Modus aktiv"
      onClear={selectionActive ? onClearSelection : undefined}
      extra={
        selectionActive ? (
          <>
            {segmentedControl}
            <span
              className="text-sm font-semibold text-gray-900 tabular-nums"
              aria-live="polite"
            >
              {selectedCount} ausgewählt
            </span>
            {selectAll ? (
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={selectAll.onSelectAll}
                disabled={
                  selectAll.count === 0 || selectAll.allSelected || isRunning
                }
                tooltip={CHECKIN_HINTS.selectAll}
                disabledReason={selectAllBlockedReason(selectAll)}
              >
                Alle {selectAll.count} auswählen
              </Button>
            ) : null}
            {leftOut ? (
              <span className="text-xs text-gray-600">{leftOut}</span>
            ) : null}
          </>
        ) : (
          <>
            {segmentedControl}
            <span className="min-w-0 flex-1 text-xs text-gray-500">
              Jeder Tipp meldet sofort an oder ab.
            </span>
          </>
        )
      }
      actions={
        selectionActive ? (
          <>
            <Button
              type="button"
              variant="success"
              size="compact"
              onClick={() => onBulkAction("in")}
              disabled={noneSelected || isRunning}
              tooltip={CHECKIN_HINTS.checkIn}
              disabledReason={
                noneSelected ? CHECKIN_HINTS.noneSelected : undefined
              }
            >
              {runningAction === "in" ? (
                <SpinnerIcon className="h-3.5 w-3.5" />
              ) : (
                <LogIn className="h-3.5 w-3.5" aria-hidden />
              )}
              Anmelden
            </Button>
            <Button
              type="button"
              variant="danger"
              size="compact"
              onClick={() => onBulkAction("out")}
              disabled={noneSelected || isRunning}
              tooltip={CHECKIN_HINTS.checkOut}
              disabledReason={
                noneSelected ? CHECKIN_HINTS.noneSelected : undefined
              }
            >
              {runningAction === "out" ? (
                <SpinnerIcon className="h-3.5 w-3.5" />
              ) : (
                <LogOut className="h-3.5 w-3.5" aria-hidden />
              )}
              Abmelden
            </Button>
            {finishButton}
          </>
        ) : (
          finishButton
        )
      }
    />
  );
}
