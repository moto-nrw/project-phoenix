"use client";

import React from "react";
import { Modal } from "~/components/ui/modal";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import type { Device } from "@/lib/iot-helpers";
import {
  getDeviceStatusDisplayName,
  getDeviceTypeDisplayName,
  formatLastSeen,
} from "@/lib/iot-helpers";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly device: Device | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly loading?: boolean;
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
  const initials = (
    device.name?.slice(0, 2) ??
    device.device_id.slice(0, 2) ??
    "DE"
  ).toUpperCase();

  return (
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

          <DetailModalActions
            onEdit={onEdit}
            onDelete={onDelete}
            entityName={device.name ?? device.device_id}
            entityType="Gerät"
          />
        </div>
      )}
    </Modal>
  );
}
