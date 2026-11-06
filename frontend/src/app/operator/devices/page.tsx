"use client";

import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";

import { Suspense, useCallback, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { Button } from "~/components/ui/button";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { SimpleEmptyState } from "../provisioning/provisioning-shared";
import { CreateDeviceModal } from "../provisioning/create-device-modal";
import { SetApiKeyModal } from "../provisioning/set-api-key-modal";
import { OrgSchoolFilter } from "../provisioning/provisioning-tables-shared";
import { useOrgSchoolFilter } from "../provisioning/use-org-school-filter";
import { DevicesTable } from "~/components/operator/devices-table";
import { DeleteDeviceModal } from "~/components/operator/delete-device-modal";
import { TransferDeviceModal } from "~/components/operator/transfer-device-modal";
import { StatusBadge } from "~/components/ui/status-badge";

function OperatorDevicesPageContent() {
  useSetBreadcrumb({ pageTitle: "Geräte" });

  const {
    isAuthenticated,
    activeSchools,
    activeOrganizations,
    filterOrgId,
    selectedSchool,
    urlSchoolId,
    filteredSchools,
    handleOrgFilterChange,
    handleSchoolFilterChange,
    organizationsLoadError,
    schoolsLoadError,
  } = useOrgSchoolFilter("/operator/devices");

  const [createDeviceOpen, setCreateDeviceOpen] = useState(false);
  const [setKeyDevice, setSetKeyDevice] = useState<OperatorDevice | null>(null);
  const [transferDevice, setTransferDevice] = useState<OperatorDevice | null>(
    null,
  );
  const [deleteDevice, setDeleteDevice] = useState<OperatorDevice | null>(null);

  const refreshFamily = useTenantMutateMatching(
    operatorProvisioningKeys.deviceFamilies,
  );

  const {
    data: schoolDevices,
    error: schoolDevicesError,
    isLoading: schoolDevicesLoading,
  } = useSWRAuth(
    isAuthenticated && urlSchoolId
      ? operatorProvisioningKeys.schoolDevices(urlSchoolId)
      : null,
    () => operatorProvisioningService.listSchoolDevices(urlSchoolId),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: orgDevices,
    error: orgDevicesError,
    isLoading: orgDevicesLoading,
  } = useSWRAuth(
    isAuthenticated && filterOrgId && !urlSchoolId
      ? operatorProvisioningKeys.orgDevices(filterOrgId)
      : null,
    () => operatorProvisioningService.listOrganizationDevices(filterOrgId),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const {
    data: allDevices,
    error: allDevicesError,
    isLoading: allDevicesLoading,
  } = useSWRAuth(
    isAuthenticated && !filterOrgId && !urlSchoolId
      ? operatorProvisioningKeys.allDevices()
      : null,
    () => operatorProvisioningService.listAllDevices(),
    {
      keepPreviousData: false,
      revalidateOnFocus: false,
      dedupingInterval: 5000,
    },
  );

  const refreshDevices = useCallback(() => refreshFamily(), [refreshFamily]);

  // Only one of the three lists is active at a time.
  const devicesLoadError = useSwrLoadError(
    urlSchoolId
      ? schoolDevicesError
      : filterOrgId
        ? orgDevicesError
        : allDevicesError,
    "die Liste der Geräte",
    () => void refreshDevices(),
  );

  const count = selectedSchool
    ? schoolDevices?.length
    : filterOrgId
      ? orgDevices?.length
      : allDevices?.length;

  const actionButton = (
    <Button
      type="button"
      variant="primary"
      size="md"
      onClick={() => setCreateDeviceOpen(true)}
    >
      Neues Gerät
    </Button>
  );

  return (
    <TenantPage
      title="Geräte"
      stats={
        count === undefined ? undefined : (
          <TenantPageStats
            items={[{ value: count, label: count === 1 ? "Gerät" : "Geräte" }]}
          />
        )
      }
      actions={actionButton}
      searchSlot={
        <OrgSchoolFilter
          idPrefix="device"
          organizations={activeOrganizations}
          filteredSchools={filteredSchools}
          filterOrgId={filterOrgId}
          selectedSchool={selectedSchool}
          onOrgChange={handleOrgFilterChange}
          onSchoolChange={handleSchoolFilterChange}
        />
      }
      overlays={
        <>
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
        </>
      }
    >
      <LoadErrorAlert error={organizationsLoadError} />
      <LoadErrorAlert error={schoolsLoadError} />
      <LoadErrorAlert error={devicesLoadError} />

      {!urlSchoolId && filterOrgId && (
        <>
          {!orgDevicesError &&
          !orgDevicesLoading &&
          orgDevices?.length === 0 ? (
            <SimpleEmptyState
              title="Keine Geräte"
              description="Für diesen Träger gibt es noch keine registrierten Geräte."
            />
          ) : (orgDevices === undefined && !orgDevicesLoading) ||
            orgDevicesError ? null : (
            <DevicesTable
              devices={orgDevices ?? []}
              showSchool
              isLoading={orgDevicesLoading}
              onSetKey={setSetKeyDevice}
              onTransfer={setTransferDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      )}

      {!urlSchoolId && !filterOrgId && (
        <>
          {!allDevicesError &&
          !allDevicesLoading &&
          allDevices?.length === 0 ? (
            <SimpleEmptyState
              title="Keine Geräte"
              description="Es gibt noch keine registrierten Geräte im System."
            />
          ) : (allDevices === undefined && !allDevicesLoading) ||
            allDevicesError ? null : (
            <DevicesTable
              devices={allDevices ?? []}
              showSchool
              isLoading={allDevicesLoading}
              onSetKey={setSetKeyDevice}
              onTransfer={setTransferDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      )}

      {selectedSchool && (
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
              <StatusBadge
                tone="gray"
                compact
                label={`${schoolDevices.length} ${
                  schoolDevices.length === 1 ? "Gerät" : "Geräte"
                }`}
              />
            )}
          </div>
          {!schoolDevicesError &&
          !schoolDevicesLoading &&
          schoolDevices?.length === 0 ? (
            <SimpleEmptyState
              title="Keine Geräte"
              description="Für diese Schule gibt es noch keine registrierten Geräte."
            />
          ) : (schoolDevices === undefined && !schoolDevicesLoading) ||
            schoolDevicesError ? null : (
            <DevicesTable
              devices={schoolDevices ?? []}
              isLoading={schoolDevicesLoading}
              onSetKey={setSetKeyDevice}
              onTransfer={setTransferDevice}
              onDelete={setDeleteDevice}
            />
          )}
        </>
      )}
    </TenantPage>
  );
}

export default function OperatorDevicesPage() {
  return (
    <Suspense>
      <OperatorDevicesPageContent />
    </Suspense>
  );
}
