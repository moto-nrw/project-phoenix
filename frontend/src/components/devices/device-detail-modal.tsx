"use client";

import React, { useState } from "react";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import type { Device } from "@/lib/iot-helpers";
import {
  getDeviceStatusDisplayName,
  getDeviceTypeDisplayName,
  formatLastSeen,
} from "@/lib/iot-helpers";

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
  const [confirmOpen, setConfirmOpen] = useState(false);
  if (!device) return null;
  const initials = (
    device.name?.slice(0, 2) ??
    device.device_id.slice(0, 2) ??
    "DE"
  ).toUpperCase();

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-yellow-500" />
              <p className="text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4 md:space-y-6">
            {/* Header */}
            <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-yellow-500 to-yellow-600 text-xl font-semibold text-white shadow-md md:h-16 md:w-16">
                {initials}
              </div>
              <div className="min-w-0">
                <h2 className="truncate text-lg font-semibold text-gray-900 md:text-xl">
                  {device.name ?? device.device_id}
                </h2>
                <p className="truncate text-sm text-gray-500">
                  {getDeviceTypeDisplayName(device.device_type)}
                </p>
              </div>
            </div>

            {/* Details */}
            <div className="space-y-3 md:space-y-4">
              <div className="rounded-xl border border-gray-100 bg-yellow-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-yellow-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"
                    />
                  </svg>
                  Gerätedetails
                </h3>
                <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                  <div>
                    <dt className="text-xs text-gray-500">Geräte-ID</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {device.device_id}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Status</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {getDeviceStatusDisplayName(device.status)}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Online</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {device.is_online ? "Online" : "Offline"}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Zuletzt gesehen</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {formatLastSeen(device.last_seen)}
                    </dd>
                  </div>
                </dl>
              </div>

              {device.api_key && (
                <div className="rounded-xl border border-gray-100 bg-white p-3 md:p-4">
                  <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                    <svg
                      className="h-3.5 w-3.5 text-yellow-600 md:h-4 md:w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
                      />
                    </svg>
                    API-Schlüssel (nur einmal sichtbar)
                  </h3>
                  <div className="space-y-2">
                    <div className="flex items-center gap-2">
                      <input
                        type="password"
                        defaultValue={device.api_key}
                        readOnly
                        className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
                      />
                      <button
                        type="button"
                        onClick={(e) => {
                          const wrapper =
                            e.currentTarget.parentElement?.querySelector(
                              "input",
                            ) as HTMLInputElement | null;
                          if (!wrapper) return;
                          if (wrapper.type === "password") {
                            wrapper.type = "text";
                            (e.currentTarget as HTMLButtonElement).textContent =
                              "Verbergen";
                          } else {
                            wrapper.type = "password";
                            (e.currentTarget as HTMLButtonElement).textContent =
                              "Anzeigen";
                          }
                        }}
                        className="rounded bg-yellow-600 px-2 py-1 text-xs text-white hover:bg-yellow-700"
                      >
                        Anzeigen
                      </button>
                      <button
                        type="button"
                        onClick={async (e) => {
                          const btn = e.currentTarget as HTMLButtonElement;
                          const original = btn.textContent;
                          await navigator.clipboard.writeText(device.api_key!);
                          btn.textContent = "Kopiert!";
                          setTimeout(() => {
                            if (btn) btn.textContent = original ?? "Kopieren";
                          }, 1500);
                        }}
                        className="rounded bg-gray-900 px-2 py-1 text-xs text-white hover:bg-gray-700"
                      >
                        Kopieren
                      </button>
                    </div>
                    <div className="rounded-md border border-yellow-200 bg-yellow-50 p-2">
                      <div className="text-xs text-yellow-800">
                        Sicherheit: Bewahren Sie diesen Schlüssel sicher auf. Er
                        ist nur bei der Erstellung sichtbar.
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* Actions */}
            <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex flex-wrap gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className="rounded-lg border border-red-300 px-3 py-2 text-xs font-medium text-red-700 transition-all duration-200 hover:border-red-400 hover:bg-red-50 hover:shadow-md active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
              >
                <span className="flex items-center gap-2">
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                  Löschen
                </span>
              </button>
              <button
                type="button"
                onClick={onEdit}
                className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
              >
                <span className="flex items-center justify-center gap-2">
                  <svg
                    className="h-4 w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                    />
                  </svg>
                  Bearbeiten
                </span>
              </button>
            </div>
          </div>
        )}
      </Modal>

      {/* Delete confirmation */}
      <ConfirmationModal
        isOpen={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        onConfirm={() => {
          setConfirmOpen(false);
          onDelete();
        }}
        title="Gerät löschen?"
        confirmText="Löschen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-700">
          Möchten Sie das Gerät{" "}
          <span className="font-medium">
            {device?.name ?? device?.device_id}
          </span>{" "}
          wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </ConfirmationModal>
    </>
  );
}
