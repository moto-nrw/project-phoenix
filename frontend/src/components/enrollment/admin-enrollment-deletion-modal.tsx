"use client";

import Link from "~/components/ui/navigation-link";
import { ShieldAlert } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { Input } from "~/components/ui/input";
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
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isOpen) {
      setImpact(null);
      setReason("");
      setError("");
      return;
    }
    let active = true;
    setLoadingPreview(true);
    setError("");
    const preview = childId
      ? getAdminChildDeleteImpact(requestId, childId)
      : getAdminRequestDeleteImpact(requestId);
    void preview
      .then((result) => {
        if (active) setImpact(result);
      })
      .catch((previewError: unknown) => {
        const message =
          previewError instanceof Error
            ? previewError.message
            : "Unbekannter Fehler";
        logger.error("enrollment_delete_preview_failed", {
          error: message,
          request_id: requestId,
          child_id: childId,
        });
        if (active) setError(message);
      })
      .finally(() => {
        if (active) setLoadingPreview(false);
      });
    return () => {
      active = false;
    };
  }, [childId, isOpen, requestId]);

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
    setError("");
    try {
      const result = childId
        ? await deleteAdminChild(requestId, childId, trimmedReason)
        : await deleteAdminRequest(requestId, trimmedReason);
      onDeleted(result);
    } catch (deleteError) {
      const message =
        deleteError instanceof Error
          ? deleteError.message
          : "Unbekannter Fehler";
      logger.error("enrollment_delete_failed", {
        error: message,
        request_id: requestId,
        child_id: childId,
      });
      setError(message);
    } finally {
      setDeleting(false);
    }
  };

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
                          <Link
                            key={studentId}
                            href={studentHref(studentId)}
                            className="text-moto-red text-sm font-semibold underline underline-offset-2"
                          >
                            Kind #{studentId} öffnen
                          </Link>
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
                    name="enrollment-deletion-reason"
                    label="Löschgrund (Pflichtfeld)"
                    value={reason}
                    onChange={(event) => setReason(event.target.value)}
                    placeholder="z. B. fehlerhafte Testanmeldung"
                    maxLength={500}
                    error={
                      reason.length > 0 && !reasonValid
                        ? "Bitte 3 bis 500 Zeichen eingeben."
                        : undefined
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
      error={error}
    />
  );
}
