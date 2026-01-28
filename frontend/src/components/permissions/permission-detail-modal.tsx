"use client";

import React from "react";
import { Modal } from "~/components/ui/modal";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import type { Permission } from "@/lib/auth-helpers";
import { formatPermissionDisplay } from "@/lib/permission-labels";

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly permission: Permission | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly loading?: boolean;
  /**
   * Custom click handler for delete button.
   * When provided, bypasses internal confirmation modal.
   * Use this to handle confirmation at the page level.
   */
  readonly onDeleteClick?: () => void;
}

export function PermissionDetailModal({
  isOpen,
  onClose,
  permission,
  onEdit,
  onDelete,
  loading = false,
  onDeleteClick,
}: Props) {
  if (!permission) return null;
  const initials = (permission.resource?.slice(0, 2) ?? "PE").toUpperCase();
  const displayTitle = formatPermissionDisplay(
    permission.resource,
    permission.action,
  );

  return (
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

          <DetailModalActions
            onEdit={onEdit}
            onDelete={onDelete}
            entityName={`${permission.resource}: ${permission.action}`}
            entityType="Berechtigung"
            onDeleteClick={onDeleteClick}
            confirmationContent={
              <p className="text-sm text-gray-700">
                Möchten Sie die Berechtigung{" "}
                <span className="font-medium">
                  {permission.resource}: {permission.action}
                </span>{" "}
                wirklich löschen? Diese Aktion kann nicht rückgängig gemacht
                werden.
              </p>
            }
          />
        </div>
      )}
    </Modal>
  );
}
