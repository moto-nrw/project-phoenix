"use client";

import { useMemo } from "react";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import {
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  formatLastSeen,
} from "~/lib/iot-helpers";
import { DataTable } from "~/components/ui/data-table";
import type { DataTableColumn } from "~/components/ui/data-table";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { ArrowRightLeft, KeyRound, Trash2 } from "lucide-react";

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

interface DevicesTableProps {
  devices: OperatorDevice[];
  showSchool?: boolean;
  isLoading?: boolean;
  onSetKey?: (device: OperatorDevice) => void;
  onTransfer?: (device: OperatorDevice) => void;
  onDelete?: (device: OperatorDevice) => void;
}

export function DevicesTable({
  devices,
  showSchool = false,
  isLoading = false,
  onSetKey,
  onTransfer,
  onDelete,
}: Readonly<DevicesTableProps>) {
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
            <span className="font-mono text-xs text-gray-500">
              {row.maskedApiKey}
            </span>
          ) : (
            <span className="text-gray-400">—</span>
          ),
        sortValue: (row) => row.maskedApiKey.toLowerCase(),
      },
    );

    if (onSetKey || onTransfer || onDelete) {
      cols.push({
        key: "actions",
        header: "Aktionen",
        render: (row) => {
          const items: OverflowMenuEntry[] = [];
          if (onTransfer) {
            items.push({
              label: "Schule wechseln",
              icon: <ArrowRightLeft className="h-4 w-4" aria-hidden="true" />,
              onClick: () => onTransfer(row),
            });
          }
          if (onSetKey) {
            items.push({
              label: "API-Key ändern",
              icon: <KeyRound className="h-4 w-4" aria-hidden="true" />,
              onClick: () => onSetKey(row),
            });
          }
          if (onDelete) {
            if (items.length > 0) items.push({ kind: "separator" });
            items.push({
              label: "Löschen",
              icon: <Trash2 className="h-4 w-4" aria-hidden="true" />,
              destructive: true,
              onClick: () => onDelete(row),
            });
          }
          return (
            <OverflowMenu
              items={items}
              triggerSize="sm"
              ariaLabel={`Aktionen für ${row.name || row.deviceId}`}
            />
          );
        },
      });
    }

    return cols;
  }, [showSchool, onSetKey, onTransfer, onDelete]);

  return (
    <DataTable
      columns={columns}
      rows={devices}
      getRowKey={(row) => row.id}
      defaultSortKey={showSchool ? "schoolName" : "deviceId"}
      isLoading={isLoading}
    />
  );
}
