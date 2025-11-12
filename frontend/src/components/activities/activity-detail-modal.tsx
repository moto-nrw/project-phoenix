"use client";

import React, { useState } from "react";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import type { Activity } from "@/lib/activity-helpers";

interface ActivityDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  activity: Activity | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
}

export function ActivityDetailModal({
  isOpen,
  onClose,
  activity,
  onEdit,
  onDelete,
  loading = false,
}: ActivityDetailModalProps) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  if (!activity) return null;

  const initials = (activity.name?.slice(0, 2) ?? "AG").toUpperCase();

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#FF3130]" />
              <p className="text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4 md:space-y-6">
            {/* Header */}
            <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
              <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-[#FF3130] to-[#e02020] text-xl font-semibold text-white shadow-md md:h-16 md:w-16">
                {initials}
              </div>
              <div className="min-w-0">
                <h2 className="truncate text-lg font-semibold text-gray-900 md:text-xl">
                  {activity.name}
                </h2>
                <p className="truncate text-sm text-gray-500">
                  {activity.category_name ?? "Keine Kategorie"}
                </p>
              </div>
            </div>

            {/* Details */}
            <div className="space-y-3 md:space-y-4">
              <div className="rounded-xl border border-gray-100 bg-red-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-red-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"
                    />
                  </svg>
                  Aktivitätsdetails
                </h3>
                <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                  <div>
                    <dt className="text-xs text-gray-500">Kategorie</dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {activity.category_name ?? "Keine Kategorie"}
                    </dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">
                      Maximale Teilnehmer
                    </dt>
                    <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                      {activity.max_participant}
                    </dd>
                  </div>
                  {activity.supervisor_name && (
                    <div className="sm:col-span-2">
                      <dt className="text-xs text-gray-500">Hauptbetreuer</dt>
                      <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                        {activity.supervisor_name}
                      </dd>
                    </div>
                  )}
                </dl>
              </div>
            </div>

            {/* Actions */}
            <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex flex-wrap gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
              <button
                type="button"
                onClick={() => setConfirmOpen(true)}
                className="rounded-lg border border-red-300 px-3 py-2 text-xs font-medium text-red-700 transition-all duration-200 hover:border-red-400 hover:bg-red-50 hover:shadow-md active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
              >
                Löschen
              </button>
              <button
                type="button"
                onClick={onEdit}
                className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
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
        onConfirm={() => {
          setConfirmOpen(false);
          onDelete();
        }}
        title="Aktivität löschen?"
        confirmText="Löschen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-700">
          Möchten Sie die Aktivität{" "}
          <span className="font-medium">{activity?.name}</span> wirklich
          löschen? Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </ConfirmationModal>
    </>
  );
}
