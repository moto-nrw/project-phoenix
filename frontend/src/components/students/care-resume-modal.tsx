"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Modal } from "~/components/ui/modal";
import { useApiFormError } from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { resumeCare } from "~/lib/care-exit-api";
import { InsetPanel } from "~/components/ui/inset-panel";

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
  /** Closes the dialog and reloads; never rejects (#6652). */
  readonly onResumed: RefreshAfterWrite;
}

/** "Betreuung wieder aufnehmen" für ein Kind (#2487). */
export function CareResumeModal({
  isOpen,
  studentId,
  displayName,
  onClose,
  onResumed,
}: CareResumeModalProps) {
  const [newStart, setNewStart] = useState(berlinTodayISO());
  const [checked, setChecked] = useState(false);
  const [saving, setSaving] = useState(false);
  const errors = useApiFormError();
  const clearErrors = errors.clear;
  // „Wiederholen“ sendet den aktuellen Stand, nicht den vom Fehler.
  const latestResumeRef = useRef<() => Promise<void>>(async () => undefined);

  useEffect(() => {
    if (isOpen) return;
    setNewStart(berlinTodayISO());
    setChecked(false);
    clearErrors();
  }, [isOpen, clearErrors]);

  const handleResume = async () => {
    if (!newStart || !checked) return;
    setSaving(true);
    errors.clear();
    try {
      await resumeCare(studentId, newStart, true);
    } catch (resumeError) {
      logger.error("care_resume_failed", {
        student_id: studentId,
        error:
          resumeError instanceof Error
            ? resumeError.message
            : String(resumeError),
      });
      await errors.show(resumeError, {
        object: "die Wiederaufnahme",
        retry: () => void latestResumeRef.current(),
      });
      setSaving(false);
      return;
    }
    setSaving(false);
    // Die Wiederaufnahme ist gespeichert. Ein gescheitertes Nachladen zeigt
    // der Aufrufer als Ladefehler, nie als gescheiterte Wiederaufnahme
    // (#6652).
    await onResumed();
  };
  useLayoutEffect(() => {
    latestResumeRef.current = handleResume;
  });

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
            noTooltip="Schließt nur das Fenster, die Betreuung bleibt beendet."
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
            tooltip="Trägt den neuen Beginn ein, Gruppe und Angebote stellen Sie selbst ein."
            disabled={saving || !newStart || !checked}
            disabledReason={
              !newStart
                ? "Wählen Sie zuerst den neuen Beginn."
                : "Bestätigen Sie zuerst, dass Sie alles geprüft haben."
            }
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

        <FormErrorAlert message={errors.error} />

        <ISODatePicker
          id="care-resume-start"
          label="Neuer Beginn"
          value={newStart}
          onChange={setNewStart}
          min={berlinTodayISO()}
          required
        />

        <InsetPanel framed padding="md">
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
        </InsetPanel>

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
