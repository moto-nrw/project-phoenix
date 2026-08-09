"use client";

import { useCallback, useState } from "react";

import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { createLogger } from "~/lib/logger";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";

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
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const handleClose = useCallback(() => {
    setError("");
    onClose();
  }, [onClose]);

  const handleDelete = useCallback(async () => {
    if (!device) return;
    setLoading(true);
    setError("");
    try {
      await operatorProvisioningService.deleteDevice(device.id);
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

  return (
    <ConfirmDeleteModal
      isOpen={Boolean(device)}
      title="Gerät löschen"
      description={
        device ? (
          <>
            <p>
              Möchten Sie das Gerät{" "}
              <span className="font-mono font-medium">{device.deviceId}</span>
              {device.name && ` (${device.name})`} von{" "}
              <span className="font-medium">{device.schoolName}</span> wirklich
              löschen?
            </p>
            <p className="text-moto-red-strong mt-2 font-medium">
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>
          </>
        ) : null
      }
      gate={{ mode: "twoStep" }}
      onConfirm={handleDelete}
      onClose={handleClose}
      loading={loading}
      error={error}
    />
  );
}
