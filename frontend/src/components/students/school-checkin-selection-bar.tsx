"use client";

import { CheckSquare, Loader2, LogIn, LogOut } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";

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
}: SchoolCheckinSelectionBarProps) {
  const noneSelected = selectedCount === 0;

  return (
    <div
      className="moto-content-surface sticky top-2 z-30 mb-3 hidden rounded-xl border px-3 py-2 shadow-sm md:block"
      role="region"
      aria-label="An- und Abmelde-Modus"
      aria-busy={isRunning || runningAction !== null}
      data-checkin-selection-bar="true"
    >
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
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

        {selectionActive ? (
          <>
            <span
              className="hidden items-center gap-1.5 text-sm font-semibold text-gray-900 tabular-nums md:inline-flex"
              aria-live="polite"
            >
              <CheckSquare
                className="text-moto-green h-4 w-4 shrink-0"
                aria-hidden
              />
              {selectedCount} ausgewählt
            </span>
            <div className="ml-auto flex items-center gap-1.5">
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="shadow-none"
                onClick={onClearSelection}
                disabled={noneSelected || isRunning}
              >
                Aufheben
              </Button>
              <Button
                type="button"
                variant="success"
                size="compact"
                className="rounded-lg text-white shadow-none hover:shadow-none"
                onClick={() => onBulkAction("in")}
                disabled={noneSelected || isRunning}
              >
                {runningAction === "in" ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
                ) : (
                  <LogIn className="h-3.5 w-3.5" aria-hidden />
                )}
                Anmelden
              </Button>
              <Button
                type="button"
                variant="danger"
                size="compact"
                className="rounded-lg shadow-none hover:shadow-none"
                onClick={() => onBulkAction("out")}
                disabled={noneSelected || isRunning}
              >
                {runningAction === "out" ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
                ) : (
                  <LogOut className="h-3.5 w-3.5" aria-hidden />
                )}
                Abmelden
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="compact"
                className="ml-1 rounded-full shadow-none"
                onClick={onFinish}
                disabled={isRunning}
              >
                Fertig
              </Button>
            </div>
          </>
        ) : (
          <>
            <span className="hidden text-xs text-gray-500 sm:inline">
              Jeder Tipp meldet sofort an oder ab.
            </span>
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="ml-auto rounded-full shadow-none"
              onClick={onFinish}
              disabled={isRunning}
            >
              Fertig
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
