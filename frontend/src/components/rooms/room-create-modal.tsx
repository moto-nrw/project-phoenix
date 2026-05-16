"use client";

import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import type { Room } from "@/lib/room-helpers";
import { roomsConfig } from "@/lib/database/configs/rooms.config";
import { configToFormSection } from "@/lib/database/types";

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
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={roomsConfig.labels?.createModalTitle ?? "Neuer Raum"}
    >
      <DatabaseForm
        theme={roomsConfig.theme}
        sections={roomsConfig.form.sections.map(configToFormSection)}
        initialData={roomsConfig.form.defaultValues}
        onSubmit={onCreate}
        onCancel={onClose}
        submitLabel="Erstellen"
        stickyActions
      />
    </Modal>
  );
}
