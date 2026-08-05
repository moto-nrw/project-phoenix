"use client";

import { useMemo, useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { roomsConfig } from "@/components/database/configs/rooms.config";
import { formatFloor, isSystemRoom, type Room } from "@/lib/room-helpers";
import { buildRoomFormSections } from "./room-form-sections";

interface RoomsMasterDetailProps {
  groupDefinitions: GroupDefinition<Room>[];
  selectedId: string | null;
  // Resolved by the page against the unfiltered list so the detail panel
  // survives a search narrowing the visible rows. See devices-master-detail
  // for the same pattern.
  selectedRoom: Room | null;
  onSelect: (id: string | null) => void;
  onSaveRoom: (data: Partial<Room>) => Promise<void>;
  onDeleteClick: () => void;
}

function keyForRoom(room: Room): string {
  return room.id;
}

function buildRoomLocation(room: Room): string {
  if (room.building && room.floor !== undefined) {
    return `${room.building} · ${formatFloor(room.floor)}`;
  }
  if (room.building) return room.building;
  if (room.floor !== undefined) return formatFloor(room.floor);
  return "Kein Standort hinterlegt";
}

function buildRoomSubtitle(room: Room): string {
  const parts: string[] = [];
  if (room.building && room.floor !== undefined) {
    parts.push(`${room.building} · ${formatFloor(room.floor)}`);
  } else if (room.building) {
    parts.push(room.building);
  } else if (room.floor !== undefined) {
    parts.push(formatFloor(room.floor));
  }
  if (room.category) parts.push(room.category);
  const base = parts.join(" · ") || "—";
  return room.isOccupied ? `Belegt · ${base}` : base;
}

export function RoomsMasterDetail({
  groupDefinitions,
  selectedId,
  selectedRoom,
  onSelect,
  onSaveRoom,
  onDeleteClick,
}: RoomsMasterDetailProps) {
  const renderItem = (room: Room) => (
    <DatabaseListItem
      title={room.name}
      subtitle={
        <span
          className={
            room.isOccupied ? "text-moto-orange font-medium" : undefined
          }
        >
          {buildRoomSubtitle(room)}
        </span>
      }
      isSelected={selectedId === room.id}
      onSelect={() => onSelect(room.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForRoom}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Keine Räume gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedRoom ? (
    <RoomDetailContent
      room={selectedRoom}
      onSaveRoom={onSaveRoom}
      onDeleteClick={onDeleteClick}
    />
  ) : (
    <EmptyDetailState
      title="Keinen Raum ausgewählt"
      description="Wähle links einen Raum, um Stammdaten direkt im Detailbereich zu bearbeiten."
    />
  );

  return (
    <MasterDetailLayout
      list={listNode}
      detail={detailNode}
      selectedId={selectedId}
      onDeselect={() => onSelect(null)}
      unselectedBehavior="expand"
      mobileDrawerTitle={selectedRoom?.name ?? "Raum"}
    />
  );
}

interface RoomDetailContentProps {
  room: Room;
  onSaveRoom: (data: Partial<Room>) => Promise<void>;
  onDeleteClick: () => void;
}

function RoomDetailContent({
  room,
  onSaveRoom,
  onDeleteClick,
}: RoomDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const [formResetCounter, setFormResetCounter] = useState(0);
  const isSystem = isSystemRoom(room);

  const handleSaveRoom = async (data: Partial<Room>) => {
    await onSaveRoom(data);
    setFormResetCounter((n) => n + 1);
  };

  const headerActions = !isSystem ? (
    <DetailDeleteButton onClick={onDeleteClick} />
  ) : null;

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <RoomStammdatenTab
          room={room}
          onSaveRoom={handleSaveRoom}
          onResetForm={() => setFormResetCounter((n) => n + 1)}
          formResetKey={`${room.id}:${formResetCounter}`}
        />
      ),
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.rooms.icon}
              tone={MOTO_CONCEPTS.rooms.tone}
              size={36}
            />
          }
          title={room.name}
          subtitle={buildRoomLocation(room)}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

interface RoomStammdatenTabProps {
  room: Room;
  onSaveRoom: (data: Partial<Room>) => Promise<void>;
  onResetForm: () => void;
  formResetKey: string;
}

function RoomStammdatenTab({
  room,
  onSaveRoom,
  onResetForm,
  formResetKey,
}: RoomStammdatenTabProps) {
  const sections = useMemo(() => buildRoomFormSections(room), [room]);

  return (
    <div className="space-y-6">
      <RoomStatusSummary room={room} />
      <DatabaseForm
        key={formResetKey}
        theme={roomsConfig.theme}
        sections={sections}
        initialData={room}
        onSubmit={onSaveRoom}
        onCancel={onResetForm}
        submitLabel="Speichern"
        stickyActions
      />
    </div>
  );
}

function RoomStatusSummary({ room }: { room: Room }) {
  const isOccupied = room.isOccupied;

  return (
    <section className="moto-content-surface rounded-lg border p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span
          className={
            isOccupied
              ? "bg-moto-orange/15 text-moto-orange rounded-full px-2.5 py-1 text-xs font-medium"
              : "bg-moto-green/15 text-moto-green rounded-full px-2.5 py-1 text-xs font-medium"
          }
        >
          {isOccupied ? "Belegt" : "Frei"}
        </span>
        {room.capacity !== undefined ? (
          <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            {room.capacity} Plätze
          </span>
        ) : null}
        {isSystemRoom(room) ? (
          <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
            Systemraum
          </span>
        ) : null}
      </div>

      {room.activityName || room.groupName ? (
        <dl className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
          {room.activityName ? (
            <div>
              <dt className="text-xs font-medium text-gray-500">Aktivität</dt>
              <dd className="mt-0.5 text-sm text-gray-900">
                {room.activityName}
              </dd>
            </div>
          ) : null}
          {room.groupName ? (
            <div>
              <dt className="text-xs font-medium text-gray-500">Gruppe</dt>
              <dd className="mt-0.5 text-sm text-gray-900">{room.groupName}</dd>
            </div>
          ) : null}
        </dl>
      ) : null}
    </section>
  );
}
