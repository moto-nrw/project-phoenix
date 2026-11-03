import { API_KEY_COPY_FAILED_MESSAGE } from "~/lib/operator/provisioning-helpers";
import { useState, useCallback, useEffect, useRef } from "react";
import { Button } from "~/components/ui/button";
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
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setUpdatedDevice(null);
    if (isOpen) {
      clearError();
      setUpdatedDevice(null);
      setCopied(false);
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

  const handleCopyApiKey = useCallback(async () => {
    if (!updatedDevice?.apiKey) return;
    try {
      await navigator.clipboard.writeText(updatedDevice.apiKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      logger.error("clipboard_copy_failed", {
        error: "Failed to copy API key to clipboard",
      });
      showMessage(API_KEY_COPY_FAILED_MESSAGE);
    }
  }, [updatedDevice, showMessage]);

  const deviceLabel = device?.name ?? device?.deviceId ?? "Gerät";

  if (updatedDevice) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} title="API-Key aktualisiert">
        <div className="space-y-4">
          <div className="border-moto-green/20 bg-moto-green/10 rounded-lg border p-4">
            <p className="text-moto-green-strong text-sm font-medium">
              API-Key für {deviceLabel} wurde aktualisiert.
            </p>
          </div>

          {updatedDevice.apiKey && (
            <div className="space-y-2">
              <p className="text-sm font-medium text-gray-700">
                Neuer API-Key:
              </p>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-xs break-all">
                  {updatedDevice.apiKey}
                </code>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={() => void handleCopyApiKey()}
                  className="shrink-0"
                >
                  {copied ? "Kopiert!" : "Kopieren"}
                </Button>
              </div>
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

        <div className="border-moto-amber/30 bg-moto-amber-soft rounded-lg border p-3">
          <p className="text-moto-amber-strong text-sm">
            Der alte Key wird sofort ungültig. Alle Geräte, die diesen Key
            nutzen, verlieren den Zugriff.
          </p>
        </div>

        <p className="text-sm text-gray-600">
          Ein neuer API-Key wird automatisch erstellt und nur einmal angezeigt.
        </p>
      </div>
    </Modal>
  );
}
