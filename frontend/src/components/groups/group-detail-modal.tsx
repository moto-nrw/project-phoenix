"use client";

import React, { useState } from "react";
import { Modal, ConfirmationModal } from "~/components/ui/modal";
import type { Group } from "@/lib/group-helpers";

interface GroupDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  group: Group | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
}

export function GroupDetailModal({
  isOpen,
  onClose,
  group,
  onEdit,
  onDelete,
  loading = false,
}: GroupDetailModalProps) {
  if (!group) return null;

  const [confirmOpen, setConfirmOpen] = useState(false);

  const initials = (group.name?.slice(0, 2) ?? "GR").toUpperCase();

  return (
    <>
      <Modal isOpen={isOpen} onClose={onClose} title="">
        {loading ? (
          <div className="flex items-center justify-center py-12">
            <div className="flex flex-col items-center gap-4">
              <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#83CD2D]" />
              <p className="text-gray-600">Daten werden geladen...</p>
            </div>
          </div>
        ) : (
          <div className="space-y-4 md:space-y-6">
            {/* Header */}
          <div className="flex items-center gap-3 md:gap-4 pb-3 md:pb-4 border-b border-gray-100">
              <div className="h-14 w-14 md:h-16 md:w-16 rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] flex items-center justify-center text-white text-xl font-semibold shadow-md">
                {initials}
              </div>
              <div className="min-w-0">
                <h2 className="text-lg md:text-xl font-semibold text-gray-900 truncate">{group.name}</h2>
                <p className="text-sm text-gray-500 truncate">{group.room_name ?? "Kein Raum zugewiesen"}</p>
              </div>
            </div>

            {/* Details */}
            <div className="space-y-3 md:space-y-4">
              <div className="rounded-xl border border-gray-100 bg-green-50/30 p-3 md:p-4">
                <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                  <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" /></svg>
                  Gruppendetails
                </h3>
                <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                  <div>
                    <dt className="text-xs text-gray-500">Raum</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{group.room_name ?? "Kein Raum"}</dd>
                  </div>
                  <div>
                    <dt className="text-xs text-gray-500">Anzahl Schüler</dt>
                    <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{group.student_count ?? 0}</dd>
                  </div>
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Aufsichtspersonen</dt>
                    <dd className="text-xs md:text-sm text-gray-700 mt-0.5 whitespace-pre-wrap break-words">
                      {group.supervisors && group.supervisors.length > 0
                        ? group.supervisors.map((s) => s.name).join(", ")
                        : "Keine Aufsichtspersonen zugewiesen"}
                    </dd>
                  </div>
                </dl>
              </div>
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
        onConfirm={() => {
          setConfirmOpen(false);
          onDelete();
        }}
        title="Gruppe löschen?"
        confirmText="Löschen"
        cancelText="Abbrechen"
        confirmButtonClass="bg-red-600 hover:bg-red-700"
      >
        <p className="text-sm text-gray-700">
          Möchten Sie die Gruppe <span className="font-medium">{group?.name}</span> wirklich löschen? Diese Aktion kann nicht rückgängig gemacht werden.
        </p>
      </ConfirmationModal>
    </>
  );
}
