"use client";

import { useCallback, useState } from "react";

import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { createLogger } from "~/lib/logger";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";

const logger = createLogger({ component: "DeletePersonModal" });

interface DeletePersonModalProps {
  person: OperatorPerson | null;
  onClose: () => void;
  onDeleted: (person: OperatorPerson) => Promise<void> | void;
}

export function DeletePersonModal({
  person,
  onClose,
  onDeleted,
}: Readonly<DeletePersonModalProps>) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleClose = useCallback(() => {
    setError("");
    onClose();
  }, [onClose]);

  const handleDelete = useCallback(async () => {
    if (!person) return;
    setLoading(true);
    setError("");
    try {
      await operatorProvisioningService.softDeletePerson(person.id);
      await onDeleted(person);
      onClose();
    } catch (err) {
      logger.error("person_soft_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error ? err.message : "Fehler beim Löschen der Person",
      );
    } finally {
      setLoading(false);
    }
  }, [person, onClose, onDeleted]);

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
        <div className="bg-moto-amber/10 text-moto-amber-strong rounded-lg px-3 py-2 text-sm">
          <p className="font-medium">Folgende Aktionen werden ausgeführt:</p>
          <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
            <li>Account wird deaktiviert und Login gesperrt</li>
            <li>Persönliche Daten werden anonymisiert</li>
            <li>RFID-Karte wird freigegeben</li>
            <li>Diese Aktion kann nicht rückgängig gemacht werden</li>
          </ul>
        </div>
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
      error={error}
    />
  );
}
