"use client";

import { useCallback, useState } from "react";

import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "DeleteDeviceModal" });

interface DeleteDeviceModalProps {
  device: OperatorDevice | null;
  onClose: () => void;
  onDeleted: () => Promise<void> | void;
}

export function DeleteDeviceModal({
  device,
  onClose,
  onDeleted,
}: Readonly<DeleteDeviceModalProps>) {
  const [confirmed, setConfirmed] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const close = useCallback(() => {
    setConfirmed(false);
    setError("");
    onClose();
  }, [onClose]);

  const handleDelete = useCallback(async () => {
    if (!device) return;
    setLoading(true);
    setError("");
    try {
      await operatorProvisioningService.deleteDevice(device.id);
      setConfirmed(false);
      await onDeleted();
      onClose();
    } catch (err) {
      logger.error("device_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error ? err.message : "Fehler beim Löschen des Geräts",
      );
    } finally {
      setLoading(false);
    }
  }, [device, onClose, onDeleted]);

  if (!device) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
        <h3 className="text-lg font-semibold text-gray-900">Gerät löschen</h3>
        <p className="mt-2 text-sm text-gray-600">
          Möchten Sie das Gerät{" "}
          <span className="font-mono font-medium">{device.deviceId}</span>
          {device.name && ` (${device.name})`} von{" "}
          <span className="font-medium">{device.schoolName}</span> wirklich
          löschen?
        </p>
        <p className="mt-2 text-sm font-medium text-[#CC2626]">
          Diese Aktion kann nicht rückgängig gemacht werden.
        </p>

        {error && (
          <div className="mt-3 rounded-lg bg-[#FF3130]/10 px-3 py-2 text-sm text-[#CC2626]">
            {error}
          </div>
        )}

        {!confirmed ? (
          <div className="mt-5 flex justify-end gap-3">
            <button
              type="button"
              onClick={close}
              className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={() => setConfirmed(true)}
              className="rounded-lg bg-[#FF3130] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#CC2626]"
            >
              Ja, löschen
            </button>
          </div>
        ) : (
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
              disabled={loading}
              className="rounded-lg bg-[#FF3130] px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-[#CC2626] disabled:opacity-50"
            >
              {loading ? "Wird gelöscht..." : "Endgültig löschen"}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
