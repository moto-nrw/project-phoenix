"use client";

import { ShieldAlert } from "lucide-react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import NavigationLink from "~/components/ui/navigation-link";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  type AdminEnrollmentDeletionImpact,
  deleteAdminChild,
  deleteAdminRequest,
  getAdminChildDeleteImpact,
  getAdminRequestDeleteImpact,
} from "~/lib/enrollment-admin-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentDeletionModal" });

const COUNT_LABELS = {
  requests: "Anmeldungen",
  request_children: "Kinder in Anmeldungen",
  request_child_offerings: "Ausgewählte Betreuungsangebote",
  request_guardians: "Weitere Erziehungsberechtigte in der Anmeldung",
  change_requests: "Änderungsanfragen",
  change_request_messages: "Nachrichten zu Änderungsanfragen",
  late_invites: "Verwendete Nachzügler-Einladungen",
  offering_adjustments: "Protokolle zu Angebotsänderungen",
  email_outbox: "Abhängige E-Mail-Outbox-Einträge",
  rollover_links_cleared: "Verknüpfungen zu Folgeanmeldungen",
  student_source_links_cleared: "Herkunftsverknüpfungen in Aktivitäten",
} as const;

type CountKey = keyof typeof COUNT_LABELS;

interface Props {
  readonly isOpen: boolean;
  readonly requestId: string;
  readonly childId?: string;
  readonly childLabel?: string;
  readonly studentHref: (studentId: string) => string;
  readonly onClose: () => void;
  readonly onDeleted: (impact: AdminEnrollmentDeletionImpact) => void;
}

