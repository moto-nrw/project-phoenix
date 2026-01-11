"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { DatabasePageLayout } from "~/components/database/database-page-layout";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { devicesConfig } from "@/lib/database/configs/devices.config";
import type { Device } from "@/lib/iot-helpers";
import {
  DeviceCreateModal,
  DeviceDetailModal,
  DeviceEditModal,
} from "@/components/devices";
import { ConfirmationModal } from "~/components/ui/modal";
import { getDeviceTypeDisplayName } from "@/lib/iot-helpers";
import { useToast } from "~/contexts/ToastContext";
import { useIsMobile } from "~/hooks/useIsMobile";

export default function DevicesPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  // No filters on this page (per requirements)
  const isMobile = useIsMobile();

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
  const [showDeleteConfirmModal, setShowDeleteConfirmModal] = useState(false);
  const [selectedDevice, setSelectedDevice] = useState<Device | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const { success: toastSuccess } = useToast();

  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });

  const service = useMemo(() => createCrudService(devicesConfig), []);

  const fetchDevices = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setDevices(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching devices:", err);
      setError(
        "Fehler beim Laden der Geräte. Bitte versuchen Sie es später erneut.",
      );
      setDevices([]);
    } finally {
      setLoading(false);
    }
  }, [service]);

  useEffect(() => {
    fetchDevices().catch(() => {
      // Error already handled in fetchDevices
    });
  }, [fetchDevices]);

  // uniqueTypes removed

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

  const filteredDevices = useMemo(() => {
    let arr = [...devices];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(
        (d) =>
          (d.name?.toLowerCase().includes(q) ?? false) ||
          d.device_id.toLowerCase().includes(q) ||
          d.device_type.toLowerCase().includes(q),
      );
    }
    // No additional filters — only search is applied
    arr.sort((a, b) =>
      (a.name ?? a.device_id).localeCompare(b.name ?? b.device_id, "de"),
    );
    return arr;
  }, [devices, searchTerm]);

  const handleSelectDevice = async (device: Device) => {
    setSelectedDevice(device);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(device.id);
      setSelectedDevice(fresh);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleCreateDevice = async (data: Partial<Device>) => {
    try {
      setCreateLoading(true);
      if (devicesConfig.form.transformBeforeSubmit)
        data = devicesConfig.form.transformBeforeSubmit(data);
      const created = await service.create(data);
      toastSuccess(
        getDbOperationMessage(
          "create",
          devicesConfig.name.singular,
          created.name ?? created.device_id,
        ),
      );
      setShowCreateModal(false);
      // Open detail to show API key if present
      setSelectedDevice(created);
      setShowDetailModal(true);
      await fetchDevices();
    } finally {
      setCreateLoading(false);
    }
  };

  const handleUpdateDevice = async (data: Partial<Device>) => {
    if (!selectedDevice) return;
    try {
      setDetailLoading(true);
      if (devicesConfig.form.transformBeforeSubmit)
        data = devicesConfig.form.transformBeforeSubmit(data);
      await service.update(selectedDevice.id, data);
      toastSuccess(
        getDbOperationMessage(
          "update",
          devicesConfig.name.singular,
          selectedDevice.name ?? selectedDevice.device_id,
        ),
      );
      const refreshed = await service.getOne(selectedDevice.id);
      setSelectedDevice(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchDevices();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleDeleteDevice = async () => {
    if (!selectedDevice) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedDevice.id);
      toastSuccess(
        getDbOperationMessage(
          "delete",
          devicesConfig.name.singular,
          selectedDevice.name ?? selectedDevice.device_id,
        ),
      );
      setShowDetailModal(false);
      setSelectedDevice(null);
      await fetchDevices();
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEditClick = () => {
    setShowDetailModal(false);
    setShowEditModal(true);
  };

  const handleDeleteClick = () => {
    setShowDetailModal(false);
    setShowDeleteConfirmModal(true);
  };

  const handleDeleteCancel = () => {
    setShowDeleteConfirmModal(false);
    setShowDetailModal(true);
  };

  return (
    <DatabasePageLayout loading={loading} sessionLoading={status === "loading"}>
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
            !isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="group relative flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-br from-yellow-500 to-yellow-600 text-white shadow-lg transition-all duration-300 hover:scale-110 hover:shadow-xl active:scale-95"
                style={{
                  background:
                    "linear-gradient(135deg, rgb(234, 179, 8) 0%, rgb(202, 138, 4) 100%)",
                  willChange: "transform, opacity",
                  WebkitTransform: "translateZ(0)",
                  transform: "translateZ(0)",
                }}
                aria-label="Gerät registrieren"
              >
                <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                <svg
                  className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 4.5v15m7.5-7.5h-15"
                  />
                </svg>
                <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
              </button>
            )
          }
        />
      </div>

      <button
        onClick={() => setShowCreateModal(true)}
        className="group pointer-events-auto fixed right-4 bottom-24 z-40 flex h-14 w-14 translate-y-0 items-center justify-center rounded-full bg-gradient-to-br from-yellow-500 to-yellow-600 text-white opacity-100 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-300 ease-out hover:shadow-[0_8px_40px_rgba(234,179,8,0.3)] active:scale-95 md:hidden"
        style={{
          background:
            "linear-gradient(135deg, rgb(234, 179, 8) 0%, rgb(202, 138, 4) 100%)",
          willChange: "transform, opacity",
          WebkitTransform: "translateZ(0)",
          transform: "translateZ(0)",
        }}
        aria-label="Gerät registrieren"
      >
        <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
        <svg
          className="pointer-events-none relative h-6 w-6 transition-transform duration-300 group-active:rotate-90"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          strokeWidth={2.5}
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M12 4.5v15m7.5-7.5h-15"
          />
        </svg>
        <div className="pointer-events-none absolute inset-0 scale-0 rounded-full bg-white/20 opacity-0 transition-transform duration-500 group-hover:scale-100 group-hover:opacity-100"></div>
      </button>

      {error && (
        <div className="mb-6 rounded-lg border border-red-200 bg-red-50 p-4">
          <p className="text-sm text-red-800">{error}</p>
        </div>
      )}

      {filteredDevices.length === 0 ? (
        <div className="flex min-h-[300px] items-center justify-center">
          <div className="text-center">
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
            <h3 className="mt-4 text-lg font-medium text-gray-900">
              {searchTerm ? "Keine Geräte gefunden" : "Keine Geräte vorhanden"}
            </h3>
            <p className="mt-2 text-sm text-gray-600">
              {searchTerm
                ? "Versuchen Sie einen anderen Suchbegriff."
                : "Es wurden noch keine Geräte registriert."}
            </p>
          </div>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredDevices.map((device, index) => {
            const handleClick = () => void handleSelectDevice(device);
            return (
              <button
                type="button"
                key={device.id}
                onClick={handleClick}
                className="group relative w-full cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.99] md:hover:-translate-y-1 md:hover:scale-[1.01] md:hover:border-amber-300/60 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                style={{
                  animationName: "fadeInUp",
                  animationDuration: "0.5s",
                  animationTimingFunction: "ease-out",
                  animationFillMode: "forwards",
                  animationDelay: `${index * 0.03}s`,
                  opacity: 0,
                }}
              >
                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-br from-yellow-50/80 to-yellow-100/80 opacity-[0.03]"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-yellow-300/60"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gradient-to-br from-yellow-500 to-yellow-600 font-semibold text-white shadow-md transition-transform duration-300 md:group-hover:scale-110">
                      {(device.name ?? device.device_id)
                        ?.charAt(0)
                        ?.toUpperCase() ?? "D"}
                    </div>
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 transition-colors duration-300 md:group-hover:text-yellow-600">
                      {device.name ?? device.device_id}
                    </h3>
                    <div className="mt-1 flex flex-wrap items-center gap-2">
                      <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-700">
                        {getDeviceTypeDisplayName(device.device_type)}
                      </span>
                    </div>
                  </div>
                  <div className="flex-shrink-0">
                    <svg
                      className="h-6 w-6 text-gray-400 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-yellow-600"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 5l7 7-7 7"
                      />
                    </svg>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-amber-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
              </button>
            );
          })}
        </div>
      )}

      {/* Create */}
      <DeviceCreateModal
        isOpen={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreateDevice}
        loading={createLoading}
      />

      {/* Detail */}
      {selectedDevice && (
        <DeviceDetailModal
          isOpen={showDetailModal}
          onClose={() => {
            setShowDetailModal(false);
            setSelectedDevice(null);
          }}
          device={selectedDevice}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteDevice()}
          loading={detailLoading}
          onDeleteClick={handleDeleteClick}
        />
      )}

      {/* Delete Confirmation */}
      {selectedDevice && (
        <ConfirmationModal
          isOpen={showDeleteConfirmModal}
          onClose={handleDeleteCancel}
          onConfirm={() => {
            setShowDeleteConfirmModal(false);
            void handleDeleteDevice();
          }}
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

      {/* Edit */}
      {selectedDevice && (
        <DeviceEditModal
          isOpen={showEditModal}
          onClose={() => {
            setShowEditModal(false);
          }}
          device={selectedDevice}
          onSave={handleUpdateDevice}
          loading={detailLoading}
        />
      )}

      {/* Success toasts are handled globally */}
    </DatabasePageLayout>
  );
}
