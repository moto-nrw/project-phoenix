"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { createLogger } from "~/lib/logger";
import {
  CardSkeletons,
  PlusIcon,
  SimpleEmptyState,
} from "../provisioning/provisioning-shared";
import { CreateDeviceModal } from "../provisioning/create-device-modal";
import { SetApiKeyModal } from "../provisioning/set-api-key-modal";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";
import { DevicesTable } from "~/components/operator/devices-table";

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
    activeSchools,
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

  const actionButton = (
    <button
      type="button"
      onClick={() => setCreateDeviceOpen(true)}
      className="rounded-full bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700"
    >
      Neues Gerät
    </button>
  );

  const mobileActionButton = (
    <button
      type="button"
      onClick={() => setCreateDeviceOpen(true)}
      className="rounded-full bg-gray-900 p-2 text-white transition-colors hover:bg-gray-700"
      aria-label="Neues Gerät"
    >
      <PlusIcon />
    </button>
  );

  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title="Geräte"
        tabs={tabs}
        actionButton={actionButton}
        mobileActionButton={mobileActionButton}
      />

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
        schools={activeSchools}
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
