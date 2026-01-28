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
  readonly loading?: boolean;
}

export function RoomCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: RoomCreateModalProps) {
  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={roomsConfig.labels?.createModalTitle ?? "Neuer Raum"}
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
          sections={roomsConfig.form.sections.map(configToFormSection)}
          initialData={roomsConfig.form.defaultValues}
          onSubmit={onCreate}
          onCancel={onClose}
          isLoading={loading}
          submitLabel="Erstellen"
          stickyActions
        />
      )}
    </Modal>
  );
}
