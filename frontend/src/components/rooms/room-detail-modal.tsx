"use client";

import { useEffect, useState } from "react";
import { Modal } from "~/components/ui/modal";
import { InlineDeleteConfirmation } from "~/components/ui/inline-delete-confirmation";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import type { Room } from "@/lib/room-helpers";

interface RoomDetailModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly room: Room | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly loading?: boolean;
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

  // Render helper for modal content
  const renderContent = () => {
    if (loading) {
      return (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-500" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      );
    }

    if (showDeleteConfirm) {
      return (
        <InlineDeleteConfirmation
          title="Raum löschen?"
          onCancel={() => setShowDeleteConfirm(false)}
          onConfirm={onDelete}
        >
          <p className="text-sm text-gray-700">
            Möchten Sie den Raum <strong>{room.name}</strong> wirklich löschen?
          </p>
        </InlineDeleteConfirmation>
      );
    }

    return (
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

        <DetailModalActions
          onEdit={onEdit}
          onDelete={onDelete}
          onDeleteClick={() => setShowDeleteConfirm(true)}
          entityName={room.name}
          entityType="Raum"
        />
      </div>
    );
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="">
      {renderContent()}
    </Modal>
  );
}
