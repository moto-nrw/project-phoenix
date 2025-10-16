"use client";

import React, { useState } from "react";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import type { Permission } from "@/lib/auth-helpers";

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
  const resourceLabels: Record<string, string> = {
    users: 'Benutzer', roles: 'Rollen', permissions: 'Berechtigungen', activities: 'Aktivitäten',
    rooms: 'Räume', groups: 'Gruppen', visits: 'Besuche', schedules: 'Zeitpläne', config: 'Konfiguration',
    feedback: 'Feedback', iot: 'Geräte', system: 'System', admin: 'Administration',
  };
  const actionLabels: Record<string, string> = {
    create: 'Erstellen', read: 'Ansehen', update: 'Bearbeiten', delete: 'Löschen', list: 'Auflisten',
    manage: 'Verwalten', assign: 'Zuweisen', enroll: 'Anmelden', '*': 'Alle',
  };
  const displayTitle = `${resourceLabels[permission.resource] ?? permission.resource}: ${actionLabels[permission.action] ?? permission.action}`;

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
            <div className="flex items-center gap-3 md:gap-4 pb-3 md:pb-4 border-b border-gray-100">
              <div className="h-14 w-14 md:h-16 md:w-16 rounded-full bg-gradient-to-br from-pink-500 to-rose-600 flex items-center justify-center text-white text-xl font-semibold shadow-md">
                {initials}
              </div>
              <div className="min-w-0">
                <h2 className="text-lg md:text-xl font-semibold text-gray-900 truncate">{displayTitle}</h2>
                <p className="text-sm text-gray-500 truncate">{permission.name || 'Systemberechtigung'}</p>
              </div>
            </div>

            {/* Details */}
            <div className="space-y-3 md:space-y-4">
              <div className="rounded-xl border border-gray-100 bg-pink-50/30 p-3 md:p-4">
                <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                  <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-pink-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" /></svg>
                  Berechtigungsdetails
                </h3>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                  <div>
                    <dt className="text-xs text-gray-500">Ressource</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{permission.resource}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Aktion</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{permission.action}</dd>
                  </div>
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Anzeigename</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{permission.name || '—'}</dd>
                  </div>
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Beschreibung</dt>
                    <dd className="text-xs md:text-sm text-gray-700 mt-0.5 whitespace-pre-wrap break-words">{permission.description || 'Keine Beschreibung'}</dd>
                  </div>
                </dl>
              </div>
            </div>

            {/* Actions */}
            <div className="sticky bottom-0 bg-white/95 backdrop-blur-sm flex flex-wrap gap-2 md:gap-3 py-3 md:py-4 border-t border-gray-100 -mx-4 md:-mx-6 -mb-4 md:-mb-6 px-4 md:px-6 mt-4 md:mt-6">
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className="flex-1 px-3 md:px-4 py-2 rounded-lg border border-red-300 text-xs md:text-sm font-medium text-red-700 hover:bg-red-50 hover:border-red-400 hover:shadow-md md:hover:scale-105 active:scale-100 transition-all duration-200 whitespace-nowrap truncate text-center"
              >
                Löschen
              </button>
              <button
                type="button"
                onClick={onEdit}
                className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-gray-900 text-xs md:text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200 whitespace-nowrap truncate text-center"
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
        title="Berechtigung löschen?"
        confirmText="Löschen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-700">
          Möchten Sie die Berechtigung <span className="font-medium">{permission?.resource}: {permission?.action}</span> wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </ConfirmationModal>
    </>
  );
}
