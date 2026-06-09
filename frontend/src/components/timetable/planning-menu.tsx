"use client";

/**
 * PlanningMenu — single entry point for both planning actions in the
 * timetable. Replaces the standalone MaterializeButton and the inline
 * "Woche neu berechnen" trigger in PlanQualityPanel.
 *
 * The two flows look almost identical to a new admin but have very
 * different consequences:
 *
 * - "Lücken füllen" (Materialize): additive. Creates missing instances
 *   from series, leaves anything that already exists alone.
 * - "Woche neu berechnen" (Replan): destructive. Deletes planned
 *   series instances in the window and re-creates them from the current
 *   series. Active/cancelled/manually-added survive.
 *
 * Both options live behind one button so users see the choice up front
 * with descriptions, instead of accidentally picking the destructive
 * path. The destructive path still requires explicit confirmation in a
 * modal dialog (`ReplanConfirmDialog`).
 */

import { useRef, useState } from "react";
import { CalendarPlus, ChevronDown, RefreshCw } from "lucide-react";

import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";

interface PlanningMenuProps {
  onMaterialize: () => Promise<void>;
  onReplan: () => Promise<void>;
  weekLabel: string;
  disabled?: boolean;
  /** When true, renders the trigger as a primary gray-900 solid button. */
  emphasis?: "primary" | "secondary" | "accent";
  label?: string;
}

export function PlanningMenu({
  onMaterialize,
  onReplan,
  weekLabel,
  disabled = false,
  emphasis = "secondary",
  label = "Woche planen",
}: PlanningMenuProps) {
  const [open, setOpen] = useState(false);
  const [materializing, setMaterializing] = useState(false);
  const [showReplanDialog, setShowReplanDialog] = useState(false);
  const [replanning, setReplanning] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useClickOutside(containerRef, () => setOpen(false), open);

  const handleMaterialize = () => {
    setOpen(false);
    setMaterializing(true);
    void onMaterialize().finally(() => setMaterializing(false));
  };

  const handleReplan = () => {
    setReplanning(true);
    void onReplan().finally(() => {
      setReplanning(false);
      setShowReplanDialog(false);
    });
  };

  const isPending = materializing || replanning;
  // The planning CTA uses the standard primary (gray-900) like every other
  // primary button; "secondary" falls back to the kit outline variant.
  const triggerVariant = emphasis === "secondary" ? "outline" : "primary";

  return (
    <>
      <div className="relative w-full sm:w-auto" ref={containerRef}>
        <Button
          type="button"
          variant={triggerVariant}
          size="compact"
          onClick={() => setOpen((v) => !v)}
          disabled={disabled || isPending}
          aria-haspopup="menu"
          aria-expanded={open}
          className="w-full sm:w-auto"
        >
          {isPending ? (
            <RefreshCw className="h-3.5 w-3.5 animate-spin" aria-hidden />
          ) : (
            <CalendarPlus className="h-3.5 w-3.5" aria-hidden />
          )}
          {materializing ? "Plane …" : replanning ? "Berechne …" : label}
          <ChevronDown
            className={`h-3.5 w-3.5 transition-transform ${open ? "rotate-180" : ""}`}
            aria-hidden
          />
        </Button>

        {open && (
          <div
            role="menu"
            aria-label="Planungsoptionen"
            className="absolute right-0 z-30 mt-2 w-[min(20rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg"
          >
            <button
              type="button"
              role="menuitem"
              onClick={handleMaterialize}
              className="flex w-full items-start gap-3 px-3 py-3 text-left transition-colors hover:bg-gray-50"
            >
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100">
                <CalendarPlus className="h-4 w-4 text-gray-700" aria-hidden />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-gray-900">
                  Lücken füllen
                </div>
                <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                  Legt fehlende Termine aus den Serien an. Bestehende Termine
                  werden nicht geändert.
                </p>
              </div>
            </button>

            <div className="h-px bg-gray-100" />

            <button
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                setShowReplanDialog(true);
              }}
              className="flex w-full items-start gap-3 px-3 py-3 text-left transition-colors hover:bg-gray-50"
            >
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[#EAB308]/10">
                <RefreshCw className="h-4 w-4 text-[#EAB308]" aria-hidden />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-gray-900">
                  Woche neu berechnen
                </div>
                <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                  Löscht noch nicht gestartete Serientermine dieser Woche und
                  erstellt sie neu aus den Serien. Laufende, abgesagte und
                  manuell angelegte Termine bleiben erhalten.
                </p>
              </div>
            </button>
          </div>
        )}
      </div>

      <ConfirmationModal
        isOpen={showReplanDialog}
        onClose={() => setShowReplanDialog(false)}
        onConfirm={handleReplan}
        title="Woche neu berechnen?"
        confirmText={replanning ? "Berechne …" : "Neu berechnen"}
        cancelText="Abbrechen"
        isConfirmLoading={replanning}
      >
        <p className="text-sm leading-relaxed text-gray-600">
          Noch nicht gestartete Serientermine in{" "}
          <span className="font-semibold text-gray-900">{weekLabel}</span>{" "}
          werden gelöscht und anhand der aktuellen Serien neu erstellt.
          Laufende, abgesagte und manuell angelegte Termine bleiben erhalten.
        </p>
      </ConfirmationModal>
    </>
  );
}
