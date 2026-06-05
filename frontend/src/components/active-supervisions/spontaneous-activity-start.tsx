"use client";

import { Plus, Search, UserPlus } from "lucide-react";
import type { FormEvent } from "react";
import { useEffect, useMemo, useState } from "react";

import { Modal } from "~/components/ui/modal";
import { activityService } from "~/lib/activity-service";
import type { Activity } from "~/lib/activity-helpers";
import { createLogger } from "~/lib/logger";
import { staffService, type Staff } from "~/lib/staff-api";

const logger = createLogger({ component: "SpontaneousActivityStart" });
const EMPTY_OCCUPIED_ROOM_IDS: readonly string[] = [];

interface RoomOption {
  id: string;
  name: string;
  building?: string | null;
}

interface BackendRoomsEnvelope {
  data?: Array<{
    id: number | string;
    name?: string;
    room_name?: string;
    building?: string | null;
  }>;
}

type BackendRoomList =
  | BackendRoomsEnvelope
  | NonNullable<BackendRoomsEnvelope["data"]>;

export interface SpontaneousActivityStartPayload {
  title: string;
  roomId: string;
  activityGroupId?: string;
  additionalStaffIds: string[];
}

interface SpontaneousActivityStartProps {
  readonly currentStaffId?: string;
  readonly defaultRoomId?: string;
  readonly disabled?: boolean;
  readonly isStarting?: boolean;
  readonly occupiedRoomIds?: readonly string[];
  readonly onStart: (payload: SpontaneousActivityStartPayload) => void;
}

function normalizeRoomEnvelope(envelope: BackendRoomList): RoomOption[] {
  const rooms = Array.isArray(envelope) ? envelope : (envelope.data ?? []);
  return rooms
    .map((room) => ({
      id: String(room.id),
      name: room.name ?? room.room_name ?? `Raum ${room.id}`,
      building: room.building ?? null,
    }))
    .sort((a, b) => a.name.localeCompare(b.name, "de"));
}

function staffLabel(staff: Staff): string {
  return (
    staff.name || [staff.firstName, staff.lastName].filter(Boolean).join(" ")
  );
}

function findSelectedActivity(
  activities: Activity[],
  activityInput: string,
): Activity | undefined {
  const normalized = activityInput.trim().toLocaleLowerCase("de");
  if (!normalized) return undefined;
  return activities.find(
    (activity) => activity.name.trim().toLocaleLowerCase("de") === normalized,
  );
}

