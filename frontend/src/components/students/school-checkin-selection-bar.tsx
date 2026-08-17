"use client";

import { CheckSquare, Loader2, LogIn, LogOut } from "lucide-react";
import { Button } from "~/components/ui/button";
import { SegmentedControl } from "~/components/ui/segmented-control";

export type SchoolCheckinTapMode = "immediate" | "select";

interface SchoolCheckinSelectionBarProps {
  /** Whether the selection sub-mode is on ("Auswahl") vs. immediate taps ("Sofort"). */
  readonly selectionActive: boolean;
  readonly onSelectionActiveChange: (active: boolean) => void;
  /** Number of currently marked students. */
  readonly selectedCount: number;
  readonly onClearSelection: () => void;
  /** Fires the bulk action for all marked students. */
  readonly onBulkAction: (action: "in" | "out") => void;
  /** True while a bulk API call is in flight — actions are locked then. */
  readonly isRunning: boolean;
}

/**
 * Sub-mode bar of the check-in/out mode on the student search page (#2359).
 *
 * Rendered only while the An-/Abmelde-Modus is active. Offers the switch
 * between the door-operation "Sofort" behavior (every tap fires immediately,
 * unchanged since #2220) and the "Auswahl" behavior, where taps only mark
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
  isRunning,
}: SchoolCheckinSelectionBarProps) {
  const noneSelected = selectedCount === 0;

  return (
    <div
      className="moto-content-surface sticky top-2 z-30 mb-3 rounded-xl border px-3 py-2 shadow-sm"
      role="region"
      aria-label="An- und Abmelde-Modus"
      data-checkin-selection-bar="true"
    >
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <SegmentedControl
          ariaLabel="Tipp-Verhalten"
          items={[
            { value: "immediate", label: "Sofort" },
            { value: "select", label: "Auswahl" },
          ]}
          value={selectionActive ? "select" : "immediate"}
          onChange={(next) => onSelectionActiveChange(next === "select")}
        />

        {selectionActive ? (
          <>
            <span
              className="inline-flex items-center gap-1.5 text-sm font-semibold text-gray-900 tabular-nums"
              aria-live="polite"
            >
              <CheckSquare
                className="text-moto-green h-4 w-4 shrink-0"
                aria-hidden
              />
              {selectedCount} ausgewählt
            </span>
            <div className="ml-auto flex flex-wrap items-center gap-1.5">
              <Button
                type="button"
                variant="ghost"
                size="compact"
                onClick={onClearSelection}
                disabled={noneSelected || isRunning}
              >
                Aufheben
              </Button>
              <Button
                type="button"
                variant="success"
                size="compact"
                onClick={() => onBulkAction("in")}
                disabled={noneSelected || isRunning}
              >
                {isRunning ? (
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
                onClick={() => onBulkAction("out")}
                disabled={noneSelected || isRunning}
              >
                {isRunning ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
                ) : (
                  <LogOut className="h-3.5 w-3.5" aria-hidden />
                )}
                Abmelden
              </Button>
            </div>
          </>
        ) : (
          <span className="text-xs text-gray-500">
            Jeder Tipp meldet sofort an oder ab.
          </span>
        )}
      </div>
      {selectionActive && noneSelected ? (
        <p className="mt-1.5 text-xs text-gray-500">
          Tippe auf Kinder, um sie auszuwählen. Erst „Anmelden“ oder „Abmelden“
          führt die Aktion für alle markierten Kinder aus.
        </p>
      ) : null}
    </div>
  );
}
