"use client";

import { useCallback, useState } from "react";

import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import { useApiFormError } from "~/contexts/ToastContext";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";

const logger = createLogger({ component: "DeleteDeviceModal" });

interface DeleteDeviceModalProps {
  device: OperatorDevice | null;
  onClose: () => void;
  /** Reloads the device lists; never rejects (#6652). */
  onDeleted: RefreshAfterWrite;
}

export function DeleteDeviceModal({
  device,
  onClose,
  onDeleted,
}: Readonly<DeleteDeviceModalProps>) {
  const [loading, setLoading] = useState(false);
  const deleteErrors = useApiFormError();
  const { show: showError, clear: clearError } = deleteErrors;

  const handleClose = useCallback(() => {
    clearError();
    onClose();
  }, [onClose, clearError]);

  const handleDelete = useCallback(async () => {
    if (!device) return;
    setLoading(true);
    clearError();
    try {
      await operatorProvisioningService.deleteDevice(device.id);
    } catch (err) {
      logger.error("device_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "das Löschen des Geräts" });
      setLoading(false);
      return;
    }
    setLoading(false);
    onClose();
    await onDeleted();
  }, [device, onClose, onDeleted, showError, clearError]);

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
      error={deleteErrors.error}
    />
  );
}
