"use client";

import { useEffect, useState } from "react";
import useSWR from "swr";
import { Modal } from "~/components/ui/modal";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import type { Room } from "@/lib/room-helpers";
import { roomsConfig } from "@/lib/database/configs/rooms.config";
import { configToFormSection } from "@/lib/database/types";
import { pickRoomColor } from "~/lib/location-helper";
import { fetchRooms } from "~/lib/rooms-api";

interface RoomCreateModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onCreate: (data: Partial<Room>) => Promise<void>;
  readonly loading?: boolean;
}

function buildInitialRoomData(usedColors: readonly string[]): Partial<Room> {
  return {
    ...roomsConfig.form.defaultValues,
    color: pickRoomColor(usedColors),
  };
}

export function RoomCreateModal({
  isOpen,
  onClose,
  onCreate,
  loading = false,
}: RoomCreateModalProps) {
  const [initialData, setInitialData] = useState<Partial<Room> | null>(null);
  const tenantSlug = useTenantSlugSafe();

  // Fetch existing rooms once so the soft-prefer-unique default knows which
  // palette colors are already in use. `fetchRooms` already swallows errors
  // into `[]` and logs internally — failure here only degrades the color
  // pick to "any palette color", which is acceptable.
  // Cache key is tenant-prefixed to prevent cross-tenant cache bleed when
  // switching tenants in the same browser session.
  const { data: rooms, isLoading: roomsLoading } = useSWR(
    isOpen ? `${tenantSlug ?? "no-tenant"}:rooms-list-for-color-default` : null,
    () => fetchRooms(),
    { revalidateOnFocus: false },
  );

  useEffect(() => {
    if (!isOpen) {
      setInitialData(null);
      return;
    }

    if (initialData) {
      return;
    }

    // On a cold open, wait for the first rooms result before choosing a
    // default so we can actually prefer an unused palette color.
    if (roomsLoading && rooms === undefined) {
      return;
    }

    // Seed once per open cycle. After that, keep the object stable so
    // DatabaseForm does not reset user-entered values during revalidation.
    const usedColors = (rooms ?? [])
      .map((r) => r.color ?? "")
      .filter((c) => c !== "");
    setInitialData(buildInitialRoomData(usedColors));
  }, [initialData, isOpen, rooms, roomsLoading]);

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
      ) : !initialData ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-indigo-500" />
            <p className="text-gray-600">Formular wird vorbereitet...</p>
          </div>
        </div>
      ) : (
        <DatabaseForm
          theme={roomsConfig.theme}
          sections={roomsConfig.form.sections.map(configToFormSection)}
          initialData={initialData}
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
