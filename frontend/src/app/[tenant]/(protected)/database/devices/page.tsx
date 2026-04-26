"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { devicesConfig } from "@/lib/database/configs/devices.config";
import { getDeviceTypeDisplayName, type Device } from "@/lib/iot-helpers";
import {
  DeviceCreateModal,
  DeviceEditModal,
  DevicesMasterDetail,
} from "@/components/devices";
import { ConfirmationModal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";
import { useDeleteConfirmation } from "~/hooks/useDeleteConfirmation";
import { useUpdateUrlParams } from "~/hooks/useUpdateUrlParams";
import { createLogger } from "~/lib/logger";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";

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
  const searchParams = useSearchParams();
  const updateUrlParams = useUpdateUrlParams();

  const selectedId = searchParams.get("device");
  const grouping = parseDevicesGrouping(searchParams.get("groupBy"));
  const [searchTerm, setSearchTerm] = useState("");
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  // The list response never carries `api_key` (it's a one-time create-only
  // secret). We snapshot the freshly-created device here so the detail panel
  // can render the key until the user navigates away — same dismiss semantics
  // as the pre-master-detail modal had when it closed.
  const [createdDevice, setCreatedDevice] = useState<Device | null>(null);
  const [savingDevice, setSavingDevice] = useState(false);

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

  // Drop the snapshot as soon as the user navigates away from the created
  // device. Re-selecting it later shows only the list values (no api_key).
  useEffect(() => {
    if (createdDevice && selectedId !== createdDevice.id) {
      setCreatedDevice(null);
    }
  }, [createdDevice, selectedId]);

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
    setCreateError(null);
  }, []);

  const handleEditClick = useCallback(() => setShowEditModal(true), []);
  const handleCloseEditModal = useCallback(() => setShowEditModal(false), []);

  const handleCreateDevice = useCallback(
    async (data: Partial<Device>) => {
      try {
        setCreateLoading(true);
        setCreateError(null);
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
        if (
          errorMessage.includes("duplicate device ID") ||
          errorMessage.includes("409")
        ) {
          setCreateError(
            `Die Geräte-ID "${data.device_id ?? ""}" ist bereits vergeben. Bitte wählen Sie eine andere ID.`,
          );
        } else {
          setCreateError(
            "Fehler beim Erstellen des Geräts. Bitte versuchen Sie es erneut.",
          );
        }
        logger.error("device_create_failed", { error: errorMessage });
      } finally {
        setCreateLoading(false);
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
        logger.error("failed to update device", {
          device_id: selectedDevice.id,
          error: err instanceof Error ? err.message : String(err),
        });
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
              <svg
                className="h-5 w-5 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                />
              </svg>
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
            <svg
              className="mx-auto h-12 w-12 text-gray-400"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={1.5}
                d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
              />
            </svg>
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

      <DeviceCreateModal
        isOpen={showCreateModal}
        onClose={handleCloseCreateModal}
        onCreate={handleCreateDevice}
        loading={createLoading}
        error={createError}
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
        <DeviceEditModal
          isOpen={showEditModal}
          onClose={handleCloseEditModal}
          device={selectedDevice}
          onSave={handleUpdateDevice}
          loading={savingDevice}
        />
      )}
    </DatabasePageLayout>
  );
}
