"use client";

import { useEffect, useMemo, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header/types";
import { getDbOperationMessage } from "@/lib/use-notification";
import { createCrudService } from "@/lib/database/service-factory";
import { devicesConfig } from "@/lib/database/configs/devices.config";
import type { Device } from "@/lib/iot-helpers";
import { DeviceCreateModal, DeviceDetailModal, DeviceEditModal } from "@/components/devices";
import { getDeviceTypeDisplayName } from "@/lib/iot-helpers";
import { useToast } from "~/contexts/ToastContext";

import { Loading } from "~/components/ui/loading";
export default function DevicesPage() {
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [devices, setDevices] = useState<Device[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  // No filters on this page (per requirements)
  const [isMobile, setIsMobile] = useState(false);

  const [showCreateModal, setShowCreateModal] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [showDetailModal, setShowDetailModal] = useState(false);
  const [showEditModal, setShowEditModal] = useState(false);
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

  useEffect(() => {
    const checkMobile = () => setIsMobile(window.innerWidth < 768);
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);


  const fetchDevices = useCallback(async () => {
    try {
      setLoading(true);
      const data = await service.getList({ page: 1, pageSize: 500 });
      const arr = Array.isArray(data.data) ? data.data : [];
      setDevices(arr);
      setError(null);
    } catch (err) {
      console.error("Error fetching devices:", err);
      setError("Fehler beim Laden der Geräte. Bitte versuchen Sie es später erneut.");
      setDevices([]);
    } finally { setLoading(false); }
  }, [service]);

  useEffect(() => { void fetchDevices(); }, [fetchDevices]);

  // uniqueTypes removed

  const filters: FilterConfig[] = useMemo(() => [], []);

  const activeFilters: ActiveFilter[] = useMemo(() => (
    searchTerm ? [{ id: 'search', label: `"${searchTerm}"`, onRemove: () => setSearchTerm("") }] : []
  ), [searchTerm]);

  const filteredDevices = useMemo(() => {
    let arr = [...devices];
    if (searchTerm) {
      const q = searchTerm.toLowerCase();
      arr = arr.filter(d =>
        (d.name?.toLowerCase().includes(q) ?? false) ||
        d.device_id.toLowerCase().includes(q) ||
        d.device_type.toLowerCase().includes(q)
      );
    }
    // No additional filters — only search is applied
    arr.sort((a, b) => (a.name ?? a.device_id).localeCompare((b.name ?? b.device_id), 'de'));
    return arr;
  }, [devices, searchTerm]);

  const handleSelectDevice = async (device: Device) => {
    setSelectedDevice(device);
    setShowDetailModal(true);
    try {
      setDetailLoading(true);
      const fresh = await service.getOne(device.id);
      setSelectedDevice(fresh);
    } finally { setDetailLoading(false); }
  };

  const handleCreateDevice = async (data: Partial<Device>) => {
    try {
      setCreateLoading(true);
      if (devicesConfig.form.transformBeforeSubmit) data = devicesConfig.form.transformBeforeSubmit(data);
      const created = await service.create(data);
      toastSuccess(getDbOperationMessage('create', devicesConfig.name.singular, created.name ?? created.device_id));
      setShowCreateModal(false);
      // Open detail to show API key if present
      setSelectedDevice(created);
      setShowDetailModal(true);
      await fetchDevices();
    } finally { setCreateLoading(false); }
  };

  const handleUpdateDevice = async (data: Partial<Device>) => {
    if (!selectedDevice) return;
    try {
      setDetailLoading(true);
      if (devicesConfig.form.transformBeforeSubmit) data = devicesConfig.form.transformBeforeSubmit(data);
      await service.update(selectedDevice.id, data);
      toastSuccess(getDbOperationMessage('update', devicesConfig.name.singular, selectedDevice.name ?? selectedDevice.device_id));
      const refreshed = await service.getOne(selectedDevice.id);
      setSelectedDevice(refreshed);
      setShowEditModal(false);
      setShowDetailModal(true);
      await fetchDevices();
    } finally { setDetailLoading(false); }
  };

  const handleDeleteDevice = async () => {
    if (!selectedDevice) return;
    try {
      setDetailLoading(true);
      await service.delete(selectedDevice.id);
      toastSuccess(getDbOperationMessage('delete', devicesConfig.name.singular, selectedDevice.name ?? selectedDevice.device_id));
      setShowDetailModal(false);
      setSelectedDevice(null);
      await fetchDevices();
    } finally { setDetailLoading(false); }
  };

  const handleEditClick = () => { setShowDetailModal(false); setShowEditModal(true); };

  if (status === "loading" || loading) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  return (
    <ResponsiveLayout>
      <div className="w-full">
        {isMobile && (
          <button onClick={() => (window.location.href = '/database')} className="flex items-center gap-2 text-gray-600 hover:text-gray-900 mb-3 transition-colors duration-200 relative z-10" aria-label="Zurück zur Datenverwaltung">
            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" /></svg>
            <span className="text-sm font-medium">Zurück</span>
          </button>
        )}

        <div className="mb-4">
          <PageHeaderWithSearch
            title={isMobile ? "Geräte" : ""}
            badge={{
              icon: (
                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                </svg>
              ),
              count: filteredDevices.length,
              label: "Geräte"
            }}
            search={{ value: searchTerm, onChange: setSearchTerm, placeholder: "Geräte suchen..." }}
            filters={filters}
            activeFilters={activeFilters}
            onClearAllFilters={() => { setSearchTerm(""); }}
            actionButton={!isMobile && (
              <button
                onClick={() => setShowCreateModal(true)}
                className="relative w-10 h-10 bg-gradient-to-br from-yellow-500 to-yellow-600 text-white rounded-full shadow-lg hover:shadow-xl transition-all duration-300 flex items-center justify-center group hover:scale-110 active:scale-95"
                aria-label="Gerät registrieren"
              >
                <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
                <svg className="relative h-5 w-5 transition-transform duration-300 group-active:rotate-90" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
                <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
              </button>
            )}
          />
        </div>

        <button
          onClick={() => setShowCreateModal(true)}
          className="md:hidden fixed right-4 bottom-24 z-40 w-14 h-14 bg-gradient-to-br from-yellow-500 to-yellow-600 text-white rounded-full shadow-[0_8px_30px_rgb(0,0,0,0.12)] hover:shadow-[0_8px_40px_rgba(234,179,8,0.3)] flex items-center justify-center group active:scale-95 transition-all duration-300 ease-out translate-y-0 opacity-100 pointer-events-auto"
          aria-label="Gerät registrieren"
        >
          <div className="absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"></div>
          <svg className="relative h-6 w-6 transition-transform duration-300 group-active:rotate-90 pointer-events-none" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2.5}><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>
          <div className="absolute inset-0 rounded-full bg-white/20 scale-0 group-hover:scale-100 transition-transform duration-500 opacity-0 group-hover:opacity-100 pointer-events-none"></div>
        </button>

        {error && (
          <div className="mb-6 rounded-lg bg-red-50 border border-red-200 p-4">
            <p className="text-sm text-red-800">{error}</p>
          </div>
        )}

        {filteredDevices.length === 0 ? (
          <div className="flex min-h-[300px] items-center justify-center">
            <div className="text-center">
              <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>
              <h3 className="mt-4 text-lg font-medium text-gray-900">{searchTerm ? 'Keine Geräte gefunden' : 'Keine Geräte vorhanden'}</h3>
              <p className="mt-2 text-sm text-gray-600">{searchTerm ? 'Versuchen Sie einen anderen Suchbegriff.' : 'Es wurden noch keine Geräte registriert.'}</p>
            </div>
          </div>
        ) : (
          <div className="space-y-3">
            {filteredDevices.map((device, index) => (
              <div
                key={device.id}
                onClick={() => void handleSelectDevice(device)}
                className="group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.01] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-1 active:scale-[0.99] md:hover:border-amber-300/60"
                style={{ animationName: 'fadeInUp', animationDuration: '0.5s', animationTimingFunction: 'ease-out', animationFillMode: 'forwards', animationDelay: `${index * 0.03}s`, opacity: 0 }}
              >
                <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-yellow-50/80 to-yellow-100/80 opacity-[0.03] rounded-3xl"></div>
                <div className="pointer-events-none absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                <div className="pointer-events-none absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-yellow-300/60 transition-all duration-300"></div>

                <div className="relative flex items-center gap-4 p-5">
                  <div className="flex-shrink-0">
                    <div className="h-12 w-12 rounded-full bg-gradient-to-br from-yellow-500 to-yellow-600 flex items-center justify-center text-white font-semibold shadow-md md:group-hover:scale-110 transition-transform duration-300">
                      {(device.name ?? device.device_id)?.charAt(0)?.toUpperCase() ?? 'D'}
                    </div>
                  </div>
                  <div className="flex-1 min-w-0">
                    <h3 className="text-lg font-semibold text-gray-900 md:group-hover:text-yellow-600 transition-colors duration-300">{device.name ?? device.device_id}</h3>
                    <div className="flex items-center gap-2 mt-1 flex-wrap">
                      <span className="inline-flex items-center px-2 py-1 rounded-full text-xs font-medium bg-gray-100 text-gray-700">{getDeviceTypeDisplayName(device.device_type)}</span>
                    </div>
                  </div>
                  <div className="flex-shrink-0">
                    <svg className="h-6 w-6 text-gray-400 md:group-hover:text-yellow-600 md:group-hover:translate-x-1 transition-all duration-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" /></svg>
                  </div>
                </div>

                <div className="pointer-events-none absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-amber-100/30 to-transparent"></div>
              </div>
            ))}
            <style jsx>{`
              @keyframes fadeInUp { from { opacity:0; transform: translateY(20px);} to {opacity:1; transform: translateY(0);} }
            `}</style>
          </div>
        )}
      </div>

      {/* Create */}
      <DeviceCreateModal isOpen={showCreateModal} onClose={() => setShowCreateModal(false)} onCreate={handleCreateDevice} loading={createLoading} />

      {/* Detail */}
      {selectedDevice && (
        <DeviceDetailModal
          isOpen={showDetailModal}
          onClose={() => { setShowDetailModal(false); setSelectedDevice(null); }}
          device={selectedDevice}
          onEdit={handleEditClick}
          onDelete={() => void handleDeleteDevice()}
          loading={detailLoading}
        />
      )}

      {/* Edit */}
      {selectedDevice && (
        <DeviceEditModal
          isOpen={showEditModal}
          onClose={() => { setShowEditModal(false); }}
          device={selectedDevice}
          onSave={handleUpdateDevice}
          loading={detailLoading}
        />
      )}

      {/* Success toasts are handled globally */}
    </ResponsiveLayout>
  );
}
