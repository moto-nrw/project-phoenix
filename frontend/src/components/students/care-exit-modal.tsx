"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { LogOut } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { ISODatePicker } from "~/components/ui/date-picker";
import { Modal } from "~/components/ui/modal";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import { formatDate, todayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  CARE_EXIT_NOTE_MAX_LENGTH,
  CARE_EXIT_REASON_LABELS,
  confirmCareExit,
  previewCareExit,
  type CareExitImpact,
  type CareExitPreview,
  type CareExitReason,
} from "~/lib/care-exit-api";

const logger = createLogger({ component: "CareExitModal" });

const STEPS = ["Angaben", "Prüfen"] as const;

const REASON_OPTIONS: ReadonlyArray<{
  value: CareExitReason | "";
  label: string;
}> = [
  { value: "", label: "Bitte auswählen" },
  { value: "moved_away", label: CARE_EXIT_REASON_LABELS.moved_away },
  { value: "no_care_needed", label: CARE_EXIT_REASON_LABELS.no_care_needed },
  { value: "other", label: CARE_EXIT_REASON_LABELS.other },
];

interface CareExitModalProps {
  readonly isOpen: boolean;
  /** Die ausgewählten Kinder. Alle bekommen denselben Tag und denselben Grund. */
  readonly studentIds: readonly string[];
  readonly onClose: () => void;
  readonly onFinished: () => Promise<void> | void;
}

/**
 * "Betreuung beenden" für ein Kind oder eine Auswahl (#2487).
 *
 * Zwei Schritte: erst die Angaben, dann die Vorschau mit allen Kindern
 * namentlich. Bestätigt wird genau der Vorschau-Stand — hat sich ein Kind
 * seither verändert, schreibt moto nichts und nennt den Grund pro Kind.
 */
