"use client";

import {
  Suspense,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { redirect, useSearchParams } from "next/navigation";
import { DatabaseCreateAction } from "~/components/database/database-create-action";
import { DatabaseGroupingToggle } from "~/components/database/database-grouping-toggle";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { formatCount } from "~/lib/format-utils";
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
import { createCrudService } from "@/lib/database/service-factory";
import { devicesConfig } from "@/components/database/configs/devices.config";
import { getDeviceTypeDisplayName, type Device } from "@/lib/iot-helpers";
import { DevicesMasterDetail } from "@/components/devices/devices-master-detail";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
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
    <NfcModeGuard title="Geräte">
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

  // The list response never carries `api_key` (it's a one-time create-only
  // secret). We snapshot the freshly-created device here so the detail panel
  // can render the key until the user navigates away — same dismiss semantics
  // as the pre-master-detail modal had when it closed.
  const [createdDevice, setCreatedDevice] = useState<Device | null>(null);

  const {
    showConfirmModal: showDeleteConfirmModal,
    handleDeleteClick,
    handleDeleteCancel,
  } = useDeleteConfirmation();

  const { success: toastSuccess } = useToast();
  // Schreibfehler bleiben im jeweiligen Dialog (#2517).
  const createErrors = useApiFormError();
  const deleteErrors = useApiFormError();
  const [deletePending, setDeletePending] = useState(false);
  const latestDeleteRef = useRef<() => void>(() => undefined);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const { clear: clearCreateErrors } = createErrors;
  // Ein neu geöffnetes Anlegen beginnt ohne den Fehler des vorigen Versuchs.
  useEffect(() => {
    if (showCreateModal) clearCreateErrors();
  }, [showCreateModal, clearCreateErrors]);

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
    isLoading: swrLoading,
    error: devicesError,
    mutate: mutateDevices,
  } = useSWRAuth("database-devices-list", async () => {
    const data = await service.getList({ page: 1, pageSize: 500 });
    return Array.isArray(data.data) ? data.data : [];
  });

  const error = useSwrLoadError(devicesError, "die Liste der Geräte", () =>
    mutateDevices(),
  );
  // Bis der Katalogtext des Ladefehlers da ist, bleibt das Skelett stehen.
  const loading = swrLoading || (Boolean(devicesError) && error === null);

  // Snapshot lifecycle is managed synchronously in the click/edit/delete
  // handlers below — never via an effect on `selectedId`. router.replace is
  // async and useSearchParams lags one render behind it, so any effect that
  // compares createdDevice.id to selectedId would race the URL update and
  // wipe the api_key before it could render.

  // Statuszeile des Seitenkopfs aus der bereits geladenen Geräteliste.
  const statusLine = useMemo(() => {
    const devices = devicesData ?? [];
    const online = devices.filter((d) => d.is_online).length;
    return `${formatCount(devices.length)} ${devices.length === 1 ? "Gerät" : "Geräte"} · ${formatCount(online)} online`;
  }, [devicesData]);

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

  const handleCreateDevice = useCallback(
    async (data: Partial<Device>) => {
      // Ein Fehler bleibt im Dialog: DatabaseForm zeigt ihn über errorPath,
      // eine vergebene Geräte-ID (iot.device_id_taken) markiert das Feld.
      const payload = devicesConfig.form.transformBeforeSubmit
        ? devicesConfig.form.transformBeforeSubmit(data)
        : data;
      const created = await service.create(payload);
      toastSuccess(
        `Das Gerät „${created.name ?? created.device_id}“ ist registriert.`,
      );
      setShowCreateModal(false);
      setCreatedDevice(created);
      handleSelectDevice(created.id);
      await tenantMutate("database-devices-list");
    },
    [service, handleSelectDevice, tenantMutate, toastSuccess],
  );

  const handleUpdateDevice = useCallback(
    async (data: Partial<Device>) => {
      if (!selectedDevice) return;
      // Ein Fehler bleibt im Formular (errorPath im Stammdaten-Reiter).
      const payload = devicesConfig.form.transformBeforeSubmit
        ? devicesConfig.form.transformBeforeSubmit(data)
        : data;
      const updatedDevice = await service.update(selectedDevice.id, payload);
      // Editing closes the api_key flash — the snapshot would otherwise
      // overlay the freshly-edited list values on the next render.
      setCreatedDevice(null);
      toastSuccess(
        `Das Gerät „${updatedDevice.name ?? updatedDevice.device_id}“ ist gespeichert.`,
      );
      await tenantMutate("database-devices-list");
    },
    [selectedDevice, service, tenantMutate, toastSuccess],
  );

  const handleDeleteDevice = useCallback(async () => {
    if (!selectedDevice) return;
    setDeletePending(true);
    deleteErrors.clear();
    try {
      const deleted = await service.remove(selectedDevice.id);
      if (!deleted) return;
      toastSuccess(
        `Das Gerät „${selectedDevice.name ?? selectedDevice.device_id}“ ist gelöscht.`,
      );
      handleDeleteCancel();
      setCreatedDevice(null);
      handleSelectDevice(null);
      await tenantMutate("database-devices-list");
    } catch (err) {
      logger.warn("device_delete_failed", {
        device_id: selectedDevice.id,
        error: err instanceof Error ? err.message : String(err),
      });
      // Der Bestätigungsdialog bleibt offen und nennt den Grund.
      void deleteErrors.show(err, {
        object: "das Löschen des Geräts",
        retry: () => latestDeleteRef.current(),
      });
    } finally {
      setDeletePending(false);
    }
  }, [
    selectedDevice,
    service,
    toastSuccess,
    handleDeleteCancel,
    handleSelectDevice,
    tenantMutate,
    deleteErrors,
  ]);

  useLayoutEffect(() => {
    latestDeleteRef.current = () => void handleDeleteDevice();
  });

  const canShowDetail =
    !loading && (filteredDevices.length > 0 || selectedDevice !== null);

  return (
    <DatabasePageLayout
      loading={loading}
      sessionLoading={status === "loading"}
      error={error}
      empty={
        filteredDevices.length === 0 && selectedDevice === null
          ? {
              title: searchTerm
                ? "Keine Geräte gefunden"
                : "Keine Geräte vorhanden",
              description: searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Registrieren Sie das erste Gerät, um Karten zu lesen.",
              icon: (
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.devices.icon}
                  tone={MOTO_CONCEPTS.devices.tone}
                  size={48}
                />
              ),
              action: searchTerm ? undefined : (
                <DatabaseCreateAction
                  label="Gerät"
                  ariaLabel="Gerät registrieren"
                  showMobileFab={false}
                  onClick={() => setShowCreateModal(true)}
                />
              ),
            }
          : null
      }
      overlays={
        <>
          <DatabaseFormModal<Device>
            isOpen={showCreateModal}
            onClose={handleCloseCreateModal}
            mode="create"
            config={devicesConfig}
            onSubmit={handleCreateDevice}
            errorPath={createErrors}
            errorObject="das Gerät"
          />

          {selectedDevice && (
            <ConfirmDeleteModal
              isOpen={showDeleteConfirmModal}
              onClose={handleDeleteCancel}
              onConfirm={() => void handleDeleteDevice()}
              title="Gerät löschen?"
              description={
                <>
                  Möchten Sie das Gerät{" "}
                  <span className="font-medium">
                    {selectedDevice.name ?? selectedDevice.device_id}
                  </span>{" "}
                  wirklich löschen? Diese Aktion kann nicht rückgängig gemacht
                  werden.
                </>
              }
              gate={{ mode: "twoStep" }}
              loading={deletePending}
              error={deleteErrors.error}
            />
          )}
        </>
      }
      className="flex w-full flex-col"
      intro={{
        title: "Geräte",
        description: statusLine,
        actions: (
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
        ),
      }}
      search={
        <PageHeaderWithSearch
          embedded
          title=""
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
            placeholder: "Geräte suchen…",
          }}
          filters={filters}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
          }}
        />
      }
    >
      {canShowDetail ? (
        <div className="min-h-0 flex-1 pb-4">
          <DevicesMasterDetail
            groupDefinitions={groupDefinitions}
            selectedId={selectedId}
            selectedDevice={selectedDevice}
            onSelect={handleSelectDevice}
            onSaveDevice={handleUpdateDevice}
            onDeleteClick={handleDeleteClick}
          />
        </div>
      ) : null}
    </DatabasePageLayout>
  );
}
