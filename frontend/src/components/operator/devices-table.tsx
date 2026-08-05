"use client";

import { useCallback, useMemo, useState } from "react";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import {
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  formatLastSeen,
} from "~/lib/iot-helpers";
import { createLogger } from "~/lib/logger";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";

const logger = createLogger({ component: "DevicesTable" });

function DeviceStatusBadge({
  status,
  isOnline,
}: Readonly<{ status: string; isOnline: boolean }>) {
  if (isOnline) {
    return (
      <span className="bg-moto-green/15 text-moto-green-strong inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
        <span className="bg-moto-green h-1.5 w-1.5 rounded-full" />
        Online
      </span>
    );
  }
  const styles: Record<string, string> = {
    active: "bg-moto-green/15 text-moto-green-strong",
    inactive: "bg-gray-100 text-gray-500",
    maintenance: "bg-moto-amber/15 text-moto-amber-strong",
    offline: "bg-moto-red/10 text-moto-red-strong",
  };
  return (
    <span
      className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-gray-100 text-gray-500"}`}
    >
      {getDeviceStatusDisplayName(status)}
    </span>
  );
}

function CopyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
      <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="text-moto-green-strong"
    >
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}

interface DevicesTableProps {
  devices: OperatorDevice[];
  showSchool?: boolean;
  onSetKey?: (device: OperatorDevice) => void;
  onDelete?: (device: OperatorDevice) => void;
}

export function DevicesTable({
  devices,
  showSchool = false,
  onSetKey,
  onDelete,
}: Readonly<DevicesTableProps>) {
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const handleCopyApiKey = useCallback(async (device: OperatorDevice) => {
    if (!device.apiKey) return;
    try {
      await navigator.clipboard.writeText(device.apiKey);
      setCopiedId(device.id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch {
      logger.error("clipboard_copy_failed", {
        error: "Failed to copy API key to clipboard",
      });
    }
  }, []);

  const columns = useMemo<DataTableColumn<OperatorDevice>[]>(() => {
    const cols: DataTableColumn<OperatorDevice>[] = [];

    if (showSchool) {
      cols.push({
        key: "schoolName",
        header: "Schule",
        render: (row) => row.schoolName,
        sortValue: (row) => row.schoolName.toLowerCase(),
        className: "text-gray-600",
      });
    }

    cols.push(
      {
        key: "deviceId",
        header: "Geräte-ID",
        render: (row) => row.deviceId,
        sortValue: (row) => row.deviceId.toLowerCase(),
        className: "font-mono text-xs font-medium text-gray-900",
      },
      {
        key: "name",
        header: "Name",
        render: (row) => row.name || "—",
        sortValue: (row) => (row.name || "").toLowerCase(),
        className: "text-gray-600",
      },
      {
        key: "deviceType",
        header: "Typ",
        render: (row) => (
          <span className="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
            {getDeviceTypeDisplayName(row.deviceType)}
          </span>
        ),
        sortValue: (row) => row.deviceType.toLowerCase(),
      },
      {
        key: "status",
        header: "Status",
        render: (row) => (
          <DeviceStatusBadge status={row.status} isOnline={row.isOnline} />
        ),
        sortValue: (row) => row.status.toLowerCase(),
      },
      {
        key: "lastSeen",
        header: "Zuletzt online",
        render: (row) =>
          row.lastSeen ? (
            <span title={formatLastSeen(row.lastSeen)}>
              {getRelativeTime(row.lastSeen)}
            </span>
          ) : (
            <span className="text-gray-400">Nie</span>
          ),
        sortValue: (row) => row.lastSeen ?? "",
        className: "text-gray-600",
      },
      {
        key: "apiKey",
        header: "API-Key",
        render: (row) =>
          row.maskedApiKey ? (
            <button
              type="button"
              onClick={() => void handleCopyApiKey(row)}
              className="group flex items-center gap-1.5 font-mono text-xs text-gray-500 transition-colors hover:text-gray-900"
              title="API-Key kopieren"
            >
              <span>{row.maskedApiKey}</span>
              <span className="text-gray-300 transition-colors group-hover:text-gray-600">
                {copiedId === row.id ? <CheckIcon /> : <CopyIcon />}
              </span>
            </button>
          ) : (
            <span className="text-gray-400">—</span>
          ),
        sortValue: (row) => row.maskedApiKey.toLowerCase(),
      },
    );

    if (onSetKey || onDelete) {
      cols.push({
        key: "actions",
        header: "Aktionen",
        render: (row) => (
          <div className="flex items-center gap-2">
            {onSetKey && (
              <button
                type="button"
                onClick={() => onSetKey(row)}
                className="rounded-lg border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
                title="API-Key ändern"
              >
                Key ändern
              </button>
            )}
            {onDelete && (
              <button
                type="button"
                onClick={() => onDelete(row)}
                className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red/10 hover:text-moto-red rounded-lg border px-2 py-1 text-xs font-medium transition-colors"
                title="Gerät löschen"
              >
                Löschen
              </button>
            )}
          </div>
        ),
      });
    }

    return cols;
  }, [showSchool, onSetKey, onDelete, copiedId, handleCopyApiKey]);

  return (
    <DataTable
      columns={columns}
      rows={devices}
      getRowKey={(row) => row.id}
      defaultSortKey={showSchool ? "schoolName" : "deviceId"}
    />
  );
}
