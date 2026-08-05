"use client";

import {
  useCallback,
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
} from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { useToast } from "~/contexts/ToastContext";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  DeviceTransferStatus,
  OperatorDevice,
  School,
} from "~/lib/operator/provisioning-helpers";

interface TransferDeviceModalProps {
  readonly device: OperatorDevice | null;
  readonly schools: readonly School[];
  readonly onClose: () => void;
  readonly onTransferred: (device: OperatorDevice) => Promise<void> | void;
}

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat("de-DE", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}

export function TransferDeviceModal({
  device,
  schools,
  onClose,
  onTransferred,
}: TransferDeviceModalProps) {
  const [targetSchoolId, setTargetSchoolId] = useState("");
  const [status, setStatus] = useState<DeviceTransferStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const statusRequestId = useRef(0);
  const schoolLabelId = useId();
  const { success: toastSuccess } = useToast();

  const destinations = useMemo(
    () =>
      [...schools]
        .filter(
          (school) =>
            device != null &&
            school.organizationId === device.organizationId &&
            school.id !== device.schoolId &&
            school.active &&
            school.deletedAt == null,
        )
        .sort((left, right) => left.name.localeCompare(right.name, "de")),
    [device, schools],
  );

  const refreshStatus = useCallback(async () => {
    if (!device) return;
    const requestId = ++statusRequestId.current;
    setStatusLoading(true);
    setError("");
    try {
      const nextStatus =
        await operatorProvisioningService.getDeviceTransferStatus(device.id);
      if (statusRequestId.current === requestId) setStatus(nextStatus);
    } catch (statusError) {
      if (statusRequestId.current === requestId) {
        setStatus(null);
        setError(
          statusError instanceof Error
            ? statusError.message
            : "Der Gerätestatus konnte nicht geladen werden.",
        );
      }
    } finally {
      if (statusRequestId.current === requestId) setStatusLoading(false);
    }
  }, [device]);

  useEffect(() => {
    if (!device) {
      statusRequestId.current += 1;
      setTargetSchoolId("");
      setStatus(null);
      setError("");
      return;
    }
    setTargetSchoolId("");
    setStatus(null);
    setError("");
    void refreshStatus();
  }, [device, refreshStatus]);

  const handleTransfer = useCallback(async () => {
    if (!device || !targetSchoolId || !status?.canTransfer) return;
    setSaving(true);
    setError("");
    try {
      const transferred = await operatorProvisioningService.transferDevice(
        device.id,
        targetSchoolId,
      );
      await onTransferred(transferred);
      toastSuccess(
        `${device.name || device.deviceId} wurde nach ${transferred.schoolName} verschoben.`,
      );
      onClose();
    } catch (transferError) {
      if (isOperatorApiError(transferError) && transferError.status === 409) {
        setError(
          "Das Gerät kann gerade nicht verschoben werden. Prüfe den aktuellen Status.",
        );
        await refreshStatus();
      } else {
        setError(
          transferError instanceof Error
            ? transferError.message
            : "Das Gerät konnte nicht verschoben werden.",
        );
      }
    } finally {
      setSaving(false);
    }
  }, [
    device,
    onClose,
    onTransferred,
    refreshStatus,
    status?.canTransfer,
    targetSchoolId,
    toastSuccess,
  ]);

  const statusRetry = (
    <Button
      type="button"
      variant="ghost"
      size="compact"
      onClick={() => void refreshStatus()}
      disabled={statusLoading}
    >
      Erneut prüfen
    </Button>
  );

  return (
    <FormModal
      isOpen={device != null}
      onClose={onClose}
      title="Gerät verschieben"
      size="md"
      footer={
        <>
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Abbrechen
          </Button>
          <Button
            type="button"
            size="md"
            onClick={() => void handleTransfer()}
            isLoading={saving}
            loadingText="Wird verschoben…"
            disabled={
              saving ||
              statusLoading ||
              status?.canTransfer !== true ||
              targetSchoolId === ""
            }
          >
            Schule wechseln
          </Button>
        </>
      }
    >
      {device ? (
        <div className="space-y-5">
          <div>
            <p className="text-sm text-gray-600">Aktuelle Schule</p>
            <p className="mt-1 font-medium text-gray-900">
              {device.schoolName}
            </p>
            <p className="mt-1 font-mono text-xs text-gray-500">
              {device.deviceId}
              {device.name ? ` · ${device.name}` : ""}
            </p>
          </div>

          <Alert
            type="info"
            message="Geräte-ID und API-Key bleiben erhalten. Die Raumzuordnung wird entfernt; bisherige Anwesenheiten und Sitzungen bleiben bei der aktuellen Schule."
          />

          {statusLoading && status == null ? (
            <p role="status" className="text-sm text-gray-500">
              Gerätestatus wird geprüft…
            </p>
          ) : null}

          {status?.isOnline ? (
            <Alert
              type="warning"
              message={`Das Gerät ist online${status.lastSeen ? ` (zuletzt gesehen ${formatTimestamp(status.lastSeen)})` : ""}. Schalte es aus und warte mindestens fünf Minuten.`}
              action={statusRetry}
            />
          ) : null}

          {status?.activeSession ? (
            <Alert
              type="warning"
              message={`Offene Sitzung seit ${formatTimestamp(status.activeSession.startedAt)}${status.activeSession.activityName ? ` · ${status.activeSession.activityName}` : ""}${status.activeSession.roomName ? ` · ${status.activeSession.roomName}` : ""}. Beende die Sitzung vor dem Verschieben.`}
              action={statusRetry}
            />
          ) : null}

          {status?.isProtected ? (
            <Alert
              type="warning"
              message="Dieses Systemgerät wird für manuelle Web-Buchungen benötigt und kann nicht verschoben werden."
            />
          ) : null}

          {status?.canTransfer ? (
            <Alert
              type="success"
              message="Das Gerät ist offline und hat keine offene Sitzung."
            />
          ) : null}

          {destinations.length > 0 ? (
            <div className="space-y-2">
              <span
                id={schoolLabelId}
                className="block text-sm font-medium text-gray-700"
              >
                Zielschule
              </span>
              <CustomSelect
                value={targetSchoolId}
                options={destinations.map((school) => ({
                  value: school.id,
                  label: school.name,
                }))}
                onChange={setTargetSchoolId}
                ariaLabelledBy={schoolLabelId}
                placeholder="Zielschule wählen"
                disabled={status?.canTransfer !== true || statusLoading}
              />
            </div>
          ) : (
            <Alert
              type="warning"
              message="Für diesen Träger gibt es keine weitere aktive Schule als Ziel."
            />
          )}

          {error ? <Alert type="error" message={error} /> : null}
        </div>
      ) : null}
    </FormModal>
  );
}
