"use client";

import { useEffect, useState } from "react";
import { useSession } from "next-auth/react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { Loading } from "~/components/ui/loading";
import { mutate } from "~/lib/swr";
import {
  fetchStaffPreviewCandidates,
  performStartStaffPreview,
  type StaffPreviewCandidate,
} from "~/lib/staff-preview-api";
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
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isOpen) return;
    setError("");
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
          setError(
            "Die Liste konnte nicht geladen werden. Bitte versuchen Sie es noch einmal.",
          );
        }
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  const handleStart = async () => {
    if (!selectedId || isStarting) return;
    setIsStarting(true);
    setError("");
    try {
      await performStartStaffPreview(selectedId, update, mutate);
      // Volle Neuladung auf der aktuellen Seite: ab jetzt rendert alles mit
      // den Rechten der gewählten Person.
      window.location.reload();
    } catch (err) {
      logger.error("staff_preview_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        "Die Vorschau konnte nicht gestartet werden. Bitte versuchen Sie es noch einmal.",
      );
      setIsStarting(false);
    }
  };

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

        {error && <Alert type="error" message={error} />}

        {candidates === null ? (
          <Loading fullPage={false} />
        ) : candidates.length === 0 && !error ? (
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
