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

export function RoomDetailModal({
  isOpen,
  onClose,
  room,
  onEdit,
  onDelete,
  loading = false,
}: RoomDetailModalProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  useEffect(() => {
    if (!isOpen) setShowDeleteConfirm(false);
  }, [isOpen]);

  if (!room) return null;

  const initial = room.name?.charAt(0)?.toUpperCase() ?? "R";

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
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
              <svg
                className="h-8 w-8 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
          </div>
          <div className="space-y-3 text-center">
            <h3 className="text-xl font-bold text-gray-900">Raum löschen?</h3>
            <p className="text-sm text-gray-700">
              Möchten Sie den Raum <strong>{room.name}</strong> wirklich
              löschen?
            </p>
            <p className="text-sm font-medium text-red-600">
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>
          </div>
          <div className="flex gap-3 border-t border-gray-100 pt-4">
            <button
              type="button"
              onClick={() => setShowDeleteConfirm(false)}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={onDelete}
              className="flex-1 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-red-700 hover:shadow-lg active:scale-100"
            >
              Löschen
            </button>
          </div>
        </div>
      ) : (
        <div className="space-y-4 md:space-y-6">
          {/* Header */}
          <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
            <div className="flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-indigo-600 text-xl font-semibold text-white shadow-md md:h-16 md:w-16">
              {initial}
            </div>
            <div className="min-w-0">
              <h2 className="truncate text-lg font-semibold text-gray-900 md:text-xl">
                {room.name}
              </h2>
              {(room.building !== undefined || room.floor !== undefined) && (
                <p className="truncate text-sm text-gray-500">
                  {room.building &&
                    room.floor !== undefined &&
                    `${room.building}, Etage ${room.floor}`}
                  {room.building && room.floor === undefined && room.building}
                  {!room.building &&
                    room.floor !== undefined &&
                    `Etage ${room.floor}`}
                </p>
              )}
            </div>
          </div>

          {/* Details */}
          <div className="space-y-3 md:space-y-4">
            <div className="rounded-xl border border-gray-100 bg-indigo-50/30 p-3 md:p-4">
              <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-indigo-600 md:h-4 md:w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                  />
                </svg>
                Raumdetails
              </h3>
              <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                <div>
                  <dt className="text-xs text-gray-500">Kategorie</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {room.category ?? "Nicht angegeben"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Gebäude</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {room.building ?? "Nicht angegeben"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Etage</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {room.floor !== undefined
                      ? `Etage ${room.floor}`
                      : "Nicht angegeben"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Status</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {room.isOccupied ? "Belegt" : "Frei"}
                  </dd>
                </div>
                {/* Activity and group info if occupied */}
                {room.activityName && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Aktivität</dt>
                    <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                      {room.activityName}
                    </dd>
                  </div>
                )}
                {room.groupName && (
                  <div className="sm:col-span-2">
                    <dt className="text-xs text-gray-500">Gruppe</dt>
                    <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                      {room.groupName}
                    </dd>
                  </div>
                )}
              </dl>
            </div>
          </div>

          {/* Actions */}
          <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
            <button
              type="button"
              onClick={handleDelete}
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
  );
}
