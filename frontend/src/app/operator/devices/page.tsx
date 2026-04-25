"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { getRelativeTime } from "~/lib/format-utils";
import {
  getDeviceTypeDisplayName,
  getDeviceStatusDisplayName,
  formatLastSeen,
} from "~/lib/iot-helpers";
import { createLogger } from "~/lib/logger";
import {
  CardSkeletons,
  PlusIcon,
  SimpleEmptyState,
} from "../provisioning/provisioning-shared";
import { CreateDeviceModal } from "../provisioning/create-device-modal";
import { SetApiKeyModal } from "../provisioning/set-api-key-modal";
import {
  OrgSchoolFilter,
  SortableHeader,
  useSort,
  type SortState,
} from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";

const logger = createLogger({ component: "OperatorDevicesPage" });

const DEVICE_SWR_PREFIXES = [
  "operator-all-devices",
  "operator-school-devices-",
  "operator-org-devices-",
];

function OperatorDevicesPageContent() {
  useSetBreadcrumb({ pageTitle: "Geräte" });

  const {
    isAuthenticated,
    schools,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
  } = useOrgSchoolFilter("/operator/devices");

  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [deleteDevice, setDeleteDeviceRaw] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteConfirmed, setDeleteConfirmed] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const setDeleteDevice = useCallback((device: OperatorDevice | null) => {
    setDeleteDeviceRaw(device);
    setDeleteConfirmed(false);
    setDeleteError("");
  }, []);

  const { mutate: globalMutate } = useSWRConfig();

  const { data: schoolDevices, isLoading: schoolDevicesLoading } = useSWR(
    isAuthenticated && selectedSchool
      ? `operator-school-devices-${selectedSchool.id}`
      : null,
    () => operatorProvisioningService.listSchoolDevices(selectedSchool!.id),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: orgDevices, isLoading: orgDevicesLoading } = useSWR(
    isAuthenticated && filterOrgId && !selectedSchool
      ? `operator-org-devices-${filterOrgId}`
      : null,
    () => operatorProvisioningService.listOrganizationDevices(filterOrgId),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const { data: allDevices, isLoading: allDevicesLoading } = useSWR(
    isAuthenticated && !filterOrgId && !selectedSchool
      ? "operator-all-devices"
      : null,
    () => operatorProvisioningService.listAllDevices(),
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const devicesLoading = selectedSchool
    ? schoolDevicesLoading
    : filterOrgId
      ? orgDevicesLoading
      : allDevicesLoading;

  const refreshDevices = useCallback(() => {
    return globalMutate(
      (key: unknown) =>
        typeof key === "string" &&
        DEVICE_SWR_PREFIXES.some((p) => key.startsWith(p)),
    );
  }, [globalMutate]);

  const handleDeleteDevice = useCallback(async () => {
    if (!deleteDevice) return;
    setDeleteLoading(true);
    setDeleteError("");
    try {
      await operatorProvisioningService.deleteDevice(deleteDevice.id);
      setDeleteDevice(null);
      setDeleteConfirmed(false);
      void refreshDevices();
    } catch (err) {
      logger.error("device_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setDeleteError(
        err instanceof Error ? err.message : "Fehler beim Löschen des Geräts",
      );
    } finally {
      setDeleteLoading(false);
    }
  }, [deleteDevice, refreshDevices, setDeleteDevice]);

  const tabs = useMemo(
    () => ({
      items: [
        {
          id: "devices",
          label: "Geräte",
          count: selectedSchool
            ? schoolDevices?.length
            : filterOrgId
              ? orgDevices?.length
              : allDevices?.length,
        },
      ],
      activeTab: "devices",
      onTabChange: () => undefined,
    }),
    [
      selectedSchool,
      filterOrgId,
      schoolDevices?.length,
      orgDevices?.length,
      allDevices?.length,
    ],
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Geräte" tabs={tabs} />

      <div className="mb-4 flex items-center justify-between">
        <div />
        <button
          type="button"
          onClick={() => setCreateDeviceOpen(true)}
          className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-gray-800"
        >
          <PlusIcon />
          Neues Gerät
        </button>
      </div>

      <OrgSchoolFilter
        idPrefix="device"
        organizations={activeOrganizations}
        filteredSchools={filteredSchools}
        filterOrgId={filterOrgId}
        selectedSchool={selectedSchool}
        onOrgChange={handleOrgFilterChange}
        onSchoolChange={handleSchoolFilterChange}
      />

      {devicesLoading && <CardSkeletons />}

      {!selectedSchool && filterOrgId && !orgDevicesLoading && (
        <>
          {orgDevices?.length === 0 && (
            <SimpleEmptyState
              title="Keine Geräte"
              description="Für diesen Träger gibt es noch keine registrierten Geräte."
            />
          )}
          {orgDevices && orgDevices.length > 0 && (
            <DevicesTable
              devices={orgDevices}
              showSchool
              onSetKey={setSetKeyDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      )}

      {!selectedSchool && !filterOrgId && !allDevicesLoading && (
        <>
          {allDevices?.length === 0 && (
            <SimpleEmptyState
              title="Keine Geräte"
              description="Es gibt noch keine registrierten Geräte im System."
            />
          )}
          {allDevices && allDevices.length > 0 && (
            <DevicesTable
              devices={allDevices}
              showSchool
              onSetKey={setSetKeyDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      )}

      {selectedSchool && !schoolDevicesLoading && (
        <>
          <div className="mb-3 flex items-center gap-2">
            <h3 className="text-sm font-semibold text-gray-900">
              {selectedSchool.name}
            </h3>
            {selectedSchool.organization && (
              <span className="text-xs text-gray-400">
                {selectedSchool.organization.name}
              </span>
            )}
            {schoolDevices && (
              <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-500">
                {schoolDevices.length}{" "}
                {schoolDevices.length === 1 ? "Gerät" : "Geräte"}
              </span>
            )}
          </div>
          {schoolDevices?.length === 0 && (
            <SimpleEmptyState
              title="Keine Geräte"
              description="Für diese Schule gibt es noch keine registrierten Geräte."
            />
          )}
          {schoolDevices && schoolDevices.length > 0 && (
            <DevicesTable
              devices={schoolDevices}
              onSetKey={setSetKeyDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      )}

      <CreateDeviceModal
        isOpen={createDeviceOpen}
        onClose={() => setCreateDeviceOpen(false)}
        schools={schools}
        onCreated={() => {
          void refreshDevices();
        }}
      />
      <SetApiKeyModal
        isOpen={setKeyDevice !== null}
        onClose={() => setSetKeyDevice(null)}
        device={setKeyDevice}
        onKeySet={() => {
          void refreshDevices();
        }}
      />

      {deleteDevice && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
          <div className="mx-4 w-full max-w-md rounded-2xl bg-white p-6 shadow-xl">
            <h3 className="text-lg font-semibold text-gray-900">
              Gerät löschen
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              Möchten Sie das Gerät{" "}
              <span className="font-mono font-medium">
                {deleteDevice.deviceId}
              </span>
              {deleteDevice.name && ` (${deleteDevice.name})`} von{" "}
              <span className="font-medium">{deleteDevice.schoolName}</span>{" "}
              wirklich löschen?
            </p>
            <p className="mt-2 text-sm font-medium text-red-600">
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>

            {deleteError && (
              <div className="mt-3 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">
                {deleteError}
              </div>
            )}

            {!deleteConfirmed ? (
              <div className="mt-5 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setDeleteDevice(null)}
                  className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50"
                >
                  Abbrechen
                </button>
                <button
                  type="button"
                  onClick={() => setDeleteConfirmed(true)}
                  className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700"
                >
                  Ja, löschen
                </button>
              </div>
            ) : (
              <div className="mt-5 flex justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setDeleteDevice(null)}
                  disabled={deleteLoading}
                  className="rounded-lg border border-gray-200 px-4 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
                >
                  Abbrechen
                </button>
                <button
                  type="button"
                  onClick={() => void handleDeleteDevice()}
                  disabled={deleteLoading}
                  className="rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-red-700 disabled:opacity-50"
                >
                  {deleteLoading ? "Wird gelöscht..." : "Endgültig löschen"}
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default function OperatorDevicesPage() {
  return (
    <Suspense fallback={<div className="-mt-1.5 w-full" />}>
      <OperatorDevicesPageContent />
    </Suspense>
  );
}

// --- Device table helpers (tab-local) ---

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

function DevicesTable({
  devices,
  showSchool = false,
  onSetKey,
  onDelete,
}: {
  readonly devices: OperatorDevice[];
  readonly showSchool?: boolean;
  readonly onSetKey?: (device: OperatorDevice) => void;
  readonly onDelete?: (device: OperatorDevice) => void;
}) {
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

function DeviceStatusBadge({
  status,
  isOnline,
}: {
  readonly status: string;
  readonly isOnline: boolean;
}) {
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
