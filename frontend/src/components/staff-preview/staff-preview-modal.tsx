"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { formErrorMessage } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { Loading } from "~/components/ui/loading";
import { mutate } from "~/lib/swr";
import {
  fetchStaffPreviewCandidates,
  performStartStaffPreview,
  type StaffPreviewCandidate,
} from "~/lib/staff-preview-api";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StaffPreviewModal" });

interface StaffPreviewModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}

function candidateLabel(candidate: StaffPreviewCandidate): string {
  const name = `${candidate.firstName} ${candidate.lastName}`.trim();
  return name || candidate.email;
}

/**
 * Auswahl-Dialog für die Mitarbeiter-Vorschau (#2893): der Admin wählt eine
 * Person der eigenen Schule, danach zeigt moto deren Ansicht — nur lesend.
 */
export function StaffPreviewModal({ isOpen, onClose }: StaffPreviewModalProps) {
  const { update } = useSession();
  const [candidates, setCandidates] = useState<StaffPreviewCandidate[] | null>(
    null,
  );
  const [selectedId, setSelectedId] = useState("");
  const [isStarting, setIsStarting] = useState(false);
  // Erhöht sich bei „Wiederholen“ und lädt die Liste neu.
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [loadFailed, setLoadFailed] = useState(false);
  // Fehler bleiben im offenen Dialog (#2517): Laden mit Wiederholen,
  // Starten im Fehlerkasten über der Auswahl.
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const startError = useApiFormError();
  const clearStartError = startError.clear;
  const latestStartRef = useRef<() => void>(() => undefined);

  useEffect(() => {
    if (!isOpen) return;
    clearLoadError();
    clearStartError();
    setLoadFailed(false);
    setSelectedId("");
    setCandidates(null);
    let cancelled = false;
    fetchStaffPreviewCandidates()
      .then((result) => {
        if (!cancelled) setCandidates(result);
      })
      .catch((err: unknown) => {
        logger.error("staff_preview_candidates_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          setCandidates([]);
          setLoadFailed(true);
          void showLoadError(err, {
            object: "die Liste der Personen",
            retry: () => setLoadAttempt((attempt) => attempt + 1),
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, loadAttempt, showLoadError, clearLoadError, clearStartError]);

  const handleStart = async () => {
    if (!selectedId || isStarting) return;
    setIsStarting(true);
    startError.clear();
    try {
      await performStartStaffPreview(selectedId, update, mutate);
      // Volle Neuladung auf der aktuellen Seite: ab jetzt rendert alles mit
      // den Rechten der gewählten Person.
      window.location.reload();
    } catch (err) {
      logger.error("staff_preview_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setIsStarting(false);
      await startError.show(err, {
        object: "die Vorschau",
        retry: () => latestStartRef.current(),
      });
    }
  };
  // „Wiederholen“ startet mit der dann gewählten Person.
  useLayoutEffect(() => {
    latestStartRef.current = () => void handleStart();
  });

  const options = (candidates ?? []).map((candidate) => ({
    value: candidate.accountId,
    label: candidateLabel(candidate),
  }));

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Ansicht eines Mitarbeitenden"
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void handleStart()}
            disabled={!selectedId || isStarting}
            isLoading={isStarting}
            loadingText="Wird gestartet …"
          >
            Vorschau starten
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          Wählen Sie eine Person. Sie sehen moto danach so wie diese Person. Sie
          können dabei nur lesen, nichts ändern.
        </p>

        <FormErrorAlert message={startError.error} />
        <LoadErrorAlert error={loadError.error} />

        {/* Bis der Fehlertext da ist, bleibt der Ladezustand stehen, nie
            „Keine Person gefunden“ für eine Liste, die nie geladen wurde. */}
        {candidates === null ||
        (loadFailed && formErrorMessage(loadError.error) === null) ? (
          <Loading fullPage={false} />
        ) : candidates.length === 0 && !loadFailed ? (
          <p className="text-sm text-gray-500">
            Keine Person gefunden, die Sie ansehen können.
          </p>
        ) : candidates.length > 0 ? (
          <CustomSelect
            value={selectedId}
            options={options}
            onChange={setSelectedId}
            placeholder="Person auswählen"
            ariaLabel="Person für die Vorschau auswählen"
          />
        ) : null}
      </div>
    </Modal>
  );
}