export function AdminEnrollmentDeletionModal({
  isOpen,
  requestId,
  childId,
  childLabel,
  studentHref,
  onClose,
  onDeleted,
}: Props) {
  const [impact, setImpact] = useState<AdminEnrollmentDeletionImpact | null>(
    null,
  );
  const [reason, setReason] = useState("");
  const [loadingPreview, setLoadingPreview] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [previewAttempt, setPreviewAttempt] = useState(0);
  const errors = useApiFormError();
  const clearErrors = errors.clear;
  const previewError = useApiLoadError();
  const showPreviewError = previewError.show;
  const clearPreviewError = previewError.clear;
  const retryPreview = useCallback(
    () => setPreviewAttempt((attempt) => attempt + 1),
    [],
  );
  // „Wiederholen“ löscht mit dem aktuellen Grund, nicht dem vom Fehler.
  const latestDeleteRef = useRef<() => Promise<void>>(async () => undefined);

  useEffect(() => {
    if (!isOpen) {
      setImpact(null);
      setReason("");
      clearErrors();
      clearPreviewError();
      return;
    }
    let active = true;
    setLoadingPreview(true);
    clearPreviewError();
    const preview = childId
      ? getAdminChildDeleteImpact(requestId, childId)
      : getAdminRequestDeleteImpact(requestId);
    void preview
      .then((result) => {
        if (active) setImpact(result);
      })
      .catch((loadError: unknown) => {
        logger.error("enrollment_delete_preview_failed", {
          error: loadError instanceof Error ? loadError.message : "unknown",
          request_id: requestId,
          child_id: childId,
        });
        if (active) {
          void showPreviewError(loadError, {
            object: "die Vorschau der Löschung",
            retry: retryPreview,
          });
        }
      })
      .finally(() => {
        if (active) setLoadingPreview(false);
      });
    return () => {
      active = false;
    };
  }, [
    childId,
    isOpen,
    requestId,
    previewAttempt,
    clearErrors,
    clearPreviewError,
    showPreviewError,
    retryPreview,
  ]);

  const countRows = useMemo(() => {
    if (!impact) return [];
    return (Object.keys(COUNT_LABELS) as CountKey[])
      .map((key) => ({
        key,
        label: COUNT_LABELS[key],
        value: impact.counts[key],
      }))
      .filter((row) => row.value > 0);
  }, [impact]);

  const trimmedReason = reason.trim();
  const reasonValid =
    [...trimmedReason].length >= 3 && [...trimmedReason].length <= 500;
  const confirmDisabled =
    loadingPreview || deleting || !impact?.can_delete || !reasonValid;

  const handleDelete = async () => {
    if (!impact || confirmDisabled) return;
    setDeleting(true);
    errors.clear();
    try {
      const result = childId
        ? await deleteAdminChild(requestId, childId, trimmedReason)
        : await deleteAdminRequest(requestId, trimmedReason);
      onDeleted(result);
    } catch (deleteError) {
      logger.error("enrollment_delete_failed", {
        error: deleteError instanceof Error ? deleteError.message : "unknown",
        request_id: requestId,
        child_id: childId,
      });
      await errors.show(deleteError, {
        object: "die Löschung",
        retry: () => void latestDeleteRef.current(),
      });
    } finally {
      setDeleting(false);
    }
  };
  useLayoutEffect(() => {
    latestDeleteRef.current = handleDelete;
  });

  const title = childId
    ? "Kind aus Anmeldung löschen"
    : "Gesamte Anmeldung löschen";
  const targetLabel = childId
    ? `${childLabel ?? `Kind ${childId}`} · Anmeldung #${requestId}`
    : `Anmeldung #${requestId}`;

  return (
    <ConfirmDeleteModal
      isOpen={isOpen}
      title={title}
      description={
        <>
          <span className="block font-semibold text-gray-900">
            {targetLabel}
          </span>
          <span className="mt-1 block">
            Diese Löschung lässt sich nicht rückgängig machen. Die Vorschau wird
            beim Löschen in derselben Transaktion erneut geprüft.
          </span>
        </>
      }
      warningSlot={
        <div className="space-y-4">
          {loadingPreview ? (
            <p className="text-sm text-gray-500">
              Auswirkungen werden geladen…
            </p>
          ) : null}

          <LoadErrorAlert error={previewError.error} />

          {impact ? (
            <>
              <section>
                <h4 className="text-sm font-semibold text-gray-900">
                  Betroffene Datensätze ({impact.counts.total})
                </h4>
                <div className="mt-2">
                  <DataGrid>
                    {countRows.map((row) => (
                      <DataField key={row.key} label={row.label}>
                        {row.value}
                      </DataField>
                    ))}
                  </DataGrid>
                </div>
              </section>

              {impact.deletes_request && childId ? (
                <Alert
                  type="info"
                  message="Dies ist das letzte Kind der Anmeldung. Deshalb wird auch der gemeinsame Anmeldungsdatensatz gelöscht."
                />
              ) : null}

              {impact.blocking_student_ids.length > 0 ? (
                <section className="border-moto-red/20 bg-moto-red/10 rounded-xl border p-4">
                  <div className="flex gap-3">
                    <ShieldAlert
                      className="text-moto-red mt-0.5 h-5 w-5 shrink-0"
                      aria-hidden="true"
                    />
                    <div>
                      <h4 className="text-sm font-semibold text-gray-900">
                        Löschung blockiert
                      </h4>
                      <p className="mt-1 text-sm text-gray-700">
                        Mindestens ein angelegtes Kind besteht noch. Löschen Sie
                        das Kind zuerst über die Kindverwaltung; die Anmeldung
                        löscht niemals still einen Kind-Datensatz.
                      </p>
                      <div className="mt-2 flex flex-wrap gap-2">
                        {impact.blocking_student_ids.map((studentId) => (
                          <NavigationLink
                            key={studentId}
                            href={studentHref(studentId)}
                            className="text-moto-red text-sm font-semibold underline underline-offset-2"
                          >
                            Kind #{studentId} öffnen
                          </NavigationLink>
                        ))}
                      </div>
                    </div>
                  </div>
                </section>
              ) : null}

              {impact.preserved_guardian_profiles > 0 ||
              impact.preserved_parent_accounts > 0 ? (
                <section className="rounded-xl border border-gray-200 bg-gray-50 p-4 text-sm text-gray-700">
                  <h4 className="font-semibold text-gray-900">
                    Erziehungsberechtigte bleiben erhalten
                  </h4>
                  <p className="mt-1">
                    {impact.preserved_guardian_profiles} Profile und{" "}
                    {impact.preserved_parent_accounts} Elternkonten werden nicht
                    gelöscht. Davon sind {impact.unlinked_guardian_profiles}{" "}
                    Profile und {impact.parent_accounts_without_students} Konten
                    aktuell keinem Kind zugeordnet. Nutzen Sie dafür bei Bedarf
                    den separaten Löschworkflow in der Kind- bzw.
                    Kontoverwaltung.
                  </p>
                </section>
              ) : null}

              {impact.can_delete ? (
                <>
                  <Input
                    name="reason"
                    label="Löschgrund (Pflichtfeld)"
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    placeholder="z. B. fehlerhafte Testanmeldung"
                    maxLength={500}
                    error={
                      reason.length > 0 && !reasonValid
                        ? "Bitte 3 bis 500 Zeichen eingeben."
                        : errors.fieldError("reason")
                    }
                    disabled={deleting}
                  />
                  <p className="text-xs text-gray-500">
                    Bitte keine Namen, Kontaktdaten oder Gesundheitsangaben in
                    den Löschgrund schreiben.
                  </p>
                </>
              ) : null}
            </>
          ) : null}
        </div>
      }
      gate={{ mode: "twoStep" }}
      confirmDisabled={confirmDisabled}
      onConfirm={handleDelete}
      onClose={onClose}
      loading={deleting}
      error={errors.error}
    />
  );
}
