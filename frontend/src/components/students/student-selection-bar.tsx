"use client";

import { Download, LogIn, LogOut } from "lucide-react";

import { Button } from "~/components/ui/button";
import { SelectionBar } from "~/components/ui/selection-bar";
import type { SchoolCheckinAction } from "~/lib/student-api";
import { SpinnerIcon } from "~/components/ui/icons";

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
    <SelectionBar
      aria-label="Ausgewählte Kinder"
      count={selectedCount}
      busy={isRunning}
      onClear={onClear}
      actions={
        <>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={onExport}
            disabled={isRunning}
            tooltip="Lässt Sie wählen, welche Angaben der Kinder Sie herunterladen."
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
                onClick={() => checkin.onAction("in")}
                disabled={isRunning}
                tooltip="Meldet die ausgewählten Kinder sofort als anwesend an."
              >
                {checkin.runningAction === "in" ? (
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
                onClick={() => checkin.onAction("out")}
                disabled={isRunning}
                tooltip="Meldet die Kinder ab, bei Raumbesuchen fragt moto vorher nach."
              >
                {checkin.runningAction === "out" ? (
                  <SpinnerIcon className="h-3.5 w-3.5" />
                ) : (
                  <LogOut className="h-3.5 w-3.5" aria-hidden />
                )}
                Abmelden
              </Button>
            </>
          ) : null}
        </>
      }
    />
  );
}
