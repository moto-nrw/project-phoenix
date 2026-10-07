"use client";

import { Clock } from "lucide-react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import type { FormErrorInput } from "~/components/ui/form-error";
import { ConfirmationModal } from "~/components/ui/modal";
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
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "GroupTransfer" });
const EMPTY_TRANSFERS: NonNullable<
  GroupTransferModalProps["existingTransfers"]
> = [];

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
  readonly loadError?: FormErrorInput;
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
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearFormErrors } = formErrors;
  const [deletingId, setDeletingId] = useState<string | null>(null);
  // Zurücknehmen läuft erst nach der Rückfrage (Bauart 2 Regel 6, #3109).
  const [cancelTarget, setCancelTarget] = useState<{
    readonly substitutionId: string;
    readonly targetName: string;
  } | null>(null);
  // „Wiederholen“ übergibt an die aktuell gewählte Fachkraft.
  const latestTransferRef = useRef<() => void>(() => undefined);

  // Reset form when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setSelectedStaffId("");
      clearFormErrors();
      setDeletingId(null);
      setCancelTarget(null);
    }
  }, [isOpen, clearFormErrors]);

  const handleTransfer = async () => {
    formErrors.clear();
    if (!selectedStaffId) {
      formErrors.invalid("Bitte wählen Sie eine pädagogische Fachkraft aus.");
      return;
    }

    const selectedUser = availableUsers.find(
      (user) => user.id === selectedStaffId,
    );
    const targetName = selectedUser?.fullName ?? "Pädagogische Fachkraft";

    try {
      setLoading(true);
      await onTransfer(selectedStaffId, targetName);
      setSelectedStaffId("");
    } catch (err) {
      logger.error("group_transfer_failed", {
        error: err instanceof Error ? err.message : String(err),
        group_id: group?.id,
      });
      void formErrors.show(err, {
        object: "die Übergabe",
        retry: () => latestTransferRef.current(),
      });
    } finally {
      setLoading(false);
    }
  };

  const handleCancel = async (substitutionId: string) => {
    if (!onCancelTransfer) return;

    try {
      setDeletingId(substitutionId);
      formErrors.clear();
      await onCancelTransfer(substitutionId);
    } catch (err) {
      logger.error("group_transfer_cancel_failed", {
        error: err instanceof Error ? err.message : String(err),
        substitution_id: substitutionId,
      });
      // Kein Wiederholen-Link: der Dialog ist dann schon zu, Zurücknehmen
      // startet man erneut über die Liste.
      void formErrors.show(err, { object: "die Übergabe" });
    } finally {
      setDeletingId(null);
    }
  };

  useLayoutEffect(() => {
    latestTransferRef.current = () => void handleTransfer();
  });

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

  if (cancelTarget) {
    return (
      <ConfirmationModal
        isOpen
        title="Übergabe zurücknehmen?"
        confirmText="Übergabe zurücknehmen"
        cancelText="Abbrechen"
        isConfirmLoading={deletingId !== null}
        isDismissDisabled={deletingId !== null}
        onConfirm={async () => {
          await handleCancel(cancelTarget.substitutionId);
          // Ein Fehler steht im Alert des Formulars; der Dialog schließt in
          // beiden Fällen.
          setCancelTarget(null);
        }}
        onClose={() => setCancelTarget(null)}
      >
        <p className="text-sm text-gray-700">
          <strong>{cancelTarget.targetName}</strong> ist danach heute nicht mehr
          zusätzlich für diese Gruppe zuständig.
        </p>
      </ConfirmationModal>
    );
  }

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
        <div
          ref={formRef}
          className="flex-1 space-y-6 overflow-y-auto px-5 py-4"
        >
          <FormErrorAlert message={formErrors.error} />
          <LoadErrorAlert error={loadError} />

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
              <ul className="moto-content-surface divide-y divide-gray-200 overflow-hidden rounded-2xl border shadow-sm">
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
                      onClick={() => setCancelTarget(transfer)}
                      isLoading={deletingId === transfer.substitutionId}
                      loadingText="Wird zurückgenommen…"
                      disabled={deletingId === transfer.substitutionId}
                    >
                      Zurücknehmen
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
