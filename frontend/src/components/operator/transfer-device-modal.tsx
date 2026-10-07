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
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import type { FormError } from "~/components/ui/form-error";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type {
  DeviceTransferStatus,
  OperatorDevice,
  School,
} from "~/lib/operator/provisioning-helpers";

const logger = createLogger({ component: "TransferDeviceModal" });

interface TransferDeviceModalProps {
  readonly device: OperatorDevice | null;
  readonly schools: readonly School[];
  readonly onClose: () => void;
  readonly onTransferred: (device: OperatorDevice) => Promise<void> | void;
}

interface TransferDeviceContentProps {
  readonly device: OperatorDevice;
  readonly destinations: readonly School[];
  readonly targetSchoolId: string;
  readonly status: DeviceTransferStatus | null;
  readonly statusLoading: boolean;
  readonly statusError: FormError | null;
  readonly schoolLabelId: string;
  readonly onTargetSchoolChange: (schoolId: string) => void;
  readonly onRefreshStatus: () => Promise<void>;
}

function formatTimestamp(value: string): string {
  return new Intl.DateTimeFormat("de-DE", {
    dateStyle: "short",
    timeStyle: "short",
  }).format(new Date(value));
}

function getTransferDestinations(
  device: OperatorDevice | null,
  schools: readonly School[],
): School[] {
  const organizationId = device?.organizationId;
  const sourceSchoolId = device?.schoolId;
  if (organizationId == null || sourceSchoolId == null) return [];

  return schools
    .filter(
      (school) =>
        school.organizationId === organizationId &&
        school.id !== sourceSchoolId &&
        school.active &&
        school.deletedAt == null,
    )
    .sort((left, right) => left.name.localeCompare(right.name, "de"));
}

function formatOnlineMessage(status: DeviceTransferStatus): string {
  const lastSeen = status.lastSeen
    ? ` (zuletzt gesehen ${formatTimestamp(status.lastSeen)})`
    : "";
  return `Das Gerät ist online${lastSeen}. Schalte es aus und warte mindestens fünf Minuten.`;
}

function formatSessionMessage(
  session: NonNullable<DeviceTransferStatus["activeSession"]>,
): string {
  const activity = session.activityName ? ` · ${session.activityName}` : "";
  const room = session.roomName ? ` · ${session.roomName}` : "";
  return `Offene Sitzung seit ${formatTimestamp(session.startedAt)}${activity}${room}. Beende die Sitzung vor dem Verschieben.`;
}

function useDeviceTransferStatus(
  device: OperatorDevice | null,
  resetTargetSchool: () => void,
) {
  const [status, setStatus] = useState<DeviceTransferStatus | null>(null);
  const [statusLoading, setStatusLoading] = useState(false);
  const statusLoad = useApiLoadError();
  const { show: showStatusError, clear: clearStatusError } = statusLoad;
  const statusRequestId = useRef(0);

  const refreshStatus = useCallback(async () => {
    if (!device) return;
    const requestId = ++statusRequestId.current;
    setStatusLoading(true);
    clearStatusError();
    try {
      const nextStatus =
        await operatorProvisioningService.getDeviceTransferStatus(device.id);
      if (statusRequestId.current === requestId) setStatus(nextStatus);
    } catch (statusError) {
      if (statusRequestId.current === requestId) {
        setStatus(null);
        logger.warn("device_transfer_status_failed", {
          device_id: device.id,
          error:
            statusError instanceof Error
              ? statusError.message
              : String(statusError),
        });
        void showStatusError(statusError, {
          object: "die Prüfung des Gerätestatus",
          retry: () => void refreshStatus(),
        });
      }
    } finally {
      if (statusRequestId.current === requestId) setStatusLoading(false);
    }
  }, [device, showStatusError, clearStatusError]);

  useEffect(() => {
    resetTargetSchool();
    setStatus(null);
    clearStatusError();
    if (!device) {
      statusRequestId.current += 1;
      return;
    }
    void refreshStatus();
  }, [device, refreshStatus, resetTargetSchool, clearStatusError]);

  return {
    status,
    statusLoading,
    statusError: statusLoad.error,
    refreshStatus,
  };
}

