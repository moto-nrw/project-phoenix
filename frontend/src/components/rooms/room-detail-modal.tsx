"use client";

import { useEffect, useState } from "react";
import { Modal } from "~/components/ui/modal";
import type { Room } from "@/lib/room-helpers";

interface RoomDetailModalProps {
  isOpen: boolean;
  onClose: () => void;
  room: Room | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
}

export function RoomDetailModal({ isOpen, onClose, room, onEdit, onDelete, loading = false }: RoomDetailModalProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  useEffect(() => {
    if (!isOpen) setShowDeleteConfirm(false);
  }, [isOpen]);

  if (!room) return null;

  const initial = room.name?.charAt(0)?.toUpperCase() ?? 'R';

  const handleDelete = () => setShowDeleteConfirm(true);

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="">
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-500" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : showDeleteConfirm ? (
        <div className="space-y-6">
          <div className="flex justify-center">
            <div className="w-16 h-16 rounded-full bg-red-100 flex items-center justify-center">
              <svg className="w-8 h-8 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" /></svg>
            </div>
          </div>
          <div className="text-center space-y-3">
            <h3 className="text-xl font-bold text-gray-900">Raum löschen?</h3>
            <p className="text-sm text-gray-700">Möchten Sie den Raum <strong>{room.name}</strong> wirklich löschen?</p>
            <p className="text-sm text-red-600 font-medium">Diese Aktion kann nicht rückgängig gemacht werden.</p>
          </div>
          <div className="flex gap-3 pt-4 border-t border-gray-100">
            <button type="button" onClick={() => setShowDeleteConfirm(false)} className="flex-1 px-4 py-2 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200">Abbrechen</button>
            <button type="button" onClick={onDelete} className="flex-1 px-4 py-2 rounded-lg bg-red-600 text-sm font-medium text-white hover:bg-red-700 hover:shadow-lg hover:scale-105 active:scale-100 transition-all duration-200">Löschen</button>
          </div>
        </div>
      ) : (
        <div className="space-y-4 md:space-y-6">
          {/* Header */}
          <div className="flex items-center gap-3 md:gap-4 pb-3 md:pb-4 border-b border-gray-100">
            <div className="h-14 w-14 md:h-16 md:w-16 rounded-full bg-gradient-to-br from-indigo-500 to-indigo-600 flex items-center justify-center text-white text-xl font-semibold shadow-md">
              {initial}
            </div>
            <div className="min-w-0">
              <h2 className="text-lg md:text-xl font-semibold text-gray-900 truncate">{room.name}</h2>
              <p className="text-sm text-gray-500 truncate">
                {room.building ? `${room.building}, ` : ''}Etage {room.floor}
              </p>
            </div>
          </div>

          {/* Details */}
          <div className="space-y-3 md:space-y-4">
            <div className="rounded-xl border border-gray-100 bg-indigo-50/30 p-3 md:p-4">
              <h3 className="text-xs md:text-sm font-semibold text-gray-900 mb-2 md:mb-3 flex items-center gap-2">
                <svg className="h-3.5 w-3.5 md:h-4 md:w-4 text-indigo-600" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" /></svg>
                Raumdetails
              </h3>
              <dl className="grid grid-cols-1 sm:grid-cols-2 gap-x-3 md:gap-x-4 gap-y-2 md:gap-y-3">
                <div>
                  <dt className="text-xs text-gray-500">Kategorie</dt>
                  <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{room.category}</dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Kapazität</dt>
                  <dd className="text-sm font-medium text-gray-900 mt-0.5 break-words">{room.capacity ?? 0} Plätze</dd>
                </div>
                {/* Status removed as per new UI (no status indicators) */}
                {room.activityName && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Aktivität</dt>
                    <dd className="text-xs md:text-sm text-gray-700 mt-0.5 whitespace-pre-wrap break-words">{room.activityName}</dd>
                  </div>
                )}
                {room.groupName && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Gruppe</dt>
                    <dd className="text-xs md:text-sm text-gray-700 mt-0.5 whitespace-pre-wrap break-words">{room.groupName}</dd>
                  </div>
                )}
              </dl>
            </div>
          </div>

          {/* Actions */}
          <div className="sticky bottom-0 bg-white/95 backdrop-blur-sm flex gap-2 md:gap-3 py-3 md:py-4 border-t border-gray-100 -mx-4 md:-mx-6 -mb-4 md:-mb-6 px-4 md:px-6 mt-4 md:mt-6">
            <button type="button" onClick={handleDelete} className="px-3 md:px-4 py-2 rounded-lg border border-red-300 text-xs md:text-sm font-medium text-red-700 hover:bg-red-50 hover:border-red-400 hover:shadow-md md:hover:scale-105 active:scale-100 transition-all duration-200">Löschen</button>
            <button type="button" onClick={onEdit} className="flex-1 px-3 md:px-4 py-2 rounded-lg bg-gray-900 text-xs md:text-sm font-medium text-white hover:bg-gray-700 hover:shadow-lg md:hover:scale-105 active:scale-100 transition-all duration-200">Bearbeiten</button>
          </div>
        </div>
      )}
    </Modal>
  );
}
