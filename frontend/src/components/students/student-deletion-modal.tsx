"use client";

import { Database, ShieldCheck, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import {
  DataField,
  DataGrid,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { WizardStepper } from "~/components/ui/wizard-stepper";
import {
  deleteStudentWithData,
  fetchStudentDeletionImpact,
  StudentDeletionApiError,
  type StudentDeletionCounts,
  type StudentDeletionImpact,
  type StudentDeletionReason,
} from "~/lib/student-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentDeletionModal" });

const COUNT_LABELS: Record<keyof StudentDeletionCounts, string> = {
  timetable_assignments: "Stundenplan-Zuordnungen",
  activity_enrollments: "Angebotsanmeldungen",
  attendance_records: "Anwesenheits- und Statusdaten",
  care_schedules: "Ankunfts- und Abholpläne",
  guardian_links: "Verknüpfungen zu Erziehungsberechtigten",
  companion_links: "Laufgemeinschaften",
  communications: "Nachrichten und Änderungsanfragen",
  consents: "Einwilligungen",
  enrollment_references: "Von Anmeldungen gelöste Verknüpfungen",
  other_records: "Weitere kindbezogene Datensätze",
};

const REASON_OPTIONS: ReadonlyArray<{
  value: StudentDeletionReason | "";
  label: string;
}> = [
  { value: "", label: "Bitte auswählen" },
  { value: "test_data", label: "Testdaten" },
  { value: "incorrect_entry", label: "Fehlerhafte Erfassung" },
  { value: "duplicate", label: "Doppelter Datensatz" },
  { value: "privacy_request", label: "Datenschutz-/Löschanfrage" },
];

const DELETION_STEPS = ["Daten prüfen", "Name bestätigen"] as const;

interface StudentDeletionModalProps {
  readonly isOpen: boolean;
  readonly studentId: string;
  readonly displayName: string;
  readonly onClose: () => void;
  readonly onDeleted: () => Promise<void> | void;
}

