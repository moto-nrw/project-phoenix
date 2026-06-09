"use client";

/**
 * TimetableAddMenu — the single "+ Neu" entry point in the timetable toolbar.
 *
 * Folds what used to be two separate toolbar buttons ("+ Termin" and the
 * "Angebote einplanen" PlanningMenu) into one compact menu so the toolbar fits
 * on a single row. The menu has two sections:
 *
 * - "Hinzufügen": create a single appointment (Einzeltermin) or a recurring
 *   series (Serie). Always available, in every view.
 * - "Woche planen" (week view + full period coverage only): the two
 *   materialize flows that previously lived in PlanningMenu —
 *     • "Lücken füllen" (additive: creates missing instances from series)
 *     • "Woche neu berechnen" (destructive: deletes not-yet-started series
 *       instances and re-creates them; gated behind a confirmation dialog).
 *
 * The destructive path keeps the explicit ConfirmationModal so it can never be
 * triggered by a single mis-click.
 */

import { useRef, useState } from "react";
import {
  CalendarPlus,
  ChevronDown,
  Plus,
  RefreshCw,
  Repeat,
} from "lucide-react";

import { useClickOutside } from "~/lib/hooks/use-click-outside";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";

interface PlanningActions {
  onMaterialize: () => Promise<void>;
  onReplan: () => Promise<void>;
  weekLabel: string;
}

interface TimetableAddMenuProps {
  /** Create a single one-off appointment. */
  onAddInstance: () => void;
  /** Create a recurring series. */
  onAddSeries: () => void;
  /**
   * Week-planning actions. Only provided in week view with full period
   * coverage; when omitted the "Woche planen" section is hidden.
   */
  planning?: PlanningActions;
  disabled?: boolean;
}

export function TimetableAddMenu({
  onAddInstance,
  onAddSeries,
  planning,
  disabled = false,
}: TimetableAddMenuProps) {
  const [open, setOpen] = useState(false);
  const [materializing, setMaterializing] = useState(false);
  const [showReplanDialog, setShowReplanDialog] = useState(false);
  const [replanning, setReplanning] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useClickOutside(containerRef, () => setOpen(false), open);

  const handleMaterialize = () => {
    if (!planning) return;
    setOpen(false);
    setMaterializing(true);
    void planning.onMaterialize().finally(() => setMaterializing(false));
  };

  const handleReplan = () => {
    if (!planning) return;
    setReplanning(true);
    void planning.onReplan().finally(() => {
      setReplanning(false);
      setShowReplanDialog(false);
    });
  };

  const isPending = materializing || replanning;

  return (
    <>
      <div className="relative w-full sm:w-auto" ref={containerRef}>
        <Button
          type="button"
          variant="primary"
          size="compact"
          onClick={() => setOpen((v) => !v)}
          disabled={disabled || isPending}
          aria-haspopup="menu"
          aria-expanded={open}
          className="w-full rounded-lg sm:w-auto"
        >
          {isPending ? (
            <RefreshCw className="h-3.5 w-3.5 animate-spin" aria-hidden />
          ) : (
            <Plus className="h-3.5 w-3.5" aria-hidden />
          )}
          {materializing ? "Plane …" : replanning ? "Berechne …" : "Neu"}
          <ChevronDown
            className={`h-3.5 w-3.5 transition-transform ${open ? "rotate-180" : ""}`}
            aria-hidden
          />
        </Button>

        {open && (
          <div
            role="menu"
            aria-label="Neu anlegen"
            className="absolute right-0 z-30 mt-2 w-[min(20rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-gray-200 bg-white shadow-lg"
          >
            <p className="px-3 pt-3 pb-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
              Hinzufügen
            </p>

            <button
              type="button"
              role="menuitem"
              onClick={() => {
                setOpen(false);
                onAddInstance();
              }}
              className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-gray-50"
            >
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100">
                <CalendarPlus className="h-4 w-4 text-gray-700" aria-hidden />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-gray-900">
                  Einzeltermin
                </div>
                <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                  Einen einzelnen Termin anlegen.
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
              className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-gray-50"
            >
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-gray-100">
                <Repeat className="h-4 w-4 text-gray-700" aria-hidden />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-semibold text-gray-900">Serie</div>
                <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                  Eine wiederkehrende Serie anlegen.
                </p>
              </div>
            </button>

            {planning && (
              <>
                <div className="my-1 h-px bg-gray-100" />
                <p className="px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
                  Woche planen
                </p>

                <button
                  type="button"
                  role="menuitem"
                  onClick={handleMaterialize}
                  className="flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-gray-50"
                >
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[#83CD2D]/10">
                    <CalendarPlus
                      className="h-4 w-4 text-[#83CD2D]"
                      aria-hidden
                    />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-semibold text-gray-900">
                      Lücken füllen
                    </div>
                    <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                      Legt fehlende Termine aus den Serien an. Bestehende
                      Termine werden nicht geändert.
                    </p>
                  </div>
                </button>

                <button
                  type="button"
                  role="menuitem"
                  onClick={() => {
                    setOpen(false);
                    setShowReplanDialog(true);
                  }}
                  className="flex w-full items-start gap-3 px-3 py-2.5 text-left transition-colors hover:bg-gray-50"
                >
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md bg-[#EAB308]/10">
                    <RefreshCw className="h-4 w-4 text-[#EAB308]" aria-hidden />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-semibold text-gray-900">
                      Woche neu berechnen
                    </div>
                    <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                      Löscht noch nicht gestartete Serientermine dieser Woche
                      und erstellt sie neu aus den Serien. Laufende, abgesagte
                      und manuell angelegte Termine bleiben erhalten.
                    </p>
                  </div>
                </button>
              </>
            )}
          </div>
        )}
      </div>

      {planning && (
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
            <span className="font-semibold text-gray-900">
              {planning.weekLabel}
            </span>{" "}
            werden gelöscht und anhand der aktuellen Serien neu erstellt.
            Laufende, abgesagte und manuell angelegte Termine bleiben erhalten.
          </p>
        </ConfirmationModal>
      )}
    </>
  );
}
