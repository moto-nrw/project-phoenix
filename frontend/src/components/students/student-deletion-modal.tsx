"use client";

import { Database } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  DataField,
  DataGrid,
  InfoSection,
  InfoText,
} from "~/components/ui/detail-modal-components";
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

// Zusätzlicher Grund für Kinder, deren Betreuung beendet ist (#2487). Er
// steht nur dort zur Wahl: bei einem Kind, das noch betreut wird, läuft keine
// Aufbewahrungsfrist, und der Grund wäre eine falsche Angabe im Protokoll.
const RETENTION_REASON_OPTION = {
  value: "retention_expired" as StudentDeletionReason,
  label: "Aufbewahrungsfrist abgelaufen",
};

interface StudentDeletionModalProps {
  readonly isOpen: boolean;
  readonly studentId: string;
  readonly displayName: string;
  readonly completionId?: string;
  /** Die Betreuung dieses Kindes ist beendet — nur dann gibt es den Grund
   *  "Aufbewahrungsfrist abgelaufen" (#2487). */
  readonly careEnded?: boolean;
  /** Aus einer offenen Abmeldung heraus: das Kind wird sofort gelöscht, ein
   *  späterer letzter Betreuungstag wird nicht abgewartet (#2434). */
  readonly skipsLastCareDay?: boolean;
  readonly onClose: () => void;
  readonly onDeleted: () => Promise<void> | void;
}

// Endgültiges Löschen eines Kindes (#3110): ein ConfirmDeleteModal mit der
// Texteingabe-Stufe. Vorschau der Folgen, Löschgrund und Bestätigungshaken
// liegen im Dialog; die Namenseingabe ist das Gate des Bauteils.
export function StudentDeletionModal({
  isOpen,
  studentId,
  displayName,
  completionId,
  careEnded = false,
  skipsLastCareDay = false,
  onClose,
  onDeleted,
}: StudentDeletionModalProps) {
  const [impact, setImpact] = useState<StudentDeletionImpact | null>(null);
  const [reason, setReason] = useState<StudentDeletionReason | "">("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [error, setError] = useState("");
  // Bumped after a 409 so the dialog remounts and the typed name is cleared.
  const [gateReset, setGateReset] = useState(0);

  useEffect(() => {
    if (!isOpen) {
      setImpact(null);
      setReason("");
      setAcknowledged(false);
      setError("");
      return;
    }

    let active = true;
    setLoadingPreview(true);
    setError("");
    void fetchStudentDeletionImpact(studentId, completionId)
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
  }, [completionId, isOpen, studentId]);

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

  const reasonOptions = useMemo(
    () =>
      careEnded ? [...REASON_OPTIONS, RETENTION_REASON_OPTION] : REASON_OPTIONS,
    [careEnded],
  );

  const prerequisitesMet = Boolean(impact && reason && acknowledged);

  const handleDelete = async () => {
    if (!impact || !reason || !acknowledged) return;
    setDeleting(true);
    setError("");
    try {
      const input = {
        expected_fingerprint: impact.fingerprint,
        confirmation_name: impact.confirmation_name,
        reason,
        acknowledged: true as const,
      };
      await deleteStudentWithData(studentId, input, completionId);
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
        deleteError.code === "students.deletion_preview_changed"
      ) {
        // The backend re-checks the preview under a row lock. A 409 means the
        // user must see a fresh impact before confirming again: the
        // acknowledgement resets, and the dialog remounts (`key` below),
        // which clears the typed name.
        setAcknowledged(false);
        setGateReset((count) => count + 1);
        setLoadingPreview(true);
        try {
          setImpact(await fetchStudentDeletionImpact(studentId, completionId));
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

  return (
    <ConfirmDeleteModal
      key={gateReset}
      isOpen={isOpen}
      title={`${displayName} löschen`}
      description={
        <Alert
          type="warning"
          message={`${displayName} wird dauerhaft entfernt. Die unten aufgeführten Daten werden je nach Art gelöscht oder vom Kind gelöst.${
            skipsLastCareDay
              ? " Das Kind wird sofort gelöscht. Auch ein späterer letzter Betreuungstag wird nicht abgewartet."
              : ""
          } Dieser Schritt kann nicht rückgängig gemacht werden.`}
        />
      }
      warningSlot={
        <div className="space-y-4">
          {loadingPreview ? (
            <p className="text-sm text-gray-500">
              Auswirkungen werden geladen…
            </p>
          ) : null}

          {impact ? (
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
                  <InfoText>
                    Keine weiteren verknüpften Daten gefunden.
                  </InfoText>
                )}
              </InfoSection>

              <InfoSection
                title="Was bleibt erhalten?"
                icon={<MotoConceptIcon concept="permissions" size="100%" />}
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
                  options={reasonOptions}
                  onChange={(value) =>
                    setReason(value as StudentDeletionReason | "")
                  }
                  ariaLabelledBy="student-deletion-reason-label"
                  disabled={deleting}
                  required
                />
              </div>

              <ChoiceTile
                htmlFor="student-deletion-acknowledgement"
                disabled={deleting}
                className="border-moto-red/20 hover:border-moto-red/20 items-start p-4 font-normal"
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
              </ChoiceTile>
            </>
          ) : null}
        </div>
      }
      gate={{
        mode: "textConfirm",
        expected: impact?.confirmation_name ?? "",
        inputId: "student-deletion-confirmation-name",
        label: "Zur Sicherheit: Name des Kindes erneut eingeben",
        placeholder: impact?.confirmation_name,
        preview: impact?.confirmation_name,
      }}
      confirmDisabled={loadingPreview || !prerequisitesMet}
      confirmLabel="Kind endgültig löschen"
      onConfirm={handleDelete}
      onClose={onClose}
      loading={deleting}
      error={error}
    />
  );
}