export function StudentDeletionModal({
  isOpen,
  studentId,
  displayName,
  onClose,
  onDeleted,
}: StudentDeletionModalProps) {
  const [impact, setImpact] = useState<StudentDeletionImpact | null>(null);
  const [step, setStep] = useState<1 | 2>(1);
  const [reason, setReason] = useState<StudentDeletionReason | "">("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [confirmationName, setConfirmationName] = useState("");
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isOpen) {
      setImpact(null);
      setStep(1);
      setReason("");
      setAcknowledged(false);
      setConfirmationName("");
      setError("");
      return;
    }

    let active = true;
    setLoadingPreview(true);
    setError("");
    void fetchStudentDeletionImpact(studentId)
      .then((result) => {
        if (active) setImpact(result);
      })
      .catch((previewError: unknown) => {
        const message =
          previewError instanceof Error
            ? previewError.message
            : "Auswirkungen der Löschung konnten nicht geladen werden.";
        logger.error("student_delete_preview_failed", {
          student_id: studentId,
          error: message,
        });
        if (active) setError(message);
      })
      .finally(() => {
        if (active) setLoadingPreview(false);
      });

    return () => {
      active = false;
    };
  }, [isOpen, studentId]);

  const countRows = useMemo(() => {
    if (!impact) return [];
    return (Object.keys(COUNT_LABELS) as Array<keyof StudentDeletionCounts>)
      .map((key) => ({
        key,
        label: COUNT_LABELS[key],
        value: impact.counts[key],
      }))
      .filter((row) => row.value > 0);
  }, [impact]);

  const firstStepComplete = Boolean(impact && reason && acknowledged);
  const nameMatches = confirmationName === impact?.confirmation_name;

  const handleDelete = async () => {
    if (!impact || !reason || !acknowledged || !nameMatches) return;
    setDeleting(true);
    setError("");
    try {
      await deleteStudentWithData(studentId, {
        expected_fingerprint: impact.fingerprint,
        confirmation_name: confirmationName,
        reason,
        acknowledged: true,
      });
      try {
        await onDeleted();
      } catch (refreshError) {
        // The destructive request already succeeded. A failed list refresh
        // must not be presented as a failed deletion or leave a retry button
        // that can only answer 404 on the second attempt.
        logger.error("student_delete_success_callback_failed", {
          student_id: studentId,
          error:
            refreshError instanceof Error
              ? refreshError.message
              : String(refreshError),
        });
        onClose();
      }
    } catch (deleteError) {
      const message =
        deleteError instanceof Error
          ? deleteError.message
          : "Das Kind konnte nicht gelöscht werden.";
      logger.error("student_delete_failed", {
        student_id: studentId,
        status:
          deleteError instanceof StudentDeletionApiError
            ? deleteError.status
            : undefined,
        error: message,
      });

      if (
        deleteError instanceof StudentDeletionApiError &&
        deleteError.status === 409
      ) {
        // The backend re-checks the preview under a row lock. A 409 means the
        // user must see a fresh impact before confirming again.
        setStep(1);
        setAcknowledged(false);
        setConfirmationName("");
        setLoadingPreview(true);
        try {
          setImpact(await fetchStudentDeletionImpact(studentId));
        } catch (refreshError) {
          logger.error("student_delete_preview_refresh_failed", {
            student_id: studentId,
            error:
              refreshError instanceof Error
                ? refreshError.message
                : String(refreshError),
          });
          setImpact(null);
        } finally {
          setLoadingPreview(false);
        }
      }
      setError(message);
    } finally {
      setDeleting(false);
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
          disabled={deleting}
        >
          Abbrechen
        </Button>
        <Button
          type="button"
          variant="danger"
          size="md"
          className="text-white"
          disabled={!firstStepComplete || loadingPreview || deleting}
          onClick={() => {
            setError("");
            setStep(2);
          }}
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
          disabled={deleting}
        >
          Zurück
        </Button>
        <Button
          type="button"
          variant="danger"
          size="md"
          className="text-white"
          isLoading={deleting}
          loadingText="Wird gelöscht…"
          disabled={!nameMatches || deleting}
          onClick={() => void handleDelete()}
        >
          <Trash2 className="mr-2 h-4 w-4" aria-hidden="true" />
          Kind endgültig löschen
        </Button>
      </>
    );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={
        step === 1
          ? `${displayName} löschen`
          : `Löschung von ${displayName} bestätigen`
      }
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      footer={footer}
      isDismissDisabled={deleting}
    >
      <div className="space-y-4">
        <WizardStepper steps={DELETION_STEPS} current={step - 1} />

        <Alert
          type="warning"
          message={`${displayName} wird dauerhaft entfernt. Die unten aufgeführten Daten werden je nach Art gelöscht oder vom Kind gelöst. Dieser Schritt kann nicht rückgängig gemacht werden.`}
        />

        {loadingPreview ? (
          <p className="text-sm text-gray-500">Auswirkungen werden geladen…</p>
        ) : null}
        {error ? <Alert type="error" message={error} /> : null}

        {impact && step === 1 ? (
          <>
            <InfoSection
              title={`Was wird gelöscht oder gelöst? (${impact.total})`}
              icon={<Database className="h-full w-full" strokeWidth={2} />}
              accentColor="red"
            >
              {countRows.length > 0 ? (
                <DataGrid>
                  {countRows.map((row) => (
                    <DataField key={row.key} label={row.label}>
                      {row.value}
                    </DataField>
                  ))}
                </DataGrid>
              ) : (
                <InfoText>Keine weiteren verknüpften Daten gefunden.</InfoText>
              )}
            </InfoSection>

            <InfoSection
              title="Was bleibt erhalten?"
              icon={<ShieldCheck className="h-full w-full" strokeWidth={2} />}
              accentColor="gray"
            >
              <InfoText>
                Elternkonten und Profile der Erziehungsberechtigten, andere
                Kinder sowie gemeinsam genutzte Stundenplan-Termine bleiben
                bestehen. Anmeldungen und Zugriffsprotokolle bleiben erhalten
                und verlieren nur die Kindzuordnung.
              </InfoText>
            </InfoSection>

            <div>
              <label
                id="student-deletion-reason-label"
                htmlFor="student-deletion-reason"
                className="mb-2 block text-sm font-medium text-gray-700"
              >
                Löschgrund
              </label>
              <CustomSelect
                id="student-deletion-reason"
                value={reason}
                options={REASON_OPTIONS}
                onChange={(value) =>
                  setReason(value as StudentDeletionReason | "")
                }
                ariaLabelledBy="student-deletion-reason-label"
                disabled={deleting}
                required
              />
            </div>

            <label
              htmlFor="student-deletion-acknowledgement"
              className="flex cursor-pointer items-start gap-3 rounded-xl border border-[#FF3130]/20 bg-white p-4 text-sm text-gray-700"
            >
              <Checkbox
                id="student-deletion-acknowledgement"
                checked={acknowledged}
                onChange={(event) => setAcknowledged(event.target.checked)}
                disabled={deleting}
              />
              <span>
                Ich habe geprüft, welche Daten entfernt werden, und möchte das
                Kind dauerhaft löschen.
              </span>
            </label>
          </>
        ) : null}

        {impact && step === 2 ? (
          <section className="space-y-3">
            <p className="text-sm text-gray-700">
              Zur Sicherheit: Tippe den Namen genau so ein, wie er hier steht.
            </p>
            <InfoSection
              title="Name des Kindes"
              icon={<ShieldCheck className="h-full w-full" strokeWidth={2} />}
              accentColor="gray"
            >
              <InfoText>{impact.confirmation_name}</InfoText>
            </InfoSection>
            <Input
              name="student-deletion-confirmation-name"
              label="Name erneut eingeben"
              value={confirmationName}
              onChange={(event) => setConfirmationName(event.target.value)}
              placeholder={impact.confirmation_name}
              autoComplete="off"
              disabled={deleting}
              error={
                confirmationName.length > 0 && !nameMatches
                  ? "Der Name stimmt noch nicht exakt überein."
                  : undefined
              }
            />
          </section>
        ) : null}
      </div>
    </Modal>
  );
}
