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
import {
  SortableHeader,
  useSort,
  type SortState,
} from "~/app/operator/provisioning/provisioning-tables-shared";

const logger = createLogger({ component: "DevicesTable" });

type DeviceSortKey =
  | "schoolName"
  | "deviceId"
  | "deviceType"
  | "name"
  | "status"
  | "lastSeen"
  | "apiKey";

function sortDevices(
  devices: readonly OperatorDevice[],
  sort: SortState<DeviceSortKey>,
): OperatorDevice[] {
  const dir = sort.direction === "asc" ? 1 : -1;
  return [...devices].sort((a, b) => {
    let av: string;
    let bv: string;
    switch (sort.key) {
      case "schoolName":
        av = a.schoolName.toLowerCase();
        bv = b.schoolName.toLowerCase();
        break;
      case "deviceId":
        av = a.deviceId.toLowerCase();
        bv = b.deviceId.toLowerCase();
        break;
      case "deviceType":
        av = a.deviceType.toLowerCase();
        bv = b.deviceType.toLowerCase();
        break;
      case "name":
        av = (a.name || "").toLowerCase();
        bv = (b.name || "").toLowerCase();
        break;
      case "status":
        av = a.status.toLowerCase();
        bv = b.status.toLowerCase();
        break;
      case "lastSeen":
        av = a.lastSeen ?? "";
        bv = b.lastSeen ?? "";
        break;
      case "apiKey":
        av = a.maskedApiKey.toLowerCase();
        bv = b.maskedApiKey.toLowerCase();
        break;
    }
    return av < bv ? -dir : av > bv ? dir : 0;
  });
}

function DeviceStatusBadge({
  status,
  isOnline,
}: Readonly<{ status: string; isOnline: boolean }>) {
  if (isOnline) {
    return (
      <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-700">
        <span className="h-1.5 w-1.5 rounded-full bg-green-500" />
        Online
      </span>
    );
  }
  const styles: Record<string, string> = {
    active: "bg-green-100 text-green-700",
    inactive: "bg-gray-100 text-gray-500",
    maintenance: "bg-yellow-100 text-yellow-700",
    offline: "bg-red-100 text-red-700",
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
      className="text-green-600"
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
  const { sort, toggle } = useSort<DeviceSortKey>(
    showSchool ? "schoolName" : "deviceId",
  );
  const sorted = useMemo(() => sortDevices(devices, sort), [devices, sort]);
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

  return (
    <div className="overflow-x-auto rounded-2xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md">
      <table className="w-full text-left text-sm">
        <thead>
          <tr className="border-b border-gray-100 text-xs font-medium text-gray-500">
            {showSchool && (
              <SortableHeader
                label="Schule"
                sortKey="schoolName"
                sort={sort}
                onToggle={toggle}
              />
            )}
            <SortableHeader
              label="Geräte-ID"
              sortKey="deviceId"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Name"
              sortKey="name"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Typ"
              sortKey="deviceType"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Status"
              sortKey="status"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="Zuletzt online"
              sortKey="lastSeen"
              sort={sort}
              onToggle={toggle}
            />
            <SortableHeader
              label="API-Key"
              sortKey="apiKey"
              sort={sort}
              onToggle={toggle}
            />
            {(onSetKey || onDelete) && (
              <th className="px-5 py-3 text-xs font-medium text-gray-500">
                Aktionen
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {sorted.map((device) => (
            <tr
              key={device.id}
              className="border-b border-gray-50 last:border-0"
            >
              {showSchool && (
                <td className="px-5 py-3 text-gray-600">{device.schoolName}</td>
              )}
              <td className="px-5 py-3 font-mono text-xs font-medium text-gray-900">
                {device.deviceId}
              </td>
              <td className="px-5 py-3 text-gray-600">{device.name || "—"}</td>
              <td className="px-5 py-3">
                <span className="inline-flex rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
                  {getDeviceTypeDisplayName(device.deviceType)}
                </span>
              </td>
              <td className="px-5 py-3">
                <DeviceStatusBadge
                  status={device.status}
                  isOnline={device.isOnline}
                />
              </td>
              <td className="px-5 py-3 text-gray-600">
                {device.lastSeen ? (
                  <span title={formatLastSeen(device.lastSeen)}>
                    {getRelativeTime(device.lastSeen)}
                  </span>
                ) : (
                  <span className="text-gray-400">Nie</span>
                )}
              </td>
              <td className="px-5 py-3">
                {device.maskedApiKey ? (
                  <button
                    type="button"
                    onClick={() => void handleCopyApiKey(device)}
                    className="group flex items-center gap-1.5 font-mono text-xs text-gray-500 transition-colors hover:text-gray-900"
                    title="API-Key kopieren"
                  >
                    <span>{device.maskedApiKey}</span>
                    <span className="text-gray-300 transition-colors group-hover:text-gray-600">
                      {copiedId === device.id ? <CheckIcon /> : <CopyIcon />}
                    </span>
                  </button>
                ) : (
                  <span className="text-gray-400">—</span>
                )}
              </td>
              {(onSetKey || onDelete) && (
                <td className="px-5 py-3">
                  <div className="flex items-center gap-2">
                    {onSetKey && (
                      <button
                        type="button"
                        onClick={() => onSetKey(device)}
                        className="rounded-lg border border-gray-200 px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900"
                        title="API-Key ändern"
                      >
                        Key ändern
                      </button>
                    )}
                    {onDelete && (
                      <button
                        type="button"
                        onClick={() => onDelete(device)}
                        className="rounded-lg border border-red-200 px-2 py-1 text-xs font-medium text-red-600 transition-colors hover:bg-red-50 hover:text-red-700"
                        title="Gerät löschen"
                      >
                        Löschen
                      </button>
                    )}
                  </div>
                </td>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