export function CareExitModal({
  isOpen,
  studentIds,
  onClose,
  onFinished,
}: CareExitModalProps) {
  const [step, setStep] = useState<1 | 2>(1);
  const [lastCareDay, setLastCareDay] = useState(todayISO());
  const [reason, setReason] = useState<CareExitReason | "">("");
  const [note, setNote] = useState("");
  const [preview, setPreview] = useState<CareExitPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const ids = useMemo(() => [...studentIds], [studentIds]);

  useEffect(() => {
    if (isOpen) return;
    setStep(1);
    setLastCareDay(todayISO());
    setReason("");
    setNote("");
    setPreview(null);
    setError("");
  }, [isOpen]);

  const noteRequired = reason === "other";
  const detailsComplete =
    Boolean(lastCareDay) &&
    Boolean(reason) &&
    (!noteRequired || note.trim().length > 0);

  const loadPreview = useCallback(async () => {
    if (!reason) return null;
    setLoading(true);
    setError("");
    try {
      const result = await previewCareExit({
        studentIds: ids,
        lastCareDay,
        reason,
        reasonNote: note,
      });
      setPreview(result);
      return result;
    } catch (previewError) {
      const message =
        previewError instanceof Error
          ? previewError.message
          : "Die Vorschau konnte nicht geladen werden. Bitte versuchen Sie es noch einmal.";
      logger.error("care_exit_preview_failed", {
        student_count: ids.length,
        error: message,
      });
      setError(message);
      setPreview(null);
      return null;
    } finally {
      setLoading(false);
    }
  }, [ids, lastCareDay, note, reason]);

  const handleContinue = async () => {
    const result = await loadPreview();
    if (result) setStep(2);
  };

  const handleConfirm = async () => {
    if (!preview || !reason || preview.blocked) return;
    setSaving(true);
    setError("");
    try {
      await confirmCareExit(preview.token, {
        studentIds: ids,
        lastCareDay,
        reason,
        reasonNote: note,
      });
      try {
        await onFinished();
      } catch (refreshError) {
        // Der Schreibvorgang war erfolgreich. Ein fehlgeschlagenes Neuladen
        // darf nicht wie ein fehlgeschlagener Austritt aussehen.
        logger.error("care_exit_success_callback_failed", {
          student_count: ids.length,
          error:
            refreshError instanceof Error
              ? refreshError.message
              : String(refreshError),
        });
        onClose();
      }
    } catch (confirmError) {
      const message =
        confirmError instanceof Error
          ? confirmError.message
          : "Die Betreuung wurde nicht beendet. Bitte versuchen Sie es noch einmal.";
      logger.error("care_exit_confirm_failed", {
        student_count: ids.length,
        error: message,
      });
      setError(message);
      // Der Vorschau-Stand ist verbraucht: neu laden, damit die Liste zeigt,
      // was sich geändert hat.
      await loadPreview();
    } finally {
      setSaving(false);
    }
  };

  const footer =
    step === 1 ? (
      <>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={onClose}
          disabled={loading}
        >
          Abbrechen
        </Button>
        <Button
          type="button"
          variant="primary"
          size="md"
          isLoading={loading}
          loadingText="Wird geprüft…"
          disabled={!detailsComplete || loading}
          onClick={() => void handleContinue()}
        >
          Weiter
        </Button>
      </>
    ) : (
      <>
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setStep(1)}
          disabled={saving}
        >
          Zurück
        </Button>
        <Button
          type="button"
          variant="primary"
          size="md"
          isLoading={saving}
          loadingText="Wird beendet…"
          disabled={saving || !preview || preview.blocked}
          onClick={() => void handleConfirm()}
        >
          <LogOut className="mr-2 h-4 w-4" aria-hidden="true" />
          Betreuung beenden
        </Button>
      </>
    );

  const title =
    ids.length === 1
      ? "Betreuung beenden"
      : `Betreuung von ${ids.length} Kindern beenden`;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={title}
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      footer={footer}
      isDismissDisabled={saving}
    >
      <div className="space-y-4">
        <WizardStepper steps={STEPS} current={step - 1} />

        <p className="text-sm text-gray-600">
          Das Kind nimmt am letzten Betreuungstag noch teil. Ab dem Folgetag ist
          seine Betreuung beendet. Die bisherigen Daten bleiben erhalten.
        </p>

        {error ? <Alert type="error" message={error} /> : null}

        {step === 1 ? (
          <div className="space-y-4">
            <ISODatePicker
              id="care-exit-last-day"
              label="Letzter Betreuungstag"
              value={lastCareDay}
              onChange={setLastCareDay}
              min={todayISO()}
              required
            />
            <div>
              <label
                id="care-exit-reason-label"
                htmlFor="care-exit-reason"
                className="mb-2 block text-sm font-medium text-gray-700"
              >
                Grund
              </label>
              <CustomSelect
                id="care-exit-reason"
                value={reason}
                options={REASON_OPTIONS}
                onChange={(value) => {
                  setReason(value as CareExitReason | "");
                  if (value !== "other") setNote("");
                }}
                ariaLabelledBy="care-exit-reason-label"
                required
              />
            </div>
            {noteRequired ? (
              <Input
                label="Kurze Erklärung"
                value={note}
                maxLength={CARE_EXIT_NOTE_MAX_LENGTH}
                placeholder="Zum Beispiel: Wechsel in eine andere Betreuung"
                onChange={(event) => setNote(event.target.value)}
              />
            ) : null}
            <p className="text-sm text-gray-500">
              {ids.length === 1
                ? "Ein Kind ist ausgewählt."
                : `${ids.length} Kinder sind ausgewählt. Alle bekommen denselben Tag und denselben Grund.`}
            </p>
          </div>
        ) : null}

        {step === 2 && preview ? (
          <CareExitPreviewList preview={preview} />
        ) : null}
      </div>
    </Modal>
  );
}

