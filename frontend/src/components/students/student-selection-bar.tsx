"use client";

import { CheckSquare, Download, Loader2, LogIn, LogOut } from "lucide-react";

import { Button } from "~/components/ui/button";
import type { SchoolCheckinAction } from "~/lib/student-api";

interface StudentSelectionBarProps {
  /** Marked children that are on screen right now. */
  readonly selectedCount: number;
  readonly onClear: () => void;
  /**
   * Bulk Anmelden/Abmelden. Omit where the page may not check children in or
   * out (web attendance off, another day than today); the buttons are then
   * not shown at all.
   */
  readonly checkin?: {
    readonly onAction: (action: SchoolCheckinAction) => void;
    readonly runningAction: SchoolCheckinAction | null;
    readonly isRunning: boolean;
  };
  readonly onExport: () => void;
}

/**
 * Actions for the children marked in the table view (#3834). Appears as soon
 * as one child is marked and stays at the top while the list scrolls, so the
 * actions are in reach after walking down a long table.
 */
export function StudentSelectionBar({
  selectedCount,
  onClear,
  checkin,
  onExport,
}: StudentSelectionBarProps) {
  if (selectedCount === 0) return null;
  const isRunning = checkin?.isRunning ?? false;

  return (
    <div
      className="moto-content-surface sticky top-2 z-30 mb-3 rounded-xl border px-3 py-2 shadow-sm"
      role="region"
      aria-label="Ausgewählte Kinder"
      aria-busy={isRunning}
    >
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
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
            className="shadow-none"
            onClick={onClear}
            disabled={isRunning}
          >
            Aufheben
          </Button>
          <Button
            type="button"
            variant="outline"
            size="compact"
            className="rounded-lg shadow-none"
            onClick={onExport}
            disabled={isRunning}
          >
            <Download className="h-3.5 w-3.5" aria-hidden />
            Exportieren
          </Button>
          {checkin ? (
            <>
              <Button
                type="button"
                variant="success"
                size="compact"
                className="rounded-lg text-white shadow-none hover:shadow-none"
                onClick={() => checkin.onAction("in")}
                disabled={isRunning}
              >
                {checkin.runningAction === "in" ? (
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
                onClick={() => checkin.onAction("out")}
                disabled={isRunning}
              >
                {checkin.runningAction === "out" ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
                ) : (
                  <LogOut className="h-3.5 w-3.5" aria-hidden />
                )}
                Abmelden
              </Button>
            </>
          ) : null}
        </div>
      </div>
    </div>
  );
}
