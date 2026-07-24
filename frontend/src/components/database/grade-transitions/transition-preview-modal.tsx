"use client";

// Preview + apply step for a Jahrgangswechsel draft: shows exactly what will
// happen (per-class counts, Abgänge, unmapped classes) and requires an
// explicit second confirmation before applying.

import { useEffect, useState } from "react";
import { Button } from "~/components/ui/button";
import { Loading } from "~/components/ui/loading";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import { createLogger } from "~/lib/logger";
import {
  applyGradeTransition,
  previewGradeTransition,
  type GradeTransition,
  type TransitionPreview,
  type TransitionResult,
} from "~/lib/grade-transition-api";

const logger = createLogger({ component: "GradeTransitionPreviewModal" });

interface TransitionPreviewModalProps {
  readonly transition: GradeTransition;
  readonly onClose: () => void;
  readonly onBackToEditor: () => void;
  readonly onApplied: (result: TransitionResult) => void;
}

export function TransitionPreviewModal({
  transition,
  onClose,
  onBackToEditor,
  onApplied,
}: TransitionPreviewModalProps) {
  const [preview, setPreview] = useState<TransitionPreview | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [applying, setApplying] = useState(false);
  const [applyError, setApplyError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    previewGradeTransition(transition.id)
      .then((data) => {
        if (!cancelled) setPreview(data);
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        logger.error("preview_load_failed", {
          transition_id: transition.id,
          error: error instanceof Error ? error.message : String(error),
        });
        setLoadError("Vorschau konnte nicht geladen werden.");
      });
    return () => {
      cancelled = true;
    };
  }, [transition.id]);

  const handleApply = async () => {
    setApplying(true);
    setApplyError(null);
    try {
      const result = await applyGradeTransition(transition.id);
      onApplied(result);
    } catch (error) {
      logger.error("apply_failed", {
        transition_id: transition.id,
        error: error instanceof Error ? error.message : String(error),
      });
      setApplyError(
        "Der Jahrgangswechsel konnte nicht angewendet werden. Bitte erneut versuchen.",
      );
      setApplying(false);
      setConfirmOpen(false);
    }
  };

  return (
    <>
      <Modal
        isOpen
        onClose={onClose}
        title={`Vorschau: Jahrgangswechsel ${transition.academicYear}`}
        widthClass="mx-4 w-[calc(100%-2rem)] max-w-3xl"
        footer={
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={onBackToEditor}
            >
              Zurück zum Entwurf
            </Button>
            <Button
              type="button"
              size="md"
              onClick={() => setConfirmOpen(true)}
              disabled={!preview || applying}
            >
              Jahrgangswechsel anwenden
            </Button>
          </div>
        }
      >
        {!preview && !loadError && <Loading fullPage={false} />}
        {loadError && <p className="text-sm text-[#FF3130]">{loadError}</p>}

        {preview && (
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-xl border border-gray-200 bg-white p-3 text-center shadow-sm">
                <p className="text-2xl font-semibold text-gray-900">
                  {preview.totalStudents}
                </p>
                <p className="text-xs text-gray-500">Kinder gesamt</p>
              </div>
              <div className="rounded-xl border border-gray-200 bg-white p-3 text-center shadow-sm">
                <p className="text-2xl font-semibold text-[#83CD2D]">
                  {preview.toPromote}
                </p>
                <p className="text-xs text-gray-500">werden versetzt</p>
              </div>
              <div className="rounded-xl border border-gray-200 bg-white p-3 text-center shadow-sm">
                <p className="text-2xl font-semibold text-[#FF3130]">
                  {preview.toGraduate}
                </p>
                <p className="text-xs text-gray-500">Abgänge</p>
              </div>
            </div>

            <ul className="divide-y divide-gray-100 rounded-xl border border-gray-200">
              {preview.byMapping.map((m) => (
                <li
                  key={m.fromClass}
                  className="flex items-center justify-between p-3 text-sm"
                >
                  <span className="font-medium text-gray-900">
                    {m.fromClass}
                    {" -> "}
                    {m.toClass ?? "Abgang"}
                  </span>
                  <span className="text-gray-500">
                    {m.studentCount} {m.studentCount === 1 ? "Kind" : "Kinder"}
                  </span>
                </li>
              ))}
            </ul>

            {preview.toGraduate > 0 && (
              <div className="rounded-xl border border-[#FF3130]/30 bg-[#FF3130]/5 p-3 text-sm text-gray-800">
                <p className="font-semibold text-[#FF3130]">
                  {preview.toGraduate}{" "}
                  {preview.toGraduate === 1
                    ? "Kind verlässt die OGS"
                    : "Kinder verlassen die OGS"}
                </p>
                <p className="mt-1">
                  Diese Kinder werden nach dem Anwenden in der App nicht mehr
                  angezeigt. Über Zurücksetzen lässt sich das wiederherstellen.
                </p>
              </div>
            )}

            {preview.unmappedClasses.length > 0 && (
              <div className="rounded-xl border border-[#F78C10]/30 bg-[#F78C10]/5 p-3 text-sm text-gray-800">
                <p className="font-semibold text-[#F78C10]">
                  Nicht berücksichtigte Klassen
                </p>
                <p className="mt-1">
                  {preview.unmappedClasses
                    .map((u) => `${u.className} (${u.studentCount})`)
                    .join(", ")}{" "}
                  bleiben unverändert.
                </p>
              </div>
            )}

            {applyError && (
              <p className="text-sm text-[#FF3130]">{applyError}</p>
            )}
          </div>
        )}
      </Modal>

      <ConfirmationModal
        isOpen={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={handleApply}
        title="Wirklich anwenden?"
        confirmText="Ja, anwenden"
        cancelText="Abbrechen"
        isConfirmLoading={applying}
        isDismissDisabled={applying}
      >
        <p>
          Der Jahrgangswechsel {transition.academicYear} wird jetzt für alle
          betroffenen Kinder durchgeführt.
          {preview && preview.toGraduate > 0 && (
            <>
              {" "}
              {preview.toGraduate}{" "}
              {preview.toGraduate === 1 ? "Kind wird" : "Kinder werden"} dabei
              als Abgang aus der App ausgeblendet.
            </>
          )}
        </p>
      </ConfirmationModal>
    </>
  );
}
