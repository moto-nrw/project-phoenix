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
import { FramedList } from "~/components/ui/framed-list";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import {
  SlideOver,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
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
  // Übergeben and Zurücknehmen run in the panel's session (#6593); another
  // group starts a new one.
  const save = useFormModalPending(isOpen, group?.id);
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

    await save.run(async (op) => {
      try {
        await onTransfer(selectedStaffId, targetName);
        if (op.isCurrent()) setSelectedStaffId("");
      } catch (err) {
        logger.error("group_transfer_failed", {
          error: err instanceof Error ? err.message : String(err),
          group_id: group?.id,
        });
        // Protokolliert; eine späte Ablehnung zeigt kein neuerer Vorgang.
        if (!op.isCurrent()) return;
        void formErrors.show(err, {
          object: "die Übergabe",
          retry: () => latestTransferRef.current(),
        });
      }
    });
  };

  const handleCancel = async (substitutionId: string) => {
    if (!onCancelTransfer) return;

    // The confirmation replaces the panel meanwhile. Running in the panel's
    // session keeps a second save out and a late error out of a later one.
    await save.run(async (op) => {
      try {
        setDeletingId(substitutionId);
        formErrors.clear();
        await onCancelTransfer(substitutionId);
      } catch (err) {
        logger.error("group_transfer_cancel_failed", {
          error: err instanceof Error ? err.message : String(err),
          substitution_id: substitutionId,
        });
        // Protokolliert; eine späte Ablehnung zeigt kein neuerer Vorgang.
        if (!op.isCurrent()) return;
        // Kein Wiederholen-Link: der Dialog ist dann schon zu, Zurücknehmen
        // startet man erneut über die Liste.
        void formErrors.show(err, { object: "die Übergabe" });
      } finally {
        setDeletingId(null);
      }
    });
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
        noTooltip="Schließt das Fenster, die Gruppe wird nicht übergeben."
      >
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void handleTransfer()}
        isLoading={save.pending}
        loadingText="Wird übergeben..."
        disabled={!selectedStaffId || availableUsers.length === 0}
        tooltip="Macht die gewählte Fachkraft heute zusätzlich für die Gruppe zuständig."
        disabledReason={
          // Bei einem Ladefehler ist die Liste nur scheinbar leer: dort steht
          // der Fehler, der Knopf behauptet nichts (#6643).
          loadError
            ? undefined
            : availableUsers.length === 0
              ? "Es ist keine pädagogische Fachkraft verfügbar."
              : "Wählen Sie zuerst eine Fachkraft aus."
        }
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
        confirmTooltip="Beendet die Übergabe für heute sofort."
        cancelText="Abbrechen"
        isConfirmLoading={deletingId !== null}
        loadingText="Wird zurückgenommen …"
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
      pending={save}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onClose();
      }}
    >
      <SlideOverContent size="md">
        <SlideOverHeader>
          <SlideOverTitle>{`Gruppe "${group.name}" übergeben`}</SlideOverTitle>
          <SlideOverDescription>
            Die Verantwortung für diese Gruppe heute übergeben.
          </SlideOverDescription>
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
              „Gruppen“ als „Vertretung“.
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
              <FramedList>
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
                      tooltip="Beendet die Übergabe an diese Person. moto fragt vorher nach."
                      isLoading={deletingId === transfer.substitutionId}
                      loadingText="Wird zurückgenommen…"
                      // Die Rückfrage ersetzt das Panel; eine laufende
                      // Übergabe darf es so nicht entsperren.
                      disabled={
                        save.pending || deletingId === transfer.substitutionId
                      }
                    >
                      Zurücknehmen
                    </Button>
                  </li>
                ))}
              </FramedList>
            </section>
          )}

          <CustomSelect
            id="transfer-user-select"
            label="Übergeben an:"
            hint={
              availableUsers.length === 0 && !loadError
                ? "Keine pädagogische Fachkraft verfügbar. Bitte wenden Sie sich an die Verwaltung."
                : undefined
            }
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
        </div>
        <SlideOverFooter>{footer}</SlideOverFooter>
      </SlideOverContent>
    </SlideOver>
  );
}
