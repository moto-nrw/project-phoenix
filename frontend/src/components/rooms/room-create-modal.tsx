"use client";

import type { Room } from "@/lib/room-helpers";
import { roomsConfig } from "@/lib/database/configs/rooms.config";
import { DatabaseFormModal } from "~/components/ui/database/database-form-modal";

interface RoomCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Room>) => Promise<void>;
}

export function RoomCreateModal({
  isOpen,
  onClose,
  onCreate,
}: RoomCreateModalProps) {
  return (
    <DatabaseFormModal<Room>
      isOpen={isOpen}
      onClose={onClose}
      title={roomsConfig.labels?.createModalTitle ?? "Neuer Raum"}
      config={roomsConfig}
      onSubmit={onCreate}
      submitLabel="Erstellen"
      stickyActions
    />
  );
}
