"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { LogOut } from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { InfoCard } from "~/components/ui/info-card";
import { ISODatePicker } from "~/components/ui/date-picker";
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import {
  formatDate,
  parseISODate,
  toISODate,
  todayISO,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  CARE_EXIT_NOTE_MAX_LENGTH,
  CARE_EXIT_REASON_LABELS,
  confirmCareExit,
  confirmWithdrawalCareEnd,
  previewCareExit,
  previewWithdrawalCareEnd,
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
  /**
   * Der bereits eingetragene letzte Betreuungstag, wenn ein geplantes Ende
   * korrigiert wird (#2487). Ohne ihn stünde im Feld der heutige Tag, und wer
   * nur den Grund ändern will, verschiebt das Ende ungewollt nach vorne.
   */
  readonly plannedLastCareDay?: string;
  /** Durable follow-up created by an authoritative complete withdrawal. */
  readonly completionId?: string;
  readonly firstBookinglessDay?: string;
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
  plannedLastCareDay,
  completionId,
  firstBookinglessDay,
  onClose,
  onFinished,
}: CareExitModalProps) {
  // Ein bereits eingetragenes Ende, das in der Vergangenheit läge, wäre im Feld
  // nicht wählbar (min = heute) — dann bleibt der heutige Tag der Startwert.
  const completionLastCareDay = useMemo(() => {
    if (!completionId || !firstBookinglessDay) return null;
    const day = parseISODate(firstBookinglessDay);
    day.setDate(day.getDate() - 1);
    return toISODate(day);
  }, [completionId, firstBookinglessDay]);
  const initialLastCareDay =
    completionLastCareDay ??
    (plannedLastCareDay && plannedLastCareDay >= todayISO()
      ? plannedLastCareDay
      : todayISO());
  const isCorrection = Boolean(plannedLastCareDay);
  const [step, setStep] = useState<1 | 2>(1);
  const [lastCareDay, setLastCareDay] = useState(initialLastCareDay);
  const [reason, setReason] = useState<CareExitReason | "">(
    completionId ? "no_care_needed" : "",
  );
  const [note, setNote] = useState("");
  const [preview, setPreview] = useState<CareExitPreview | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const ids = useMemo(() => [...studentIds], [studentIds]);

  useEffect(() => {
    if (isOpen) return;
    setStep(1);
    setLastCareDay(initialLastCareDay);
    setReason(completionId ? "no_care_needed" : "");
    setNote("");
    setPreview(null);
    setError("");
  }, [completionId, isOpen, initialLastCareDay]);

  const noteRequired = reason === "other";
  const detailsComplete =
    Boolean(lastCareDay) &&
    Boolean(reason) &&
    (!noteRequired || note.trim().length > 0);

  // Lädt die Vorschau neu. Eine vorhandene Fehlermeldung wird NICHT gelöscht:
  // nach einer abgelehnten Bestätigung ist genau diese Meldung der Grund,
  // warum gerade neu geladen wird — sie muss stehen bleiben.
  const loadPreview = useCallback(async () => {
    if (!reason) return null;
    setLoading(true);
    try {
      const input = {
        studentIds: ids,
        lastCareDay,
        reason,
        reasonNote: note,
      };
      const result = completionId
        ? await previewWithdrawalCareEnd(completionId, input)
        : await previewCareExit(input);
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
  }, [completionId, ids, lastCareDay, note, reason]);

  const handleContinue = async () => {
    setError("");
    const result = await loadPreview();
    if (result) setStep(2);
  };

  const handleConfirm = async () => {
    if (!preview || !reason || preview.blocked) return;
    setSaving(true);
    setError("");
    try {
      const input = {
        studentIds: ids,
        lastCareDay,
        reason,
        reasonNote: note,
      };
      if (completionId) {
        await confirmWithdrawalCareEnd(completionId, preview.token, input);
      } else {
        await confirmCareExit(preview.token, input);
      }
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

  const title = isCorrection
    ? "Ende der Betreuung ändern"
    : ids.length === 1
      ? "Betreuung beenden"
      : `Betreuung von ${ids.length} Kindern beenden`;

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !saving) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[640px]">
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>{title}</SlideOverTitle>
            <SlideOverDescription>
              {step === 1
                ? "Schritt 1 von 2: Tag und Grund festlegen."
                : "Schritt 2 von 2: Vorschau prüfen und bestätigen."}
            </SlideOverDescription>
          </div>
          <SlideOverCloseButton disabled={saving} />
        </SlideOverHeader>
        <div className="flex-1 space-y-4 overflow-y-auto px-5 py-4">
          <WizardStepper steps={STEPS} current={step - 1} />

          <p className="text-sm text-gray-600">
            {completionId && !isCorrection
              ? completionLastCareDay && completionLastCareDay >= todayISO()
                ? `Der letzte Betreuungstag ist am ${formatDate(completionLastCareDay)}. Das Kind nimmt an diesem Tag noch teil.`
                : "Der letzte Betreuungstag liegt in der Vergangenheit. Mit diesem Schritt wird das frühere Ende dokumentiert."
              : isCorrection
                ? "Das Ende ist schon eingetragen. Tag und Grund werden neu gespeichert. Das Kind nimmt am letzten Betreuungstag noch teil."
                : "Das Kind nimmt am letzten Betreuungstag noch teil. Ab dem Folgetag ist seine Betreuung beendet. Die bisherigen Daten bleiben erhalten."}
          </p>

          {error ? <Alert type="error" message={error} /> : null}

          {step === 1 ? (
            <div className="space-y-4">
              <ISODatePicker
                id="care-exit-last-day"
                label="Letzter Betreuungstag"
                value={lastCareDay}
                onChange={setLastCareDay}
                min={completionId ? undefined : todayISO()}
                max={completionLastCareDay ?? undefined}
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
                  id="care-exit-note"
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
        <SlideOverFooter className="flex-row justify-end gap-2">
          {footer}
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
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
        <div className="moto-content-surface rounded-xl border shadow-sm">
          <div className="border-b border-gray-200 px-4 py-3">
            <p className="text-sm font-semibold text-gray-900">
              {ready.length === 1
                ? "Betreuung für dieses Kind beenden"
                : `Betreuung für ${ready.length} Kinder beenden`}
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
      {ready.length > 0 ? (
        <InfoCard
          title="Folgen des Austritts"
          icon={<LogOut className="h-5 w-5 text-gray-600" aria-hidden="true" />}
        >
          <ul className="list-disc space-y-1 pl-5 text-sm text-gray-600">
            <li>
              Die Anmeldung zur Betreuung endet. Das Kind erscheint danach nicht
              mehr in aktuellen Betreuungslisten.
            </li>
            <li>
              Alle laufenden und geplanten Angebote und Betreuungstermine enden.
            </li>
            <li>Die oben genannten Angebote und Wochentage enden.</li>
            <li>
              Ab dem Folgetag werden übrige offene Eltern-Anfragen geschlossen.
              Das Armband wird freigegeben. Eine noch offene Anwesenheit wird
              beendet.
            </li>
            <li>
              Vergangene Anwesenheit und andere historische Daten bleiben
              erhalten. Bei einer späteren Rückkehr muss die Betreuung neu
              geplant werden.
            </li>
          </ul>
        </InfoCard>
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
  if (impact.sourceOfferings.length > 0) {
    lines.push(
      `Gebuchte Angebote: ${impact.sourceOfferings
        .map(
          (offering) =>
            `${offering.name} (${formatCompactDays(offering.days)})`,
        )
        .join("; ")}`,
    );
  }
  if (impact.weeklyPlans?.length) {
    lines.push(`Wöchentliche Zeiten: ${impact.weeklyPlans.join("; ")}`);
  }
  if (impact.plannedEndsOn) {
    lines.push(
      `Bisher geplantes Ende: ${formatDate(impact.plannedEndsOn)}, wird geändert`,
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

const COMPACT_DAY_LABELS: Record<string, string> = {
  mon: "Mo",
  tue: "Di",
  wed: "Mi",
  thu: "Do",
  fri: "Fr",
  sat: "Sa",
  sun: "So",
};

function formatCompactDays(days: readonly string[]): string {
  if (days.length === 0) return "keine Betreuungstage";
  return days.map((day) => COMPACT_DAY_LABELS[day] ?? day).join(", ");
}
