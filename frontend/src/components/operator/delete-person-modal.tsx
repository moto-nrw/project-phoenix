"use client";

import { useCallback, useState } from "react";

import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DeletePersonModal" });

interface DeletePersonModalProps {
  person: OperatorPerson | null;
  onClose: () => void;
  onDeleted: () => Promise<void> | void;
}

export function DeletePersonModal({
  person,
  onClose,
  onDeleted,
}: Readonly<DeletePersonModalProps>) {
  const [confirmInput, setConfirmInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const close = useCallback(() => {
    setConfirmInput("");
    setError("");
    onClose();
  }, [onClose]);

  const handleDelete = useCallback(async () => {
    if (!person) return;
    setLoading(true);
    setError("");
    try {
      await operatorProvisioningService.softDeletePerson(person.id);
      await onDeleted();
      setConfirmInput("");
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

  if (!person) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h3 className="text-lg font-semibold text-gray-900">Person löschen</h3>
        <p className="mt-2 text-sm text-gray-600">
          Möchten Sie{" "}
          <span className="font-medium">
            {person.firstName} {person.lastName}
          </span>{" "}
          von <span className="font-medium">{person.schoolName}</span> wirklich
          löschen?
        </p>
        <div className="mt-3 rounded-lg bg-amber-50 px-3 py-2 text-sm text-amber-800">
          <p className="font-medium">Folgende Aktionen werden ausgeführt:</p>
          <ul className="mt-1 list-inside list-disc space-y-0.5 text-xs">
            <li>Account wird deaktiviert und Login gesperrt</li>
            <li>Persönliche Daten werden anonymisiert</li>
            <li>RFID-Karte wird freigegeben</li>
            <li>Diese Aktion kann nicht rückgängig gemacht werden</li>
          </ul>
        </div>

        <div className="mt-4">
          <label
            htmlFor="delete-person-confirm-shared"
            className="block text-sm font-medium text-gray-700"
          >
            Geben Sie den vollständigen Namen ein:
          </label>
          <p className="mb-1 text-sm font-medium text-gray-900">
            {person.fullName}
          </p>
          <input
            id="delete-person-confirm-shared"
            type="text"
            value={confirmInput}
            onChange={(event) => setConfirmInput(event.target.value)}
            placeholder={person.fullName}
            className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-red-500 focus:ring-1 focus:ring-red-500 focus:outline-none"
            autoComplete="off"
          />
        </div>

        {error && (
          <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
            {error}
          </div>
        )}

        <div className="mt-5 flex justify-end gap-3">
          <button
            type="button"
            onClick={close}
            disabled={loading}
            className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={() => void handleDelete()}
            disabled={loading || confirmInput !== person.fullName}
            className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {loading ? "Wird gelöscht..." : "Endgültig löschen"}
          </button>
        </div>
      </div>
    </div>
  );
}
