import { useState, useCallback, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { useScrollToError } from "~/lib/hooks/use-scroll-to-error";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { createLogger } from "~/lib/logger";
import { FormError } from "./provisioning-shared";

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
  const [mode, setMode] = useState<"auto" | "manual">("auto");
  const [customKey, setCustomKey] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState("");
  const errorRef = useScrollToError(error);

  const [updatedDevice, setUpdatedDevice] = useState<OperatorDevice | null>(
    null,
  );
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setMode("auto");
      setCustomKey("");
      setError("");
      setUpdatedDevice(null);
      setCopied(false);
    }
  }, [isOpen]);

  const handleSubmit = useCallback(async () => {
    if (!device) return;

    setIsSaving(true);
    setError("");
    try {
      const apiKey =
        mode === "manual" && customKey.trim() ? customKey.trim() : undefined;
      const result = await operatorProvisioningService.setDeviceAPIKey(
        device.id,
        apiKey,
      );
      setUpdatedDevice(result);
      onKeySet(result);
    } catch (err) {
      if (isOperatorApiError(err) && err.status === 409) {
        setError("Dieser API-Key wird bereits verwendet.");
      } else if (isOperatorApiError(err) && err.status === 404) {
        setError("Gerät nicht gefunden.");
      } else {
        setError(
          err instanceof Error
            ? err.message
            : "Fehler beim Ändern des API-Keys.",
        );
        logger.error("set_api_key_failed", {
          error: err instanceof Error ? err.message : String(err),
          device_id: device.id,
        });
      }
    } finally {
      setIsSaving(false);
    }
  }, [device, mode, customKey, onKeySet]);

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
    }
  }, [updatedDevice]);

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
                <button
                  type="button"
                  onClick={() => void handleCopyApiKey()}
                  className="shrink-0 rounded-lg border border-gray-200 px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
                >
                  {copied ? "Kopiert!" : "Kopieren"}
                </button>
              </div>
              <p className="text-xs text-gray-500">
                Der neue Key ist aktiv. Der alte Key ist ab sofort ungültig.
              </p>
            </div>
          )}

          <div className="flex justify-end pt-2">
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800"
            >
              Schließen
            </button>
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
          <button
            type="button"
            onClick={onClose}
            className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={() => void handleSubmit()}
            disabled={isSaving || (mode === "manual" && !customKey.trim())}
            className="bg-moto-red hover:bg-moto-red-hover flex-1 rounded-lg px-4 py-2 text-sm font-medium text-white transition-colors disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSaving ? "Wird geändert..." : "Übernehmen"}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-600">
          API-Key für <span className="font-medium">{deviceLabel}</span> ändern.
        </p>

        <div className="border-moto-amber/30 bg-moto-amber-soft rounded-lg border p-3">
          <p className="text-moto-amber-strong text-sm">
            Der alte Key wird sofort ungültig. Alle Geräte, die diesen Key
            nutzen, verlieren den Zugriff.
          </p>
        </div>

        <div className="space-y-3">
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="radio"
              name="set-key-mode"
              value="auto"
              checked={mode === "auto"}
              onChange={() => setMode("auto")}
              className="text-moto-blue"
            />
            <span className="text-sm text-gray-700">Neuen Key generieren</span>
          </label>
          <label className="flex cursor-pointer items-center gap-2">
            <input
              type="radio"
              name="set-key-mode"
              value="manual"
              checked={mode === "manual"}
              onChange={() => setMode("manual")}
              className="text-moto-blue"
            />
            <span className="text-sm text-gray-700">Eigenen Key eingeben</span>
          </label>
          {mode === "manual" && (
            <input
              type="text"
              value={customKey}
              onChange={(e) => setCustomKey(e.target.value)}
              placeholder="API-Key eingeben..."
              maxLength={255}
              className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
            />
          )}
        </div>

        {error && <FormError ref={errorRef} message={error} />}
      </div>
    </Modal>
  );
}
