"use client";

import { Plus, Search } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { FormEvent, KeyboardEvent } from "react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { activityService } from "~/lib/activity-service";
import type { Activity } from "~/lib/activity-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchPlannerRooms,
  type PlannerRoomReference,
} from "~/lib/planner-reference-api";
import { staffService, type Staff } from "~/lib/staff-api";

const logger = createLogger({ component: "SpontaneousActivityStart" });
const EMPTY_OCCUPIED_ROOM_IDS: readonly string[] = [];

interface RoomOption {
  id: string;
  name: string;
  building?: string | null;
}

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

function normalizeRooms(rooms: PlannerRoomReference[]): RoomOption[] {
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

function consumeListboxEscape(event: KeyboardEvent): void {
  // Keep Escape from reaching FormModal's document-level handler (which would
  // close the whole modal and wipe the draft). stopImmediatePropagation is
  // required because that listener sits on the same document node as React's
  // delegated listener, where plain stopPropagation would not stop it.
  event.preventDefault();
  event.nativeEvent.stopImmediatePropagation();
}

function isRoomOccupied(
  room: RoomOption,
  occupiedRoomIds: ReadonlySet<string>,
): boolean {
  return occupiedRoomIds.has(room.id);
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
  const [roomId, setRoomId] = useState("");
  const [additionalStaffIds, setAdditionalStaffIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [activityMenuOpen, setActivityMenuOpen] = useState(false);
  const [activeActivityIndex, setActiveActivityIndex] = useState(0);
  const activityFieldRef = useRef<HTMLDivElement>(null);
  const occupiedRoomIdSet = useMemo(
    () => new Set(occupiedRoomIds),
    [occupiedRoomIds],
  );

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setIsLoadingRefs(true);
    setError(null);

    void Promise.all([
      activityService.getActivities().catch((err: unknown) => {
        logger.error("spontaneous_activities_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return [] as Activity[];
      }),
      fetchPlannerRooms()
        .then(normalizeRooms)
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
        if (cancelled) return;
        // The staff room list includes Schulhof as a regular destination
        // (#2161) while keeping WC infrastructure hidden.
        const spontaneousRooms = roomData;
        setActivities(
          [...activityData].sort((a, b) => a.name.localeCompare(b.name, "de")),
        );
        setRooms(spontaneousRooms);
        setStaff(
          staffData
            .filter((item) => item.id !== currentStaffId)
            .sort((a, b) => staffLabel(a).localeCompare(staffLabel(b), "de")),
        );
        setRoomId((prev) => {
          const firstAvailableRoom =
            spontaneousRooms.find(
              (room) => !isRoomOccupied(room, occupiedRoomIdSet),
            ) ?? null;
          const previousRoom = spontaneousRooms.find(
            (room) => room.id === prev,
          );
          if (
            previousRoom &&
            !isRoomOccupied(previousRoom, occupiedRoomIdSet)
          ) {
            return prev;
          }
          const configuredDefaultRoom = spontaneousRooms.find(
            (room) => room.id === defaultRoomId,
          );
          if (
            configuredDefaultRoom &&
            !isRoomOccupied(configuredDefaultRoom, occupiedRoomIdSet)
          ) {
            return configuredDefaultRoom.id;
          }
          return firstAvailableRoom?.id ?? "";
        });
      })
      .finally(() => {
        if (!cancelled) setIsLoadingRefs(false);
      });
    return () => {
      cancelled = true;
    };
  }, [currentStaffId, defaultRoomId, isOpen, occupiedRoomIdSet]);

  // Close the activity suggestions when clicking outside the field.
  useEffect(() => {
    if (!activityMenuOpen) return;
    const handlePointerDown = (event: MouseEvent) => {
      if (
        activityFieldRef.current &&
        !activityFieldRef.current.contains(event.target as Node)
      ) {
        setActivityMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [activityMenuOpen]);

  const selectedActivity = useMemo(
    () => findSelectedActivity(activities, activityInput),
    [activities, activityInput],
  );
  const title = selectedActivity?.name ?? activityInput.trim();
  const selectedRoom = rooms.find((room) => room.id === roomId);
  const isSelectedRoomOccupied =
    selectedRoom !== undefined &&
    isRoomOccupied(selectedRoom, occupiedRoomIdSet);
  const canSubmit =
    title.length > 0 &&
    roomId.length > 0 &&
    !isSelectedRoomOccupied &&
    !isStarting;
  const suggestedActivities = activities.slice(0, 5);
  const filteredActivities = useMemo(() => {
    const query = activityInput.trim().toLocaleLowerCase("de");
    const matches = query
      ? activities.filter((activity) =>
          activity.name.toLocaleLowerCase("de").includes(query),
        )
      : activities;
    return matches.slice(0, 8);
  }, [activities, activityInput]);
  const activityListboxOpen = activityMenuOpen && filteredActivities.length > 0;
  const roomOptions = useMemo(
    () =>
      rooms.map((room) => ({
        value: room.id,
        label: `${room.building ? `${room.building} - ` : ""}${room.name}${
          isRoomOccupied(room, occupiedRoomIdSet) ? " (belegt)" : ""
        }`,
        disabled: isRoomOccupied(room, occupiedRoomIdSet),
      })),
    [rooms, occupiedRoomIdSet],
  );

  useEffect(() => {
    if (!activityListboxOpen) {
      setActiveActivityIndex(0);
      return;
    }
    setActiveActivityIndex((prev) =>
      Math.min(prev, filteredActivities.length - 1),
    );
  }, [activityListboxOpen, filteredActivities.length]);

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
    setRoomId("");
    setAdditionalStaffIds([]);
    setError(null);
    setActivityMenuOpen(false);
    setActiveActivityIndex(0);
  }

  function selectActivitySuggestion(activity: Activity) {
    setActivityInput(activity.name);
    setActivityMenuOpen(false);
    setActiveActivityIndex(0);
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
          className="border-moto-green/40 bg-moto-green/10 hover:border-moto-green hover:bg-moto-green/15 flex min-h-14 w-full items-center gap-3 rounded-lg border px-3 py-2 text-left transition disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="bg-moto-green flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-gray-950">
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

      <FormModal
        isOpen={isOpen}
        onClose={resetAndClose}
        title="Spontane Aktivität"
        size="md"
        mobilePosition="center"
        footer={
          <>
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={resetAndClose}
            >
              Abbrechen
            </Button>
            <Button
              type="submit"
              form="spontaneous-activity-form"
              variant="primary"
              size="md"
              isLoading={isStarting}
              loadingText="Startet ..."
              disabled={!canSubmit || isLoadingRefs}
            >
              Aktivität starten
            </Button>
          </>
        }
      >
        <form
          id="spontaneous-activity-form"
          className="space-y-4"
          onSubmit={handleSubmit}
        >
          {error ? <Alert type="error" message={error} /> : null}

          <div
            ref={activityFieldRef}
            onBlur={(event) => {
              const nextFocusedNode = event.relatedTarget;
              if (
                !(nextFocusedNode instanceof Node) ||
                !event.currentTarget.contains(nextFocusedNode)
              ) {
                setActivityMenuOpen(false);
              }
            }}
          >
            <label
              htmlFor="activity"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Aktivität
            </label>
            <div className="relative">
              <Search
                className="pointer-events-none absolute top-1/2 left-3 z-10 h-4 w-4 -translate-y-1/2 text-gray-400"
                aria-hidden="true"
              />
              <Input
                name="activity"
                controlSize="compact"
                className="pl-9"
                value={activityInput}
                onChange={(event) => {
                  setActivityInput(event.target.value);
                  setActivityMenuOpen(true);
                  setActiveActivityIndex(0);
                }}
                onFocus={() => {
                  setActivityMenuOpen(true);
                  setActiveActivityIndex(0);
                }}
                onKeyDown={(event) => {
                  if (event.key === "Escape" && activityListboxOpen) {
                    consumeListboxEscape(event);
                    setActivityMenuOpen(false);
                    return;
                  }
                  if (!activityListboxOpen) return;

                  if (event.key === "ArrowDown") {
                    event.preventDefault();
                    setActiveActivityIndex(
                      (prev) => (prev + 1) % filteredActivities.length,
                    );
                    return;
                  }
                  if (event.key === "ArrowUp") {
                    event.preventDefault();
                    setActiveActivityIndex(
                      (prev) =>
                        (prev - 1 + filteredActivities.length) %
                        filteredActivities.length,
                    );
                    return;
                  }
                  if (event.key === "Home") {
                    event.preventDefault();
                    setActiveActivityIndex(0);
                    return;
                  }
                  if (event.key === "End") {
                    event.preventDefault();
                    setActiveActivityIndex(filteredActivities.length - 1);
                    return;
                  }
                  if (event.key === "Enter") {
                    const activeActivity =
                      filteredActivities[activeActivityIndex];
                    if (activeActivity) {
                      event.preventDefault();
                      selectActivitySuggestion(activeActivity);
                    }
                  }
                }}
                placeholder="Aktivität suchen oder neu eingeben"
                autoComplete="off"
                role="combobox"
                tabIndex={0}
                aria-expanded={activityListboxOpen}
                aria-controls="spontaneous-activity-listbox"
                aria-activedescendant={
                  activityListboxOpen
                    ? `spontaneous-activity-option-${filteredActivities[activeActivityIndex]?.id}`
                    : undefined
                }
                aria-autocomplete="list"
                required
              />
              {activityListboxOpen ? (
                <ul
                  id="spontaneous-activity-listbox"
                  role="listbox"
                  className="absolute top-full left-0 z-50 mt-1 max-h-60 w-full overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg"
                >
                  {filteredActivities.map((activity, index) => (
                    <li key={activity.id} role="presentation">
                      <button
                        id={`spontaneous-activity-option-${activity.id}`}
                        type="button"
                        role="option"
                        // Keep options out of the tab order: focus stays on the
                        // combobox input (aria-activedescendant drives the active
                        // option). Otherwise Tab lands on an option button, where
                        // Escape would bubble to FormModal's document listener and
                        // close the whole modal instead of just the listbox.
                        tabIndex={-1}
                        aria-selected={index === activeActivityIndex}
                        onMouseEnter={() => setActiveActivityIndex(index)}
                        // Select on pointer-down and keep focus on the input.
                        // Safari/iOS does not focus a clicked button, so the
                        // input would blur with relatedTarget === null, the
                        // blur handler would close the menu, and the option
                        // would unmount before its click fired — making pointer
                        // selection unreliable on touch. preventDefault stops
                        // the focus shift (no blur, menu stays open); we run the
                        // selection here so a missed onClick can't drop it.
                        onPointerDown={(event) => {
                          event.preventDefault();
                          selectActivitySuggestion(activity);
                        }}
                        className="flex w-full items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50 aria-selected:bg-gray-100 aria-selected:text-gray-900"
                      >
                        {activity.name}
                      </button>
                    </li>
                  ))}
                </ul>
              ) : null}
            </div>
          </div>

          {suggestedActivities.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {suggestedActivities.map((activity) => (
                <button
                  key={activity.id}
                  type="button"
                  onClick={() => setActivityInput(activity.name)}
                  className="hover:border-moto-blue hover:text-moto-blue-strong rounded-full border border-gray-200 bg-gray-50 px-3 py-1.5 text-sm text-gray-700"
                >
                  {activity.name}
                </button>
              ))}
            </div>
          ) : null}

          <div>
            <span className="mb-2 block text-sm font-medium text-gray-700">
              Raum
            </span>
            <CustomSelect
              value={roomId}
              options={roomOptions}
              onChange={setRoomId}
              ariaLabel="Raum"
              disabled={isLoadingRefs}
              required
              invalid={isSelectedRoomOccupied}
              placeholder={isLoadingRefs ? "Lade Räume ..." : "Raum auswählen"}
            />
            {isSelectedRoomOccupied ? (
              <span className="text-moto-red-hover mt-1 block text-xs">
                Dieser Raum ist bereits belegt.
              </span>
            ) : null}
          </div>

          {staff.length > 0 ? (
            <div>
              <div className="mb-2 flex items-center gap-2 text-sm font-medium text-gray-700">
                <MotoConceptIcon concept="staff" size={16} />
                Weitere Betreuer
              </div>
              <div className="grid max-h-44 gap-2 overflow-y-auto rounded-lg border border-gray-200 bg-gray-50 p-2">
                {staff.map((item) => (
                  <label
                    key={item.id}
                    className="flex min-h-10 cursor-pointer items-center gap-3 rounded-md bg-white px-3 py-2 text-sm text-gray-800"
                  >
                    <Checkbox
                      checked={additionalStaffIds.includes(item.id)}
                      onChange={() => toggleStaff(item.id)}
                    />
                    <span className="truncate">{staffLabel(item)}</span>
                  </label>
                ))}
              </div>
            </div>
          ) : null}
        </form>
      </FormModal>
    </>
  );
}
