"use client";

import { useCallback, useState } from "react";

import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { useApiFormError } from "~/contexts/ToastContext";
import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";

const logger = createLogger({ component: "DeletePersonModal" });

interface DeletePersonModalProps {
  person: OperatorPerson | null;
  onClose: () => void;
  /** Drops the person from the shown lists and reloads them; never rejects
   *  (#6652). */
  onDeleted: RefreshAfterWrite<[person: OperatorPerson]>;
}

export function DeletePersonModal({
  person,
  onClose,
  onDeleted,
}: Readonly<DeletePersonModalProps>) {
  const [loading, setLoading] = useState(false);
  const deleteErrors = useApiFormError();
  const { show: showError, clear: clearError } = deleteErrors;

  const handleClose = useCallback(() => {
    clearError();
    onClose();
  }, [onClose, clearError]);

  const handleDelete = useCallback(async () => {
    if (!person) return;
    setLoading(true);
    clearError();
    try {
      await operatorProvisioningService.softDeletePerson(person.id);
    } catch (err) {
      logger.error("person_soft_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "das Löschen der Person" });
      setLoading(false);
      return;
    }
    setLoading(false);
    onClose();
    await onDeleted(person);
  }, [person, onClose, onDeleted, showError, clearError]);

  return (
    <ConfirmDeleteModal
      isOpen={Boolean(person)}
      title="Person löschen"
      description={
        person ? (
          <p>
            Möchten Sie{" "}
            <span className="font-medium">
              {person.firstName} {person.lastName}
            </span>{" "}
            von <span className="font-medium">{person.schoolName}</span>{" "}
            wirklich löschen?
          </p>
        ) : null
      }
      warningSlot={
        <Alert
          type="warning"
          announce="off"
          message={
            <>
              <p className="font-medium">
                Folgende Aktionen werden ausgeführt:
              </p>
              <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
                {person?.hasAccount && (
                  <li>Account wird deaktiviert und Login gesperrt</li>
                )}
                <li>Persönliche Daten werden anonymisiert</li>
                <li>RFID-Karte wird freigegeben</li>
                <li>Diese Aktion kann nicht rückgängig gemacht werden</li>
              </ul>
            </>
          }
        />
      }
      gate={
        person
          ? {
              mode: "textConfirm",
              expected: person.fullName,
              inputId: "delete-person-confirm-shared",
              label: "Geben Sie den vollständigen Namen ein:",
              placeholder: person.fullName,
              preview: person.fullName,
            }
          : {
              mode: "textConfirm",
              expected: "",
              inputId: "delete-person-confirm-shared",
              label: "",
            }
      }
      onConfirm={handleDelete}
      onClose={handleClose}
      loading={loading}
      error={deleteErrors.error}
    />
  );
}
