import { API_KEY_COPY_FAILED_MESSAGE } from "~/lib/operator/provisioning-helpers";
import { useState, useCallback, useEffect, useRef } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CopyField } from "~/components/ui/copy-field";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SetApiKeyModal" });

export function SetApiKeyModal({
  isOpen,
  onClose,
  device,
  onKeySet,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly device: OperatorDevice | null;
  readonly onKeySet: (device: OperatorDevice) => void;
}) {
  const [isSaving, setIsSaving] = useState(false);
  const formRef = useRef<HTMLDivElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, clear: clearError } = formErrors;
  const { showMessage } = useApiErrorDisplay();

  const [updatedDevice, setUpdatedDevice] = useState<OperatorDevice | null>(
    null,
  );

  useEffect(() => {
    setUpdatedDevice(null);
    if (isOpen) {
      clearError();
      setUpdatedDevice(null);
    }
  }, [isOpen, clearError]);

  const handleSubmit = useCallback(async () => {
    if (!device) return;

    setIsSaving(true);
    clearError();
    try {
      const result = await operatorProvisioningService.setDeviceAPIKey(
        device.id,
      );
      setUpdatedDevice(result);
      onKeySet({ ...result, apiKey: "" });
    } catch (err) {
      logger.error("set_api_key_failed", {
        error: err instanceof Error ? err.message : String(err),
        device_id: device.id,
      });
      void showError(err, { object: "die Änderung des API-Keys" });
    } finally {
      setIsSaving(false);
    }
  }, [device, onKeySet, clearError, showError]);

  const handleCopyFailed = useCallback(() => {
    logger.error("clipboard_copy_failed", {
      error: "Failed to copy API key to clipboard",
    });
    showMessage(API_KEY_COPY_FAILED_MESSAGE);
  }, [showMessage]);

  const deviceLabel = device?.name ?? device?.deviceId ?? "Gerät";

  if (updatedDevice) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} title="API-Key aktualisiert">
        <div className="space-y-4">
          <Alert
            type="success"
            announce="off"
            message={`API-Key für ${deviceLabel} wurde aktualisiert.`}
          />

          {updatedDevice.apiKey && (
            <div className="space-y-2">
              <p className="text-sm font-medium text-gray-700">
                Neuer API-Key:
              </p>
              <CopyField
                value={updatedDevice.apiKey}
                monospace
                onCopyFailed={handleCopyFailed}
              />
              <p className="text-xs text-gray-500">
                Kopieren Sie den API-Key jetzt. Er wird nur einmal angezeigt.
                Der alte Key ist ab sofort ungültig.
              </p>
            </div>
          )}

          <div className="flex justify-end pt-2">
            <Button type="button" variant="primary" size="md" onClick={onClose}>
              Schließen
            </Button>
          </div>
        </div>
      </Modal>
    );
  }

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={`API-Key ändern`}
      footer={
        <>
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            className="flex-1"
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="danger"
            size="md"
            onClick={() => void handleSubmit()}
            disabled={isSaving}
            className="flex-1"
          >
            {isSaving ? "Wird geändert..." : "Übernehmen"}
          </Button>
        </>
      }
    >
      <div ref={formRef} className="space-y-4">
        <FormErrorAlert message={formErrors.error} />
        <p className="text-sm text-gray-600">
          API-Key für <span className="font-medium">{deviceLabel}</span> ändern.
        </p>

        <Alert
          type="warning"
          announce="off"
          message="Der alte Key wird sofort ungültig. Alle Geräte, die diesen Key nutzen, verlieren den Zugriff."
        />

        <p className="text-sm text-gray-600">
          Ein neuer API-Key wird automatisch erstellt und nur einmal angezeigt.
        </p>
      </div>
    </Modal>
  );
}
