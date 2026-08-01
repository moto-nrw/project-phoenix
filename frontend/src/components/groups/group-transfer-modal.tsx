"use client";

import { Clock } from "lucide-react";
import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GroupTransfer" });
const EMPTY_TRANSFERS: NonNullable<
  GroupTransferModalProps["existingTransfers"]
> = [];

/**
 * Holt aus einem Fehler die Meldung heraus, die der Nutzerin gezeigt werden
 * kann: die Klienten werfen `API error (409): {"error":"..."}`, interessant ist
 * nur der innere Text.
 */
function extractErrorMessage(err: unknown, fallback: string): string {
  if (!(err instanceof Error)) {
    return fallback;
  }

  const withoutPrefix = err.message.replace(/^API error \(\d+\):\s*/, "");

  try {
    const parsed = JSON.parse(withoutPrefix) as { error?: string };
    return parsed.error ?? withoutPrefix;
  } catch {
    return withoutPrefix;
  }
}

interface GroupTransferModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly group: {
    readonly id: string;
    readonly name: string;
    readonly studentCount?: number;
  } | null;
  readonly availableUsers: ReadonlyArray<{
    readonly id: string;
    readonly personId: string;
    readonly firstName: string;
    readonly lastName: string;
    readonly fullName: string;
    readonly email: string;
  }>;
  readonly onTransfer: (
    targetPersonId: string,
    targetName: string,
  ) => Promise<void>;
  readonly existingTransfers?: ReadonlyArray<{
    readonly targetName: string;
    readonly substitutionId: string;
    readonly targetStaffId: string;
  }>;
  readonly onCancelTransfer?: (substitutionId: string) => Promise<void>;
  readonly onRefreshTransfers?: () => Promise<void>;
}

export function GroupTransferModal({
  isOpen,
  onClose,
  group,
  availableUsers,
  onTransfer,
  existingTransfers = EMPTY_TRANSFERS,
  onCancelTransfer,
  onRefreshTransfers: _onRefreshTransfers,
}: GroupTransferModalProps) {
  const [selectedPersonId, setSelectedPersonId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const errorRef = useScrollToError(error);

  // Reset form when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setSelectedPersonId("");
      setError(null);
      setDeletingId(null);
    }
  }, [isOpen]);

  const handleTransfer = async () => {
    if (!selectedPersonId) {
      setError("Bitte wählen Sie einen Betreuer aus.");
      return;
    }

    const selectedUser = availableUsers.find(
      (user) => user.personId === selectedPersonId,
    );
    const targetName = selectedUser?.fullName ?? "Betreuer";

    try {
      setLoading(true);
      setError(null);
      await onTransfer(selectedPersonId, targetName);
      setSelectedPersonId("");
      setError(null);
    } catch (err) {
      logger.error("group_transfer_failed", {
        error: err instanceof Error ? err.message : String(err),
        group_id: group?.id,
      });
      setError(
        extractErrorMessage(
          err,
          "Fehler beim Übergeben der Gruppe. Bitte versuchen Sie es erneut.",
        ),
      );
    } finally {
      setLoading(false);
    }
  };

  const handleCancel = async (substitutionId: string) => {
    if (!onCancelTransfer) return;

    try {
      setDeletingId(substitutionId);
      setError(null);
      await onCancelTransfer(substitutionId);
      setError(null);
    } catch (err) {
      logger.error("group_transfer_cancel_failed", {
        error: err instanceof Error ? err.message : String(err),
        substitution_id: substitutionId,
      });
      setError(
        extractErrorMessage(
          err,
          "Fehler beim Zurücknehmen. Bitte versuchen Sie es erneut.",
        ),
      );
    } finally {
      setDeletingId(null);
    }
  };

  if (!group) return null;

  const footer = (
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
        onClick={() => void handleTransfer()}
        isLoading={loading}
        loadingText="Wird übergeben..."
        disabled={!selectedPersonId || loading || availableUsers.length === 0}
      >
        Übergeben
      </Button>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Gruppe "${group.name}" übergeben`}
      footer={footer}
      isDismissDisabled={loading}
    >
      <div className="space-y-6">
        {error ? (
          <div ref={errorRef}>
            <Alert type="error" message={error} />
          </div>
        ) : null}

        <InfoSection
          title="Was die Übergabe bewirkt"
          icon={<Clock className="h-full w-full" />}
        >
          <p className="text-sm text-gray-600">
            Der ausgewählte Betreuer erhält{" "}
            <strong className="font-medium text-gray-900">
              zusätzliche Berechtigungen
            </strong>{" "}
            für diese Gruppe bis{" "}
            <strong className="font-medium text-gray-900">
              heute 23:59 Uhr
            </strong>
            . Du behältst weiterhin vollen Zugriff auf die Gruppe.
          </p>
        </InfoSection>

        <DataGrid>
          <DataField label="Gruppe">{group.name}</DataField>
          {group.studentCount !== undefined && (
            <DataField label="Gruppengröße">
              {group.studentCount} Kinder insgesamt
            </DataField>
          )}
        </DataGrid>

        {existingTransfers.length > 0 && (
          <section className="space-y-2">
            <p className="text-sm font-medium text-gray-700">
              Aktuell übergeben an:
            </p>
            <ul className="divide-y divide-gray-200 overflow-hidden rounded-2xl border border-gray-200 bg-white">
              {existingTransfers.map((transfer) => (
                <li
                  key={transfer.substitutionId}
                  className="flex items-center justify-between gap-3 p-3"
                >
                  <span className="min-w-0 truncate text-sm font-medium text-gray-900">
                    {transfer.targetName}
                  </span>
                  <Button
                    type="button"
                    variant="outline_danger"
                    size="compact"
                    onClick={() => void handleCancel(transfer.substitutionId)}
                    isLoading={deletingId === transfer.substitutionId}
                    loadingText="Wird entfernt..."
                    disabled={deletingId === transfer.substitutionId}
                  >
                    Entfernen
                  </Button>
                </li>
              ))}
            </ul>
          </section>
        )}

        <div>
          <label
            id="transfer-user-select-label"
            htmlFor="transfer-user-select"
            className="mb-2 block text-sm font-medium text-gray-700"
          >
            Übergeben an:
          </label>
          <CustomSelect
            id="transfer-user-select"
            ariaLabelledBy="transfer-user-select-label"
            value={selectedPersonId}
            onChange={setSelectedPersonId}
            options={[
              { value: "", label: "Betreuer auswählen..." },
              ...availableUsers.map((user) => ({
                value: user.personId,
                label: user.fullName,
              })),
            ]}
            placeholder="Betreuer auswählen..."
          />
          {availableUsers.length === 0 && (
            <p className="mt-2 text-sm text-gray-500">
              Keine Betreuer verfügbar. Bitte stellen Sie sicher, dass
              Lehrkräfte im System angelegt sind.
            </p>
          )}
        </div>
      </div>
    </Modal>
  );
}
