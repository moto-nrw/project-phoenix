"use client";

import React, { useState } from "react";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import type { Permission } from "@/lib/auth-helpers";
import { formatPermissionDisplay } from "@/lib/permission-labels";

interface Props {
  isOpen: boolean;
  onClose: () => void;
  permission: Permission | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
}

export function PermissionDetailModal({
  isOpen,
  onClose,
  permission,
  onEdit,
  onDelete,
  loading = false,
}: Props) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  if (!permission) return null;
  const initials = (permission.resource?.slice(0, 2) ?? "PE").toUpperCase();
  const displayTitle = formatPermissionDisplay(
    permission.resource,
    permission.action,
  );

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-pink-600" />
              <p className="text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4 md:space-y-6">
            {/* Header */}
            <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-pink-500 to-rose-600 text-xl font-semibold text-white shadow-md md:h-16 md:w-16">
                {initials}
              </div>
              <div className="min-w-0">
                <h2 className="truncate text-lg font-semibold text-gray-900 md:text-xl">
                  {displayTitle}
                </h2>
                <p className="truncate text-sm text-gray-500">
                  {permission.name || "Systemberechtigung"}
                </p>
              </div>
            </div>

            {/* Details */}
            <div className="space-y-3 md:space-y-4">
              <div className="rounded-xl border border-gray-100 bg-pink-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-pink-600 md:h-4 md:w-4"
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
                  Berechtigungsdetails
                </h3>
                <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                  <div>
                    <dt className="text-xs text-gray-500">Ressource</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {permission.resource}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Aktion</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {permission.action}
                    </dd>
                  </div>
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Anzeigename</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {permission.name || "—"}
                    </dd>
                  </div>
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Beschreibung</dt>
                    <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                      {permission.description || "Keine Beschreibung"}
                    </dd>
                  </div>
                </dl>
              </div>
            </div>

            {/* Actions */}
            <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex flex-wrap gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className="flex-1 truncate rounded-lg border border-red-300 px-3 py-2 text-center text-xs font-medium whitespace-nowrap text-red-700 transition-all duration-200 hover:border-red-400 hover:bg-red-50 hover:shadow-md active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
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
                      d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                    />
                  </svg>
                  Löschen
                </span>
              </button>
              <button
                type="button"
                onClick={onEdit}
                className="flex-1 truncate rounded-lg bg-gray-900 px-3 py-2 text-center text-xs font-medium whitespace-nowrap text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
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
        title="Berechtigung löschen?"
        confirmText="Löschen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-700">
          Möchten Sie die Berechtigung{" "}
          <span className="font-medium">
            {permission?.resource}: {permission?.action}
          </span>{" "}
          wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </ConfirmationModal>
    </>
  );
}
