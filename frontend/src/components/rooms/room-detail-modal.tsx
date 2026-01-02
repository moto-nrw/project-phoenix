"use client";

import { useEffect, useState } from "react";
import { Modal } from "~/components/ui/modal";
import { InlineDeleteConfirmation } from "~/components/ui/inline-delete-confirmation";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import { ModalLoadingState } from "~/components/ui/modal-loading-state";
import {
  DataField,
  DataGrid,
  InfoSection,
  DetailIcons,
} from "~/components/ui/detail-modal-components";
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
      return <ModalLoadingState accentColor="indigo" />;
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
                {room.building && room.floor !== undefined
                  ? `${room.building}, Etage ${room.floor}`
                  : (room.building ?? `Etage ${room.floor}`)}
              </p>
            )}
          </div>
        </div>

        {/* Details */}
        <div className="space-y-3 md:space-y-4">
          <InfoSection
            title="Raumdetails"
            icon={DetailIcons.building}
            accentColor="indigo"
          >
            <DataGrid>
              <DataField label="Kategorie">
                {room.category ?? "Nicht angegeben"}
              </DataField>
              <DataField label="Gebäude">
                {room.building ?? "Nicht angegeben"}
              </DataField>
              <DataField label="Etage">
                {room.floor === undefined
                  ? "Nicht angegeben"
                  : `Etage ${room.floor}`}
              </DataField>
              <DataField label="Status">
                {room.isOccupied ? "Belegt" : "Frei"}
              </DataField>
              {room.activityName && (
                <DataField label="Aktivität" fullWidth>
                  {room.activityName}
                </DataField>
              )}
              {room.groupName && (
                <DataField label="Gruppe" fullWidth>
                  {room.groupName}
                </DataField>
              )}
            </DataGrid>
          </InfoSection>
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
