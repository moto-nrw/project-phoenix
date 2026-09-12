"use client";

import { useMemo, useState } from "react";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { isSystemRoom, type Room } from "~/lib/room-helpers";
import { buildRoomFormSections } from "./room-form-sections";

interface RoomStammdatenTabProps {
  readonly room: Room;
  readonly onSave: (data: Partial<Room>) => Promise<void>;
}

/**
 * Der Reiter „Stammdaten" der Raumseite (#3115): das Formular, das vorher im
 * Pane der Datenverwaltung lag. Ein Speichern unten, das Formular setzt sich
 * nach dem Speichern, beim Abbrechen und beim Raumwechsel auf den
 * gespeicherten Stand zurück, damit kein alter Entwurf stehen bleibt.
 */
export function RoomStammdatenTab({ room, onSave }: RoomStammdatenTabProps) {
  const [formResetCounter, setFormResetCounter] = useState(0);
  const sections = useMemo(() => buildRoomFormSections(room), [room]);

  const handleSave = async (data: Partial<Room>) => {
    await onSave(data);
    setFormResetCounter((n) => n + 1);
  };

  return (
    <div className="space-y-4 sm:space-y-6">
      <SectionCard
        title="Belegung"
        description="Der aktuelle Zustand des Raums, wie ihn die Aufsicht sieht."
      >
        <div className="mb-3 flex flex-wrap items-center gap-2">
          <StatusBadge
            label={room.isOccupied ? "Belegt" : "Frei"}
            tone={room.isOccupied ? "orange" : "green"}
          />
          {room.capacity !== undefined ? (
            <StatusBadge label={`${room.capacity} Plätze`} tone="gray" />
          ) : null}
          {isSystemRoom(room) ? (
            <StatusBadge label="Systemraum" tone="gray" />
          ) : null}
        </div>
        {room.activityName || room.groupName ? (
          <DataGrid>
            {room.activityName ? (
              <DataField label="Aktivität">{room.activityName}</DataField>
            ) : null}
            {room.groupName ? (
              <DataField label="Gruppe">{room.groupName}</DataField>
            ) : null}
          </DataGrid>
        ) : null}
      </SectionCard>

      <SectionCard title="Stammdaten">
        <DatabaseForm
          key={`${room.id}:${formResetCounter}`}
          sections={sections}
          initialData={room}
          onSubmit={handleSave}
          onCancel={() => setFormResetCounter((n) => n + 1)}
          submitLabel="Speichern"
          stickyActions
          sectionLevel={3}
        />
      </SectionCard>
    </div>
  );
}
