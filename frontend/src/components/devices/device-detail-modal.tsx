"use client";

import React, { useState } from "react";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import type { Device } from "@/lib/iot-helpers";
import { getDeviceStatusDisplayName, getDeviceTypeDisplayName, formatLastSeen } from "@/lib/iot-helpers";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  device: Device | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
}

export function DeviceDetailModal({
  isOpen,
  onClose,
  device,
  onEdit,
  onDelete,
  loading = false,
}: Props) {
  if (!device) return null;

  const [confirmOpen, setConfirmOpen] = useState(false);
  const initials = (device.name?.slice(0, 2) ?? device.device_id.slice(0,2) ?? 'DE').toUpperCase();

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-amber-500" />
              <p className="text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4 md:space-y-6">
            {/* Header */}
            <div className="flex items-center gap-3 md:gap-4 pb-3 md:pb-4 border-b border-gray-100">
              <div className="h-14 w-14 md:h-16 md:w-16 rounded-full bg-gradient-to-br from-amber-500 to-orange-600 flex items-center justify-center text-white text-xl font-semibold shadow-md">
                {initials}
              </div>
              <div className="min-w-0">
                <h2 className="text-lg md:text-xl font-semibold text-gray-900 truncate">{device.name ?? device.device_id}</h2>
                <p className="text-sm text-gray-500 truncate">{getDeviceTypeDisplayName(device.device_type)}</p>
              </div>
            </div>

            {/* Details */}
            <div className="space-y-3 md:space-y-4">
              <div className="rounded-xl border border-gray-100 bg-amber-50/30 p-3 md:p-4">
                <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                  <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>
                  Gerätedetails
                </h3>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                  <div>
                    <dt className="text-xs text-gray-500">Geräte-ID</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{device.device_id}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Status</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{getDeviceStatusDisplayName(device.status)}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Online</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{device.is_online ? 'Online' : 'Offline'}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Zuletzt gesehen</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{formatLastSeen(device.last_seen)}</dd>
                  </div>
                </dl>
              </div>

              {device.api_key && (
                <div className="rounded-xl border border-gray-100 bg-white p-3 md:p-4">
                  <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                    <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-amber-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 11c0-3.314 2.686-6 6-6a6 6 0 110 12 6 6 0 01-6-6zm-8 4a4 4 0 118 0v2a2 2 0 11-4 0" /></svg>
                    API-Schlüssel (nur einmal sichtbar)
                  </h3>
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <input type="password" defaultValue={device.api_key} readOnly className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm" />
                      <button
                        type="button"
                        onClick={(e) => {
                          const wrapper = (e.currentTarget.parentElement?.querySelector('input')) as HTMLInputElement | null;
                          if (!wrapper) return;
                          if (wrapper.type === 'password') { wrapper.type = 'text'; (e.currentTarget as HTMLButtonElement).textContent = 'Verbergen'; }
                          else { wrapper.type = 'password'; (e.currentTarget as HTMLButtonElement).textContent = 'Anzeigen'; }
                        }}
                        className="px-2 py-1 bg-amber-600 text-white text-xs rounded hover:bg-amber-700"
                      >Anzeigen</button>
                      <button
                        type="button"
                        onClick={async (e) => {
                          await navigator.clipboard.writeText(device.api_key!);
                          const btn = e.currentTarget as HTMLButtonElement;
                          const original = btn.textContent;
                          btn.textContent = 'Kopiert!';
                          setTimeout(() => { if (btn) btn.textContent = original ?? 'Kopieren'; }, 1500);
                        }}
                        className="px-2 py-1 bg-gray-900 text-white text-xs rounded hover:bg-gray-700"
                      >Kopieren</button>
                    </div>
                    <div className="bg-amber-50 border border-amber-200 rounded-md p-2">
                      <div className="text-xs text-amber-800">
                        Sicherheit: Bewahren Sie diesen Schlüssel sicher auf. Er ist nur bei der Erstellung sichtbar.
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Actions */}
            <div className="sticky bottom-0 bg-white/95 backdrop-blur-sm flex flex-wrap gap-2 md:gap-3 py-3 md:py-4 border-t border-gray-100 -mx-4 md:-mx-6 -mb-4 md:-mb-6 px-4 md:px-6 mt-4 md:mt-6">
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className="px-3 md:px-4 py-2 rounded-lg border border-red-300 text-xs md:text-sm font-medium text-red-700 hover:bg-red-50 hover:border-red-400 hover:shadow-md md:hover:scale-105 active:scale-100 transition-all duration-200"
              >
                Löschen
              </button>
              <button
                type="button"
                onClick={onEdit}
                className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-gray-900 text-xs md:text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200"
              >
                Bearbeiten
              </button>
            </div>
          </div>
        )}
      </Modal>

      {/* Delete confirmation */}
      <ConfirmationModal
        isOpen={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => { setConfirmOpen(false); onDelete(); }}
        title="Gerät löschen?"
        confirmText="Löschen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-700">
          Möchten Sie das Gerät <span className="font-medium">{device?.name ?? device?.device_id}</span> wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </ConfirmationModal>
    </>
  );
}