export function SpontaneousActivityStart({
  currentStaffId,
  defaultRoomId,
  disabled = false,
  isStarting = false,
  occupiedRoomIds = EMPTY_OCCUPIED_ROOM_IDS,
  onStart,
}: SpontaneousActivityStartProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [isLoadingRefs, setIsLoadingRefs] = useState(false);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [staff, setStaff] = useState<Staff[]>([]);
  const [activityInput, setActivityInput] = useState("");
  const [roomId, setRoomId] = useState(defaultRoomId ?? "");
  const [additionalStaffIds, setAdditionalStaffIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const occupiedRoomIdSet = useMemo(
    () => new Set(occupiedRoomIds),
    [occupiedRoomIds],
  );

  useEffect(() => {
    if (!isOpen) return;
    setRoomId((prev) => prev || defaultRoomId || "");
    setIsLoadingRefs(true);
    setError(null);

    void Promise.all([
      activityService.getActivities().catch((err: unknown) => {
        logger.error("spontaneous_activities_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return [] as Activity[];
      }),
      fetch("/api/rooms", { credentials: "include" })
        .then((response) => response.json() as Promise<BackendRoomsEnvelope>)
        .then(normalizeRoomEnvelope)
        .catch((err: unknown) => {
          logger.error("spontaneous_rooms_fetch_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [] as RoomOption[];
        }),
      staffService.getAllStaff().catch((err: unknown) => {
        logger.error("spontaneous_staff_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return [] as Staff[];
      }),
    ])
      .then(([activityData, roomData, staffData]) => {
        setActivities(
          [...activityData].sort((a, b) => a.name.localeCompare(b.name, "de")),
        );
        setRooms(roomData);
        setStaff(
          staffData
            .filter((item) => item.id !== currentStaffId)
            .sort((a, b) => staffLabel(a).localeCompare(staffLabel(b), "de")),
        );
        setRoomId((prev) => {
          const firstAvailableRoom =
            roomData.find((room) => !occupiedRoomIdSet.has(room.id)) ?? null;
          if (
            prev &&
            !occupiedRoomIdSet.has(prev) &&
            roomData.some((room) => room.id === prev)
          ) {
            return prev;
          }
          if (
            defaultRoomId &&
            !occupiedRoomIdSet.has(defaultRoomId) &&
            roomData.some((room) => room.id === defaultRoomId)
          ) {
            return defaultRoomId;
          }
          return firstAvailableRoom?.id ?? "";
        });
      })
      .finally(() => setIsLoadingRefs(false));
  }, [currentStaffId, defaultRoomId, isOpen, occupiedRoomIdSet]);

  const selectedActivity = useMemo(
    () => findSelectedActivity(activities, activityInput),
    [activities, activityInput],
  );
  const title = selectedActivity?.name ?? activityInput.trim();
  const isSelectedRoomOccupied =
    roomId.length > 0 && occupiedRoomIdSet.has(roomId);
  const canSubmit =
    title.length > 0 &&
    roomId.length > 0 &&
    !isSelectedRoomOccupied &&
    !isStarting;
  const suggestedActivities = activities.slice(0, 5);

  function toggleStaff(staffId: string) {
    setAdditionalStaffIds((prev) =>
      prev.includes(staffId)
        ? prev.filter((id) => id !== staffId)
        : [...prev, staffId],
    );
  }

  function resetAndClose() {
    setIsOpen(false);
    setActivityInput("");
    setAdditionalStaffIds([]);
    setError(null);
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (isSelectedRoomOccupied) {
      setError("Der Raum ist bereits belegt.");
      return;
    }
    if (!canSubmit) {
      setError("Aktivität und Raum sind erforderlich.");
      return;
    }
    onStart({
      title,
      roomId,
      activityGroupId: selectedActivity?.id,
      additionalStaffIds,
    });
    resetAndClose();
  }

  return (
    <>
      <section className="mb-4 rounded-lg border border-gray-200 bg-white p-3 shadow-sm sm:p-4">
        <button
          type="button"
          disabled={disabled || isStarting}
          onClick={() => setIsOpen(true)}
          className="flex min-h-14 w-full items-center gap-3 rounded-lg border border-[#83CD2D]/40 bg-[#83CD2D]/10 px-3 py-2 text-left transition hover:border-[#83CD2D] hover:bg-[#83CD2D]/15 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-[#83CD2D] text-white">
            <Plus className="h-5 w-5" aria-hidden="true" />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-sm font-semibold text-gray-900">
              Spontane Aktivität starten
            </span>
            <span className="block truncate text-sm text-gray-600">
              Aktivität wählen, Raum belegen, Kinder danach hinzufügen
            </span>
          </span>
        </button>
      </section>

      <Modal
        isOpen={isOpen}
        onClose={resetAndClose}
        title="Spontane Aktivität"
        widthClass="mx-3 w-[calc(100%-1.5rem)] max-w-xl"
      >
        <form className="space-y-4" onSubmit={handleSubmit}>
          {error ? (
            <div className="rounded-md border border-[#E5484D]/30 bg-[#E5484D]/10 px-3 py-2 text-sm text-[#A32020]">
              {error}
            </div>
          ) : null}

          <label className="block">
            <span className="mb-1 block text-xs font-semibold text-gray-700">
              Aktivität
            </span>
            <div className="relative">
              <Search
                className="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-gray-400"
                aria-hidden="true"
              />
              <input
                value={activityInput}
                onChange={(event) => setActivityInput(event.target.value)}
                list="spontaneous-activity-options"
                placeholder="Aktivität suchen oder neu eingeben"
                className="min-h-11 w-full rounded-md border border-gray-300 bg-white pr-3 pl-9 text-sm focus:border-[#5080D8] focus:ring-2 focus:ring-[#5080D8]/20 focus:outline-none"
                autoComplete="off"
                required
              />
            </div>
            <datalist id="spontaneous-activity-options">
              {activities.map((activity) => (
                <option key={activity.id} value={activity.name} />
              ))}
            </datalist>
          </label>

          {suggestedActivities.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {suggestedActivities.map((activity) => (
                <button
                  key={activity.id}
                  type="button"
                  onClick={() => setActivityInput(activity.name)}
                  className="rounded-full border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm text-gray-700 hover:border-[#5080D8] hover:text-[#315EA8]"
                >
                  {activity.name}
                </button>
              ))}
            </div>
          ) : null}

          <label className="block">
            <span className="mb-1 block text-xs font-semibold text-gray-700">
              Raum
            </span>
            <select
              value={roomId}
              onChange={(event) => setRoomId(event.target.value)}
              disabled={isLoadingRefs}
              className="min-h-11 w-full rounded-md border border-gray-300 bg-white px-3 text-sm focus:border-[#5080D8] focus:ring-2 focus:ring-[#5080D8]/20 focus:outline-none disabled:bg-gray-100"
              required
            >
              <option value="">
                {isLoadingRefs ? "Lade Räume ..." : "Raum auswählen"}
              </option>
              {rooms.map((room) => (
                <option
                  key={room.id}
                  value={room.id}
                  disabled={occupiedRoomIdSet.has(room.id)}
                >
                  {room.building
                    ? `${room.building} - ${room.name}`
                    : room.name}
                  {occupiedRoomIdSet.has(room.id) ? " (belegt)" : ""}
                </option>
              ))}
            </select>
            {isSelectedRoomOccupied ? (
              <span className="mt-1 block text-xs text-[#A32020]">
                Dieser Raum ist bereits belegt.
              </span>
            ) : null}
          </label>

          {staff.length > 0 ? (
            <div>
              <div className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-700">
                <UserPlus className="h-4 w-4" aria-hidden="true" />
                Weitere Betreuer
              </div>
              <div className="grid max-h-44 gap-2 overflow-y-auto rounded-md border border-gray-200 bg-gray-50 p-2">
                {staff.map((item) => (
                  <label
                    key={item.id}
                    className="flex min-h-10 items-center gap-3 rounded-md bg-white px-3 py-2 text-sm text-gray-800"
                  >
                    <input
                      type="checkbox"
                      checked={additionalStaffIds.includes(item.id)}
                      onChange={() => toggleStaff(item.id)}
                      className="h-4 w-4 rounded border-gray-300 text-[#83CD2D] focus:ring-[#83CD2D]"
                    />
                    <span className="truncate">{staffLabel(item)}</span>
                  </label>
                ))}
              </div>
            </div>
          ) : null}

          <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              onClick={resetAndClose}
              className="min-h-11 rounded-md border border-gray-300 px-4 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="submit"
              disabled={!canSubmit || isLoadingRefs}
              className="min-h-11 rounded-md bg-[#83CD2D] px-4 text-sm font-semibold text-white hover:bg-[#6FB624] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isStarting ? "Startet ..." : "Aktivität starten"}
            </button>
          </div>
        </form>
      </Modal>
    </>
  );
}
