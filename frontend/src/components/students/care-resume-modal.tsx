"use client";

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Modal } from "~/components/ui/modal";
import { todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { resumeCare } from "~/lib/care-exit-api";

const logger = createLogger({ component: "CareResumeModal" });

/**
 * Was die Leitung nach einer Wiederaufnahme selbst prüfen muss. moto schaltet
 * nichts davon automatisch wieder ein — die alten Angaben sind Monate alt und
 * eine stille Reaktivierung wäre für niemanden nachvollziehbar.
 */
const CHECK_ITEMS = [
  "Gruppe",
  "Angebote",
  "Wochenplan",
  "Ankunfts- und Gehzeiten",
] as const;

interface CareResumeModalProps {
  readonly isOpen: boolean;
  readonly studentId: string;
  readonly displayName: string;
  readonly onClose: () => void;
  readonly onResumed: () => Promise<void> | void;
}

/** "Betreuung wieder aufnehmen" für ein Kind (#2487). */
export function CareResumeModal({
  isOpen,
  studentId,
  displayName,
  onClose,
  onResumed,
}: CareResumeModalProps) {
  const [newStart, setNewStart] = useState(todayISO());
  const [checked, setChecked] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (isOpen) return;
    setNewStart(todayISO());
    setChecked(false);
    setError("");
  }, [isOpen]);

  const handleResume = async () => {
    if (!newStart || !checked) return;
    setSaving(true);
    setError("");
    try {
      await resumeCare(studentId, newStart, true);
      try {
        await onResumed();
      } catch (refreshError) {
        logger.error("care_resume_success_callback_failed", {
          student_id: studentId,
          error:
            refreshError instanceof Error
              ? refreshError.message
              : String(refreshError),
        });
        onClose();
      }
    } catch (resumeError) {
      const message =
        resumeError instanceof Error
          ? resumeError.message
          : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
      logger.error("care_resume_failed", {
        student_id: studentId,
        error: message,
      });
      setError(message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Betreuung von ${displayName} wieder aufnehmen`}
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-lg"
      isDismissDisabled={saving}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            isLoading={saving}
            loadingText="Wird gespeichert…"
            disabled={saving || !newStart || !checked}
            onClick={() => void handleResume()}
          >
            Betreuung wieder aufnehmen
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          Die Stammdaten des Kindes sind noch da. Gruppe, Angebote, Wochenplan
          und Zeiten schaltet moto nicht von selbst wieder ein.
        </p>

        {error ? <Alert type="error" message={error} /> : null}

        <ISODatePicker
          id="care-resume-start"
          label="Neuer Beginn"
          value={newStart}
          onChange={setNewStart}
          min={todayISO()}
          required
        />

        <div className="rounded-xl border border-gray-200 bg-gray-50 p-4">
          <p className="text-sm font-medium text-gray-900">
            Bitte danach selbst prüfen
          </p>
          <ul className="mt-2 space-y-1">
            {CHECK_ITEMS.map((item) => (
              <li key={item} className="text-sm text-gray-600">
                {item}
              </li>
            ))}
          </ul>
        </div>

        <ChoiceTile
          htmlFor="care-resume-checked"
          disabled={saving}
          className="items-start p-4 font-normal"
        >
          <Checkbox
            id="care-resume-checked"
            checked={checked}
            onChange={(event) => setChecked(event.target.checked)}
            disabled={saving}
          />
          <span>
            Ich habe Gruppe, Angebote, Wochenplan sowie Ankunfts- und Gehzeiten
            geprüft.
          </span>
        </ChoiceTile>
      </div>
    </Modal>
  );
}