/** Die Vorschau: jedes Kind namentlich, mit dem, was sich für es ändert. */
function CareExitPreviewList({
  preview,
}: {
  readonly preview: CareExitPreview;
}) {
  const blocked = preview.students.filter((student) => student.blocker);
  const ready = preview.students.filter((student) => !student.blocker);

  return (
    <div className="space-y-4">
      {blocked.length > 0 ? (
        <div className="rounded-xl border border-gray-200 bg-gray-50 p-4">
          <p className="text-sm font-semibold text-gray-900">
            Die Betreuung wurde nicht beendet
          </p>
          <p className="mt-1 text-sm text-gray-600">
            Bei diesen Kindern geht es nicht. Bitte nehmen Sie sie aus der
            Auswahl.
          </p>
          <ul className="mt-3 space-y-2">
            {blocked.map((student) => (
              <li key={student.studentId} className="text-sm">
                <span className="font-medium text-gray-900">
                  {student.firstName} {student.lastName}
                </span>
                <span className="block text-gray-600">{student.blocker}</span>
              </li>
            ))}
          </ul>
        </div>
      ) : null}

      {ready.length > 0 ? (
        <div className="rounded-xl border border-gray-200 bg-white">
          <div className="border-b border-gray-200 px-4 py-3">
            <p className="text-sm font-semibold text-gray-900">
              {ready.length === 1
                ? "Dieses Kind wird beendet"
                : `Diese ${ready.length} Kinder werden beendet`}
            </p>
            <p className="mt-0.5 text-sm text-gray-600">
              Letzter Betreuungstag: {formatDate(preview.lastCareDay)}
            </p>
          </div>
          <ul className="divide-y divide-gray-100">
            {ready.map((student) => (
              <li key={student.studentId} className="px-4 py-3">
                <div className="flex flex-wrap items-baseline gap-x-2">
                  <span className="text-sm font-medium text-gray-900">
                    {student.firstName} {student.lastName}
                  </span>
                  {student.schoolClass ? (
                    <span className="text-sm text-gray-500">
                      {student.schoolClass}
                    </span>
                  ) : null}
                </div>
                <CareExitImpactLines impact={student} />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

/**
 * Was sich für dieses Kind ändert. Nur Zeilen, die wirklich zutreffen — eine
 * Liste aus lauter Nullen sagt nichts und lenkt von den echten Folgen ab.
 */
function CareExitImpactLines({ impact }: { readonly impact: CareExitImpact }) {
  const lines: string[] = [];
  if (impact.plannedEndsOn) {
    lines.push(
      `Bisher geplantes Ende: ${formatDate(impact.plannedEndsOn)} — wird geändert`,
    );
  }
  if (impact.plannedRosterRows > 0) {
    lines.push(
      impact.plannedRosterRows === 1
        ? "1 geplanter Termin danach entfällt"
        : `${impact.plannedRosterRows} geplante Termine danach entfallen`,
    );
  }
  if (impact.activityBookings > 0) {
    lines.push(
      impact.activityBookings === 1
        ? "1 Angebot endet am letzten Betreuungstag"
        : `${impact.activityBookings} Angebote enden am letzten Betreuungstag`,
    );
  }
  if (impact.openParentRequests > 0) {
    lines.push(
      impact.openParentRequests === 1
        ? "1 offene Eltern-Anfrage wird geschlossen"
        : `${impact.openParentRequests} offene Eltern-Anfragen werden geschlossen`,
    );
  }
  if (impact.hasRfidTag) {
    lines.push("Das Armband wird frei und kann neu vergeben werden");
  }
  if (impact.currentlyPresent) {
    lines.push("Das Kind ist gerade angemeldet und wird am Ende abgemeldet");
  }

  if (lines.length === 0) {
    return (
      <p className="mt-1 text-sm text-gray-500">
        Es sind keine weiteren Änderungen nötig.
      </p>
    );
  }

  return (
    <ul className="mt-1 space-y-0.5">
      {lines.map((line) => (
        <li key={line} className="text-sm text-gray-600">
          {line}
        </li>
      ))}
    </ul>
  );
}