function TransferDeviceContent({
  device,
  destinations,
  targetSchoolId,
  status,
  statusLoading,
  statusError,
  schoolLabelId,
  onTargetSchoolChange,
  onRefreshStatus,
}: TransferDeviceContentProps) {
  const statusRetry = (
    <Button
      type="button"
      variant="ghost"
      size="compact"
      onClick={() => void onRefreshStatus()}
      disabled={statusLoading}
    >
      Erneut prüfen
    </Button>
  );

  return (
    <div className="space-y-5">
      <div>
        <p className="text-sm text-gray-600">Aktuelle Schule</p>
        <p className="mt-1 font-medium text-gray-900">{device.schoolName}</p>
        <p className="mt-1 font-mono text-xs text-gray-500">
          {device.deviceId}
          {device.name ? ` · ${device.name}` : ""}
        </p>
      </div>

      <Alert
        type="info"
        message="Geräte-ID und API-Key bleiben erhalten. Die Raumzuordnung wird entfernt; bisherige Anwesenheiten und Sitzungen bleiben bei der aktuellen Schule."
      />

      <LoadErrorAlert error={statusError} />

      {statusLoading && status == null ? (
        <output className="block text-sm text-gray-500">
          Gerätestatus wird geprüft…
        </output>
      ) : null}

      {status?.isOnline ? (
        <Alert
          type="warning"
          message={formatOnlineMessage(status)}
          action={statusRetry}
        />
      ) : null}

      {status?.activeSession ? (
        <Alert
          type="warning"
          message={formatSessionMessage(status.activeSession)}
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
            onChange={onTargetSchoolChange}
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
    </div>
  );
}

export function TransferDeviceModal({
  device,
  schools,
  onClose,
  onTransferred,
}: TransferDeviceModalProps) {
  const [targetSchoolId, setTargetSchoolId] = useState("");
  const [saving, setSaving] = useState(false);
  const schoolLabelId = useId();
  const { success: toastSuccess } = useToast();
  const resetTargetSchool = useCallback(() => setTargetSchoolId(""), []);
  const { status, statusLoading, statusError, refreshStatus } =
    useDeviceTransferStatus(device, resetTargetSchool);
  const transferErrors = useApiFormError();
  const { show: showTransferError, clear: clearTransferError } = transferErrors;

  useEffect(() => {
    clearTransferError();
  }, [device, clearTransferError]);

  const destinations = useMemo(
    () => getTransferDestinations(device, schools),
    [device, schools],
  );

  const handleTransfer = useCallback(async () => {
    if (!device || !targetSchoolId || !status?.canTransfer) return;
    setSaving(true);
    clearTransferError();
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
      logger.error("device_transfer_failed", {
        device_id: device.id,
        error:
          transferError instanceof Error
            ? transferError.message
            : String(transferError),
      });
      void showTransferError(transferError, {
        object: "die Übertragung des Geräts",
      });
      // The device may have come online or opened a session meanwhile.
      await refreshStatus();
    } finally {
      setSaving(false);
    }
  }, [
    device,
    onClose,
    onTransferred,
    refreshStatus,
    showTransferError,
    clearTransferError,
    status?.canTransfer,
    targetSchoolId,
    toastSuccess,
  ]);

  return (
    <FormModal
      isOpen={device != null}
      onClose={onClose}
      title="Gerät verschieben"
      size="md"
      error={transferErrors.error}
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
        <TransferDeviceContent
          device={device}
          destinations={destinations}
          targetSchoolId={targetSchoolId}
          status={status}
          statusLoading={statusLoading}
          statusError={statusError}
          schoolLabelId={schoolLabelId}
          onTargetSchoolChange={setTargetSchoolId}
          onRefreshStatus={refreshStatus}
        />
      ) : null}
    </FormModal>
  );
}
