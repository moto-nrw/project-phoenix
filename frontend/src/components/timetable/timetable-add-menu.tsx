"use client";

/**
 * TimetableAddMenu: the single "+ Neu" entry point in the timetable toolbar.
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
          {materializing ? "Plane ..." : replanning ? "Berechne ..." : "Neu"}
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
              Termin anlegen
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
                  Einmaliger Termin
                </div>
                <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                  Findet nur an diesem Tag statt.
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
                <div className="text-sm font-semibold text-gray-900">
                  Regelmäßiger Termin
                </div>
                <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                  Für Angebote, die jede Woche oder alle zwei Wochen
                  stattfinden.
                </p>
              </div>
            </button>

            {planning && (
              <>
                <div className="my-1 h-px bg-gray-100" />
                <p className="px-3 pt-2 pb-1 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">
                  Regeltermine übernehmen
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
                      Fehlende Termine eintragen
                    </div>
                    <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                      Trägt Regeltermine ein, die in dieser Woche noch fehlen.
                      Bestehende Termine bleiben unverändert.
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
                      Regeltermine neu aufbauen
                    </div>
                    <p className="mt-0.5 text-[11px] leading-relaxed text-gray-500">
                      Nutze das nach Änderungen an Regelterminen. Noch nicht
                      gestartete Regeltermine werden ersetzt. Laufende,
                      abgesagte und einzelne Termine bleiben erhalten.
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
          title="Regeltermine neu aufbauen?"
          confirmText={replanning ? "Berechne ..." : "Neu aufbauen"}
          cancelText="Abbrechen"
          isConfirmLoading={replanning}
        >
          <p className="text-sm leading-relaxed text-gray-600">
            Noch nicht gestartete Regeltermine in{" "}
            <span className="font-semibold text-gray-900">
              {planning.weekLabel}
            </span>{" "}
            werden entfernt und anhand der aktuellen Regeltermine neu
            eingetragen. Laufende, abgesagte und einzelne Termine bleiben
            erhalten.
          </p>
        </ConfirmationModal>
      )}
    </>
  );
}
