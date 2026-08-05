"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR, { useSWRConfig } from "swr";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
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
import { DeleteDeviceModal } from "~/components/operator/delete-device-modal";
import { TransferDeviceModal } from "~/components/operator/transfer-device-modal";

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
  const [transferDevice, setTransferDevice] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteDevice, setDeleteDevice] = useState<OperatorDevice | null>(null);

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
              onTransfer={setTransferDevice}
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
              onTransfer={setTransferDevice}
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
              onTransfer={setTransferDevice}
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
      <TransferDeviceModal
        device={transferDevice}
        schools={activeSchools}
        onClose={() => setTransferDevice(null)}
        onTransferred={() => refreshDevices().then(() => undefined)}
      />
      <DeleteDeviceModal
        device={deleteDevice}
        onClose={() => setDeleteDevice(null)}
        onDeleted={() => refreshDevices().then(() => undefined)}
      />
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
