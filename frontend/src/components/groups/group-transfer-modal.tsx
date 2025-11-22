"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";

interface GroupTransferModalProps {
  isOpen: boolean;
  onClose: () => void;
  group: {
    id: string;
    name: string;
    studentCount?: number;
  } | null;
  availableUsers: Array<{
    id: string;
    personId: string;
    firstName: string;
    lastName: string;
    fullName: string;
    email: string;
  }>;
  onTransfer: (targetPersonId: string) => Promise<void>;
  existingTransfer?: {
    targetName: string;
    substitutionId: string;
  } | null;
  onCancelTransfer?: () => Promise<void>;
}

export function GroupTransferModal({
  isOpen,
  onClose,
  group,
  availableUsers,
  onTransfer,
  existingTransfer,
  onCancelTransfer,
}: GroupTransferModalProps) {
  const [selectedPersonId, setSelectedPersonId] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset form when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setSelectedPersonId("");
      setError(null);
    }
  }, [isOpen]);

  const handleTransfer = async () => {
    if (!selectedPersonId) {
      setError("Bitte wählen Sie einen Betreuer aus.");
      return;
    }

    try {
      setLoading(true);
      setError(null);
      await onTransfer(selectedPersonId);
      onClose();
    } catch (err) {
      console.error("Transfer error:", err);
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Übergeben der Gruppe. Bitte versuchen Sie es erneut.",
      );
    } finally {
      setLoading(false);
    }
  };

  const handleCancel = async () => {
    if (!onCancelTransfer) return;

    try {
      setLoading(true);
      setError(null);
      await onCancelTransfer();
      onClose();
    } catch (err) {
      console.error("Cancel transfer error:", err);
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Zurücknehmen. Bitte versuchen Sie es erneut.",
      );
    } finally {
      setLoading(false);
    }
  };

  if (!group) return null;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`Gruppe "${group.name}" übergeben`}
    >
      <div className="space-y-4 md:space-y-5">
        {/* Error Alert */}
        {error && (
          <div className="rounded-lg border border-red-200 bg-red-50 p-3">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {/* Info Box */}
        <div className="rounded-lg border border-blue-100 bg-blue-50/50 p-3 md:p-4">
          <div className="flex items-start gap-3">
            <svg
              className="mt-0.5 h-5 w-5 flex-shrink-0 text-blue-600"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <div className="flex-1">
              <p className="text-sm text-gray-700">
                Der ausgewählte Betreuer erhält{" "}
                <strong>zusätzliche Berechtigungen</strong> für diese Gruppe bis{" "}
                <strong>heute 23:59 Uhr</strong>.
              </p>
              <p className="mt-2 text-sm text-gray-600">
                Du behältst weiterhin vollen Zugriff auf die Gruppe.
              </p>
            </div>
          </div>
        </div>

        {/* Group Info */}
        <div className="rounded-lg border border-gray-100 bg-gray-50 p-3 md:p-4">
          <p className="text-sm text-gray-600">
            <span className="font-medium text-gray-900">Gruppe:</span>{" "}
            {group.name}
          </p>
          {group.studentCount !== undefined && (
            <p className="mt-1 text-sm text-gray-600">
              <span className="font-medium text-gray-900">Schüler:</span>{" "}
              {group.studentCount}
            </p>
          )}
        </div>

        {/* Existing Transfer or New Transfer */}
        {existingTransfer ? (
          <div className="rounded-lg border border-orange-200 bg-orange-50 p-3 md:p-4">
            <div className="flex items-start gap-3">
              <svg
                className="mt-0.5 h-5 w-5 flex-shrink-0 text-orange-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
              <div className="flex-1">
                <p className="text-sm font-medium text-orange-900">
                  Bereits übergeben
                </p>
                <p className="mt-1 text-sm text-orange-800">
                  Diese Gruppe wurde bereits an{" "}
                  <strong>{existingTransfer.targetName}</strong> übergeben.
                </p>
              </div>
            </div>

            <button
              onClick={handleCancel}
              disabled={loading}
              className="mt-4 w-full rounded-lg border border-red-200 bg-red-50 px-4 py-2.5 text-sm font-medium text-red-700 transition-all duration-200 hover:bg-red-100 active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {loading ? "Wird zurückgenommen..." : "Übergabe zurücknehmen"}
            </button>
          </div>
        ) : (
          <>
            {/* User Dropdown */}
            <div>
              <label className="mb-2 block text-sm font-medium text-gray-700">
                Übergeben an:
              </label>
              <div className="relative">
                <select
                  value={selectedPersonId}
                  onChange={(e) => setSelectedPersonId(e.target.value)}
                  className="block w-full cursor-pointer appearance-none rounded-lg border border-gray-200 bg-white py-3 pr-10 pl-4 text-base text-gray-900 transition-colors focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 md:text-lg"
                >
                  <option value="">Betreuer auswählen...</option>
                  {availableUsers.map((user) => (
                    <option key={user.id} value={user.personId}>
                      {user.fullName}
                    </option>
                  ))}
                </select>
                {/* Custom dropdown arrow */}
                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
                  <svg
                    className="h-5 w-5 text-gray-400"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                  >
                    <path
                      fillRule="evenodd"
                      d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z"
                      clipRule="evenodd"
                    />
                  </svg>
                </div>
              </div>
              {availableUsers.length === 0 && (
                <p className="mt-2 text-sm text-gray-500">
                  Keine Betreuer mit Rolle &quot;user&quot; verfügbar. Bitte
                  erstellen Sie zuerst Benutzer mit dieser Rolle.
                </p>
              )}
            </div>

            {/* Action Buttons */}
            <div className="flex gap-3 pt-2 md:pt-4">
              <button
                type="button"
                onClick={onClose}
                disabled={loading}
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:hover:scale-105"
              >
                Abbrechen
              </button>

              <button
                type="button"
                onClick={handleTransfer}
                disabled={
                  !selectedPersonId || loading || availableUsers.length === 0
                }
                className="flex-1 rounded-lg bg-blue-600 px-4 py-2.5 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-blue-700 hover:shadow-lg active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100 md:hover:scale-105"
              >
                {loading ? (
                  <span className="flex items-center justify-center gap-2">
                    <svg
                      className="h-4 w-4 animate-spin text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      ></circle>
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                    Wird übergeben...
                  </span>
                ) : (
                  "Übergeben"
                )}
              </button>
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}
