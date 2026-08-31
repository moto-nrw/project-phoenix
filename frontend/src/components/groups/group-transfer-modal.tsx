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
import {
  SlideOver,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GroupTransfer" });
const EMPTY_TRANSFERS: NonNullable<
  GroupTransferModalProps["existingTransfers"]
> = [];

function extractErrorMessage(err: unknown, fallback: string): string {
  if (
    !(err instanceof Error) ||
    !["TransferError", "CancelTransferError"].includes(err.name)
  ) {
    return fallback;
  }
  return err.message || fallback;
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
    readonly fullName: string;
  }>;
  readonly onTransfer: (
    targetStaffId: string,
    targetName: string,
  ) => Promise<void>;
  readonly existingTransfers?: ReadonlyArray<{
    readonly targetName: string;
    readonly substitutionId: string;
    readonly targetStaffId: string;
  }>;
  readonly loadError?: string | null;
  readonly onCancelTransfer?: (substitutionId: string) => Promise<void>;
}

export function GroupTransferModal({
  isOpen,
  onClose,
  group,
  availableUsers,
  onTransfer,
  existingTransfers = EMPTY_TRANSFERS,
  loadError = null,
  onCancelTransfer,
}: GroupTransferModalProps) {
  const [selectedStaffId, setSelectedStaffId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const displayedError = error ?? loadError;
  const errorRef = useScrollToError(displayedError);

  // Reset form when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setSelectedStaffId("");
      setError(null);
      setDeletingId(null);
    }
  }, [isOpen]);

  const handleTransfer = async () => {
    if (!selectedStaffId) {
      setError("Bitte wählen Sie eine pädagogische Fachkraft aus.");
      return;
    }

    const selectedUser = availableUsers.find(
      (user) => user.id === selectedStaffId,
    );
    const targetName = selectedUser?.fullName ?? "Pädagogische Fachkraft";

    try {
      setLoading(true);
      setError(null);
      await onTransfer(selectedStaffId, targetName);
      setSelectedStaffId("");
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
        disabled={!selectedStaffId || loading || availableUsers.length === 0}
      >
        Übergeben
      </Button>
    </>
  );

  return (
    <SlideOver
      open={isOpen}
      onOpenChange={(nextOpen) => {
        if (!nextOpen && !loading) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[560px]">
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>{`Gruppe "${group.name}" übergeben`}</SlideOverTitle>
            <SlideOverDescription>
              Die Verantwortung für diese Gruppe heute übergeben.
            </SlideOverDescription>
          </div>
          <SlideOverCloseButton disabled={loading} />
        </SlideOverHeader>
        <div className="flex-1 space-y-6 overflow-y-auto px-5 py-4">
          {displayedError ? (
            <div ref={errorRef}>
              <Alert type="error" message={displayedError} />
            </div>
          ) : null}

          <InfoSection
            title="Was die Übergabe bewirkt"
            icon={<Clock className="h-full w-full" />}
          >
            <p className="text-sm text-gray-600">
              Die ausgewählte pädagogische Fachkraft ist{" "}
              <strong className="font-medium text-gray-900">
                heute zusätzlich zuständig
              </strong>{" "}
              für diese Gruppe. Die Gruppe erscheint für diese Person unter
              „Meine Gruppen“.
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
              value={selectedStaffId}
              onChange={setSelectedStaffId}
              options={[
                { value: "", label: "Fachkraft auswählen..." },
                ...availableUsers.map((user) => ({
                  value: user.id,
                  label: user.fullName,
                })),
              ]}
              placeholder="Fachkraft auswählen..."
            />
            {availableUsers.length === 0 && !loadError && (
              <p className="mt-2 text-sm text-gray-500">
                Keine pädagogische Fachkraft verfügbar. Bitte wenden Sie sich an
                die Verwaltung.
              </p>
            )}
          </div>
        </div>
        <SlideOverFooter className="flex-row justify-end gap-2">
          {footer}
        </SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  );
}
