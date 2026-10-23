import { API_KEY_COPY_FAILED_MESSAGE } from "~/lib/operator/provisioning-helpers";
import { useState, useCallback, useEffect, useRef } from "react";
import { Modal } from "~/components/ui/modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiErrorDisplay, useApiFormError } from "~/contexts/ToastContext";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  School,
  OperatorDevice,
} from "~/lib/operator/provisioning-helpers";
import { DEVICE_TYPE_OPTIONS } from "~/lib/iot-helpers";
import { createLogger } from "~/lib/logger";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormField } from "./provisioning-shared";

const logger = createLogger({ component: "CreateDeviceModal" });

export function CreateDeviceModal({
  isOpen,
  onClose,
  schools,
  onCreated,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly schools: School[] | undefined;
  readonly onCreated: (device: OperatorDevice) => void;
}) {
  const [schoolId, setSchoolId] = useState("");
  const [deviceId, setDeviceId] = useState("");
  const [deviceType, setDeviceType] = useState("");
  const [name, setName] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { show: showError, invalid, clear: clearError } = formErrors;
  const { showMessage } = useApiErrorDisplay();

  const [createdDevice, setCreatedDevice] = useState<OperatorDevice | null>(
    null,
  );
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setCreatedDevice(null);
    if (isOpen) {
      setSchoolId(schools?.length === 1 ? schools[0]!.id : "");
      setDeviceId("");
      setDeviceType("");
      setName("");
      clearError();
      setCreatedDevice(null);
      setCopied(false);
    }
    // schools is intentionally omitted: callers pass a fresh array reference on
    // every render, and onCreated handlers refetch summaries which re-creates
    // that array — a schools-driven reset would wipe the success view (and the
    // generated API key) before the user can read it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen]);

  const handleCreate = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      // CustomSelect's `required` is ARIA-only — it does not join native form
      // constraint validation, so an Enter-submit with school or device type
      // unset lands here and must produce a visible error instead of a silent
      // return. The device-ID text input keeps its native `required`.
      if (!schoolId) {
        const hint = "Bitte wählen Sie eine Schule aus.";
        invalid("Bitte prüfen Sie die markierten Felder.", { school_id: hint });
        return;
      }
      if (!deviceType) {
        const hint = "Bitte wählen Sie einen Gerätetyp aus.";
        invalid("Bitte prüfen Sie die markierten Felder.", {
          device_type: hint,
        });
        return;
      }
      if (!deviceId.trim()) return;

      setIsSaving(true);
      clearError();
      try {
        const device = await operatorProvisioningService.createDevice({
          school_id: parseInt(schoolId, 10),
          device_id: deviceId.trim(),
          device_type: deviceType,
          ...(name.trim() && { name: name.trim() }),
        });
        setCreatedDevice(device);
        onCreated({ ...device, apiKey: "" });
      } catch (err) {
        logger.error("device_create_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        void showError(err, { object: "das Anlegen des Geräts" });
      } finally {
        setIsSaving(false);
      }
    },
    [
      schoolId,
      deviceId,
      deviceType,
      name,
      onCreated,
      invalid,
      clearError,
      showError,
    ],
  );

  const handleCopyApiKey = useCallback(async () => {
    if (!createdDevice?.apiKey) return;
    try {
      await navigator.clipboard.writeText(createdDevice.apiKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      logger.error("clipboard_copy_failed", {
        error: "Failed to copy API key to clipboard",
      });
      showMessage(API_KEY_COPY_FAILED_MESSAGE);
    }
  }, [createdDevice, showMessage]);

  if (createdDevice) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} title="Gerät erstellt">
        <div className="space-y-4">
          <div className="border-moto-green/20 bg-moto-green/10 rounded-lg border p-4">
            <p className="text-moto-green-strong text-sm font-medium">
              Gerät erfolgreich erstellt
            </p>
            <div className="text-moto-green-strong mt-2 space-y-1 text-sm">
              <p>
                Geräte-ID:{" "}
                <span className="font-mono">{createdDevice.deviceId}</span>
              </p>
              <p>Schule: {createdDevice.schoolName}</p>
            </div>
          </div>

          <p className="text-sm text-gray-600">
            Kopieren Sie den API-Key jetzt. Er wird nur einmal angezeigt.
          </p>
          {createdDevice.apiKey && (
            <div className="space-y-2">
              <p className="text-sm font-medium text-gray-700">API-Key:</p>
              <div className="flex items-center gap-2">
                <code className="flex-1 rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 font-mono text-xs break-all">
                  {createdDevice.apiKey}
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
                Der API-Key kann jederzeit im Geräte-Tab kopiert werden.
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
      title="Neues Gerät"
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
            onClick={(e) => void handleCreate(e)}
            disabled={isSaving || !schoolId || !deviceId.trim() || !deviceType}
            className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isSaving ? "Wird erstellt..." : "Erstellen"}
          </button>
        </>
      }
    >
      <form
        ref={formRef}
        onSubmit={(e) => void handleCreate(e)}
        className="space-y-4"
        id="create-device-form"
      >
        <FormErrorAlert message={formErrors.error} />
        <FormField
          label="Schule"
          htmlFor="device-school"
          required
          error={formErrors.fieldError("school_id")}
        >
          <CustomSelect
            id="device-school"
            ariaLabel="Schule"
            value={schoolId}
            options={[
              { value: "", label: "Schule auswählen..." },
              ...(schools?.map((school) => ({
                value: school.id,
                label: school.name,
              })) ?? []),
            ]}
            onChange={setSchoolId}
            placeholder="Schule auswählen..."
            required
          />
        </FormField>

        <FormField
          label="Geräte-ID"
          htmlFor="device-id"
          required
          error={formErrors.fieldError("device_id")}
        >
          <input
            id="device-id"
            name="device_id"
            type="text"
            value={deviceId}
            onChange={(e) => setDeviceId(e.target.value)}
            placeholder="z.B. T-001"
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 font-mono text-sm focus:ring-2 focus:outline-none"
            required
          />
        </FormField>

        <FormField
          label="Typ"
          htmlFor="device-type"
          required
          error={formErrors.fieldError("device_type")}
        >
          <CustomSelect
            id="device-type"
            ariaLabel="Typ"
            value={deviceType}
            options={[
              { value: "", label: "Typ auswählen..." },
              ...Object.entries(DEVICE_TYPE_OPTIONS).map(([value, label]) => ({
                value,
                label,
              })),
            ]}
            onChange={setDeviceType}
            placeholder="Typ auswählen..."
            required
          />
        </FormField>

        <FormField
          label="Name"
          htmlFor="device-name"
          error={formErrors.fieldError("name")}
        >
          <input
            id="device-name"
            name="name"
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="z.B. Eingangsbereich"
            maxLength={255}
            className="focus:ring-moto-blue w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:ring-2 focus:outline-none"
          />
        </FormField>

        <p className="text-sm text-gray-600">
          Der API-Key wird automatisch erstellt und nur einmal angezeigt.
        </p>
      </form>
    </Modal>
  );
}
