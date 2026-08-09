"use client";

import { Suspense, useCallback, useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseEmptyState } from "~/components/database/database-empty-state";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import {
  useGroupedItems,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { devicesConfig } from "@/components/database/configs/devices.config";
import { getDeviceTypeDisplayName, type Device } from "@/lib/iot-helpers";
import { DevicesMasterDetail } from "@/components/devices/devices-master-detail";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/components/ui/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { NfcModeGuard } from "~/components/tenant/nfc-mode-guard";

const logger = createLogger({ component: "DatabaseDevicesPage" });

type DevicesGroupingMode = "none" | "type" | "room";

const DEVICES_GROUPING_DEFAULT: DevicesGroupingMode = "type";

const DEVICES_GROUPING_OPTIONS: {
  value: DevicesGroupingMode;
  label: string;
}[] = [
  { value: "type", label: "Typ" },
  { value: "room", label: "Raum" },
  { value: "none", label: "Keine" },
];

function parseDevicesGrouping(value: string | null): DevicesGroupingMode {
  if (value === "room" || value === "none") return value;
  return DEVICES_GROUPING_DEFAULT;
}

export default function DevicesPage() {
  return (
    <NfcModeGuard>
      <Suspense fallback={null}>
        <DevicesPageContent />
      </Suspense>
    </NfcModeGuard>
  );
}

function DevicesPageContent() {
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("device");
  const grouping = parseDevicesGrouping(searchParams.get("groupBy"));
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [savingDevice, setSavingDevice] = useState(false);
  // The list response never carries `api_key` (it's a one-time create-only
  // secret). We snapshot the freshly-created device here so the detail panel
  // can render the key until the user navigates away — same dismiss semantics
  // as the pre-master-detail modal had when it closed.
  const [createdDevice, setCreatedDevice] = useState<Device | null>(null);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
    confirmDelete,
  } = useDeleteConfirmation();

  const { success: toastSuccess, error: toastError } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(devicesConfig), []);
  const tenantMutate = useTenantMutate();

  const {
    data: devicesData,
    isLoading: loading,
    error: devicesError,
  } = useSWRAuth("database-devices-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = devicesError
    ? "Fehler beim Laden der Geräte. Bitte versuchen Sie es später erneut."
    : null;

  // Snapshot lifecycle is managed synchronously in the click/edit/delete
  // handlers below — never via an effect on `selectedId`. router.replace is
  // async and useSearchParams lags one render behind it, so any effect that
  // compares createdDevice.id to selectedId would race the URL update and
  // wipe the api_key before it could render.

  const filters: FilterConfig[] = useMemo(() => [], []);

  const activeFilters: ActiveFilter[] = useMemo(
    () =>
      searchTerm
        ? [
            {
              id: "search",
              label: `"${searchTerm}"`,
              onRemove: () => setSearchTerm(""),
            },
          ]
        : [],
    [searchTerm],
  );

  const allDevices = useMemo(() => devicesData ?? [], [devicesData]);

  const filteredDevices = useMemo(() => {
    let arr = [...allDevices];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (d) =>
          (d.name?.toLowerCase().includes(q) ?? false) ||
          d.device_id.toLowerCase().includes(q) ||
          d.device_type.toLowerCase().includes(q),
      );
    }
    arr.sort((a, b) =>
      (a.name ?? a.device_id).localeCompare(b.name ?? b.device_id, "de"),
    );
    return arr;
  }, [allDevices, searchTerm]);

  // Detail lookup uses the unfiltered list so the panel survives a search
  // narrowing the visible rows. The snapshot wins for the just-created device
  // because it's the only place the api_key lives.
  const selectedDevice = useMemo(() => {
    if (!selectedId) return null;
    const fromList = allDevices.find((d) => d.id === selectedId) ?? null;
    if (createdDevice && createdDevice.id === selectedId) {
      return fromList
        ? { ...fromList, api_key: createdDevice.api_key }
        : createdDevice;
    }
    return fromList;
  }, [allDevices, createdDevice, selectedId]);

  const handleSelectDevice = useCallback(
    (id: string | null) => {
      setCreatedDevice((current) =>
        current && current.id !== id ? null : current,
      );
      updateUrlParams({ device: id });
    },
    [updateUrlParams],
  );

  const handleGroupingChange = useCallback(
    (next: DevicesGroupingMode) => {
      updateUrlParams({
        groupBy: next === DEVICES_GROUPING_DEFAULT ? null : next,
      });
    },
    [updateUrlParams],
  );

  const groupers = useMemo<
    Partial<Record<DevicesGroupingMode, Grouper<Device>>>
  >(
    () => ({
      type: (device) => {
        const id = device.device_type || "__no_type__";
        const title = device.device_type
          ? getDeviceTypeDisplayName(device.device_type)
          : "Ohne Typ";
        return { id, title };
      },
      room: (device) => {
        const id = device.room_name?.trim() || "__no_room__";
        const title = device.room_name?.trim() || "Ohne Raum";
        return { id, title };
      },
    }),
    [],
  );

  const groupDefinitions = useGroupedItems(
    filteredDevices,
    grouping,
    groupers,
    "Geräte",
  );

  const handleCloseCreateModal = useCallback(() => {
    setShowCreateModal(false);
  }, []);

  const handleEditClick = useCallback(() => setShowEditModal(true), []);
  const handleCloseEditModal = useCallback(() => setShowEditModal(false), []);

  const handleCreateDevice = useCallback(
    async (data: Partial<Device>) => {
      try {
        const payload = devicesConfig.form.transformBeforeSubmit
          ? devicesConfig.form.transformBeforeSubmit(data)
          : data;
        const created = await service.create(payload);
        toastSuccess(
          getDbOperationMessage(
            "create",
            devicesConfig.name.singular,
            created.name ?? created.device_id,
          ),
        );
        setShowCreateModal(false);
        setCreatedDevice(created);
        handleSelectDevice(created.id);
        await tenantMutate("database-devices-list");
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : String(err);
        logger.error("device_create_failed", { error: errorMessage });
        if (
          errorMessage.includes("ist bereits vergeben") ||
          errorMessage.includes("409")
        ) {
          throw new Error(
            `Die Geräte-ID "${data.device_id ?? ""}" ist bereits vergeben. Bitte wählen Sie eine andere ID.`,
            { cause: err },
          );
        }
        throw new Error(
          "Fehler beim Erstellen des Geräts. Bitte versuchen Sie es erneut.",
          { cause: err },
        );
      }
    },
    [service, handleSelectDevice, tenantMutate, toastSuccess],
  );

  const handleUpdateDevice = useCallback(
    async (data: Partial<Device>) => {
      if (!selectedDevice) return;
      try {
        setSavingDevice(true);
        const payload = devicesConfig.form.transformBeforeSubmit
          ? devicesConfig.form.transformBeforeSubmit(data)
          : data;
        const updatedDevice = await service.update(selectedDevice.id, payload);
        // Editing closes the api_key flash — the snapshot would otherwise
        // overlay the freshly-edited list values on the next render.
        setCreatedDevice(null);
        setShowEditModal(false);
        toastSuccess(
          getDbOperationMessage(
            "update",
            devicesConfig.name.singular,
            updatedDevice.name ?? updatedDevice.device_id,
          ),
        );
        await tenantMutate("database-devices-list");
      } catch (err) {
        const errorMessage = err instanceof Error ? err.message : String(err);
        logger.error("failed to update device", {
          device_id: selectedDevice.id,
          error: errorMessage,
        });
        if (
          errorMessage.includes("ist bereits vergeben") ||
          errorMessage.includes("409")
        ) {
          throw new Error(
            `Die Geräte-ID "${data.device_id ?? ""}" ist bereits vergeben. Bitte wählen Sie eine andere ID.`,
            { cause: err },
          );
        }
        throw err;
      } finally {
        setSavingDevice(false);
      }
    },
    [selectedDevice, service, tenantMutate, toastSuccess],
  );

  const handleDeleteDevice = useCallback(async () => {
    if (!selectedDevice) return;
    const deleteError = await service.delete(selectedDevice.id);
    if (deleteError) {
      toastError(deleteError);
      return;
    }
    toastSuccess(
      getDbOperationMessage(
        "delete",
        devicesConfig.name.singular,
        selectedDevice.name ?? selectedDevice.device_id,
      ),
    );
    setCreatedDevice(null);
    handleSelectDevice(null);
    await tenantMutate("database-devices-list");
  }, [
    selectedDevice,
    service,
    toastError,
    toastSuccess,
    handleSelectDevice,
    tenantMutate,
  ]);

  const canShowDetail =
    !loading && (filteredDevices.length > 0 || selectedDevice !== null);

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      className="-mt-1.5 flex w-full flex-col"
    >
      <div className="mb-4">
        <PageHeaderWithSearch
          title={isMobile ? "Geräte" : ""}
          badge={{
            icon: (
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.devices.icon}
                tone={MOTO_CONCEPTS.devices.tone}
                size={20}
              />
            ),
            count: filteredDevices.length,
            label: "Geräte",
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Geräte suchen...",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
          actionButton={
            <div className="flex items-center gap-2">
              {!isMobile ? (
                <DatabaseGroupingToggle
                  value={grouping}
                  options={DEVICES_GROUPING_OPTIONS}
                  onChange={handleGroupingChange}
                />
              ) : null}
              <DatabaseCreateAction
                label="Gerät"
                ariaLabel="Gerät registrieren"
                onClick={() => setShowCreateModal(true)}
              />
            </div>
          }
        />
      </div>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <DevicesMasterDetail
            groupDefinitions={groupDefinitions}
            selectedId={selectedId}
            selectedDevice={selectedDevice}
            onSelect={handleSelectDevice}
            onEditClick={handleEditClick}
            onDeleteClick={handleDeleteClick}
          />
        </div>
      ) : !loading ? (
        <DatabaseEmptyState
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.devices.icon}
              tone={MOTO_CONCEPTS.devices.tone}
              size={48}
              className="mx-auto"
            />
          }
          title={
            searchTerm ? "Keine Geräte gefunden" : "Keine Geräte vorhanden"
          }
          description={
            searchTerm
              ? "Versuchen Sie einen anderen Suchbegriff."
              : "Es wurden noch keine Geräte registriert."
          }
        />
      ) : null}

      <DatabaseFormModal<Device>
        isOpen={showCreateModal}
        onClose={handleCloseCreateModal}
        mode="create"
        config={devicesConfig}
        onSubmit={handleCreateDevice}
      />

      {selectedDevice && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => confirmDelete(() => void handleDeleteDevice())}
          title="Gerät löschen?"
          confirmText="Löschen"
          cancelText="Abbrechen"
          confirmButtonClass="bg-red-600 hover:bg-red-700"
        >
          <p className="text-sm text-gray-700">
            Möchten Sie das Gerät{" "}
            <span className="font-medium">
              {selectedDevice.name ?? selectedDevice.device_id}
            </span>{" "}
            wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
          </p>
        </ConfirmationModal>
      )}

      {selectedDevice && (
        <DatabaseFormModal<Device>
          isOpen={showEditModal}
          onClose={handleCloseEditModal}
          mode="edit"
          config={devicesConfig}
          initialData={selectedDevice}
          onSubmit={handleUpdateDevice}
          isLoading={savingDevice}
        />
      )}
    </DatabasePageLayout>
  );
}
