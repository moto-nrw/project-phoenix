"use client";

import { useMemo } from "react";
import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import type { Room } from "@/lib/room-helpers";
import { roomsConfig } from "@/lib/database/configs/rooms.config";
import { configToFormSection } from "@/lib/database/types";

// Standard categories that are always available
const STANDARD_CATEGORIES = [
  "Normaler Raum",
  "Gruppenraum",
  "Themenraum",
  "Sport",
];

interface RoomEditModalProps {
  isOpen: boolean;
  onClose: () => void;
  room: Room | null;
  onSave: (data: Partial<Room>) => Promise<void>;
  loading?: boolean;
}

export function RoomEditModal({
  isOpen,
  onClose,
  room,
  onSave,
  loading = false,
}: RoomEditModalProps) {
  // Dynamically add legacy category if room has one not in standard list
  const sections = useMemo(() => {
    const baseSections = roomsConfig.form.sections.map(configToFormSection);

    // Check if room has a legacy category
    if (room?.category && !STANDARD_CATEGORIES.includes(room.category)) {
      // Find the category field and add the legacy option
      return baseSections.map((section) => ({
        ...section,
        fields: section.fields.map((field) => {
          if (field.name === "category" && Array.isArray(field.options)) {
            return {
              ...field,
              options: [
                ...field.options,
                { value: room.category!, label: `${room.category} (Legacy)` },
              ],
            };
          }
          return field;
        }),
      }));
    }

    return baseSections;
  }, [room?.category]);

  if (!room) return null;

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={roomsConfig.labels?.editModalTitle ?? "Raum bearbeiten"}
    >
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-500" />
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={roomsConfig.theme}
          sections={sections}
          initialData={room}
          onSubmit={onSave}
          onCancel={onClose}
          isLoading={loading}
          submitLabel="Speichern"
          stickyActions
        />
      )}
    </Modal>
  );
}
