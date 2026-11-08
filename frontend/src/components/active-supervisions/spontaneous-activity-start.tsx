"use client";

import { Plus, Search } from "lucide-react";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { InsetPanel } from "~/components/ui/inset-panel";
import type { FormEvent } from "react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { CustomSelect } from "~/components/ui/custom-select";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { SuggestInput } from "~/components/ui/suggest-input";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { activityService } from "~/lib/activity-service";
import type { Activity } from "~/lib/activity-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchPlannerRooms,
  type PlannerRoomReference,
} from "~/lib/planner-reference-api";
import { staffService, type Staff } from "~/lib/staff-api";
import { externalAccountMark } from "~/lib/staff-helpers";
import { ExternalBadge } from "~/components/staff/external-badge";
import { StatusBadge } from "~/components/ui/status-badge";

import { ExternalCaregiverEntry } from "~/components/staff/external-caregiver-entry";
import { staffConcept } from "~/lib/concepts/staff";
import { IconTile } from "~/components/ui/icon-tile";
import { ChoiceTile } from "~/components/ui/choice-tile";

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
  readonly canCreateExternalCaregiver?: boolean;
  /** Offer „Nur ansehen“ in the external entry (users:manage, ADR 0091). */
  readonly canGrantExternalView?: boolean;
  readonly currentStaffId?: string;
  readonly defaultRoomId?: string;
  readonly disabled?: boolean;
  readonly disabledReason?: string;
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

function isRoomOccupied(
  room: RoomOption,
  occupiedRoomIds: ReadonlySet<string>,
): boolean {
  return occupiedRoomIds.has(room.id);
}

export function SpontaneousActivityStart({
  canCreateExternalCaregiver = false,
  canGrantExternalView = false,
  currentStaffId,
  defaultRoomId,
  disabled = false,
  disabledReason,
  isStarting = false,
  occupiedRoomIds = EMPTY_OCCUPIED_ROOM_IDS,
  onStart,
}: SpontaneousActivityStartProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [isLoadingRefs, setIsLoadingRefs] = useState(false);
  const [activities, setActivities] = useState<Activity[]>([]);
  const [rooms, setRooms] = useState<RoomOption[]>([]);
  const [staff, setStaff] = useState<Staff[]>([]);
  // Without the staff list the section has nobody to offer, so it stays
  // hidden after a failed load; the load error in the dialog says why.
  const [staffLoaded, setStaffLoaded] = useState(false);
  const [activityInput, setActivityInput] = useState("");
  const [roomId, setRoomId] = useState("");
  const [additionalStaffIds, setAdditionalStaffIds] = useState<string[]>([]);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { clear: clearFormErrors } = formErrors;
  // Auswahllisten (Aktivitäten, Räume, Personal): ein Fehlschlag steht im
  // Dialog, „Wiederholen“ lädt alle drei neu.
  const {
    error: loadError,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const [loadAttempt, setLoadAttempt] = useState(0);
  const occupiedRoomIdSet = useMemo(
    () => new Set(occupiedRoomIds),
    [occupiedRoomIds],
  );
  // The dashboard poll hands over a new occupancy every few seconds. The
  // load below only needs it to preselect a free room, so it reads the latest
  // value instead of reloading on every poll: a reload hid the staff section
  // and discarded a half-typed external person. The room list itself marks
  // occupied rooms from the live value.
  const occupiedRoomIdSetRef = useRef(occupiedRoomIdSet);
  useEffect(() => {
    occupiedRoomIdSetRef.current = occupiedRoomIdSet;
  }, [occupiedRoomIdSet]);

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    setIsLoadingRefs(true);
    clearFormErrors();
    clearLoadError();
    // Jede Liste füllt, was sie bekommt; der erste Fehlschlag steht im
    // Dialog, statt still eine leere Auswahl zu zeigen.
    let firstFailure: unknown = null;
    const noteFailure = (event: string, err: unknown) => {
      logger.error(event, {
        error: err instanceof Error ? err.message : String(err),
      });
      firstFailure ??= err;
    };

    void Promise.all([
      activityService.getActivities().catch((err: unknown) => {
        noteFailure("spontaneous_activities_fetch_failed", err);
        return [] as Activity[];
      }),
      fetchPlannerRooms()
        .then(normalizeRooms)
        .catch((err: unknown) => {
          noteFailure("spontaneous_rooms_fetch_failed", err);
          return [] as RoomOption[];
        }),
      staffService
        .getAllStaff()
        .then((list) => ({ list, loaded: true }))
        .catch((err: unknown) => {
          noteFailure("spontaneous_staff_fetch_failed", err);
          return { list: [] as Staff[], loaded: false };
        }),
    ])
      .then(([activityData, roomData, staffData]) => {
        if (cancelled) return;
        if (firstFailure) {
          void showLoadError(firstFailure, {
            object: "die Auswahl für die spontane Aktivität",
            retry: () => setLoadAttempt((value) => value + 1),
          });
        }
        // The staff room list includes Schulhof as a regular destination
        // (#2161) while keeping WC infrastructure hidden.
        const spontaneousRooms = roomData;
        setActivities(
          [...activityData].sort((a, b) => a.name.localeCompare(b.name, "de")),
        );
        setRooms(spontaneousRooms);
        setStaffLoaded(staffData.loaded);
        setStaff(
          staffData.list
            .filter((item) => item.id !== currentStaffId)
            .sort((a, b) => staffLabel(a).localeCompare(staffLabel(b), "de")),
        );
        const occupied = occupiedRoomIdSetRef.current;
        setRoomId((prev) => {
          const firstAvailableRoom =
            spontaneousRooms.find((room) => !isRoomOccupied(room, occupied)) ??
            null;
          const previousRoom = spontaneousRooms.find(
            (room) => room.id === prev,
          );
          if (previousRoom && !isRoomOccupied(previousRoom, occupied)) {
            return prev;
          }
          const configuredDefaultRoom = spontaneousRooms.find(
            (room) => room.id === defaultRoomId,
          );
          if (
            configuredDefaultRoom &&
            !isRoomOccupied(configuredDefaultRoom, occupied)
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
  }, [
    currentStaffId,
    defaultRoomId,
    isOpen,
    loadAttempt,
    clearFormErrors,
    clearLoadError,
    showLoadError,
  ]);

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
  const activityNames = useMemo(
    () => activities.map((activity) => activity.name),
    [activities],
  );
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

  function toggleStaff(staffId: string) {
    setAdditionalStaffIds((prev) =>
      prev.includes(staffId)
        ? prev.filter((id) => id !== staffId)
        : [...prev, staffId],
    );
  }

  function addExternalCaregiver(saved: Staff) {
    // The new record carries its open invitation and is offered like
    // everyone else.
    setStaff((prev) =>
      [...prev.filter((item) => item.id !== saved.id), saved].sort((a, b) =>
        staffLabel(a).localeCompare(staffLabel(b), "de"),
      ),
    );
    setAdditionalStaffIds((prev) =>
      prev.includes(saved.id) ? prev : [...prev, saved.id],
    );
  }

  function resetAndClose() {
    setIsOpen(false);
    setActivityInput("");
    setRoomId("");
    setAdditionalStaffIds([]);
    clearFormErrors();
    clearLoadError();
  }

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (isSelectedRoomOccupied) {
      formErrors.invalid(
        "Der Raum ist schon belegt. Bitte wählen Sie einen anderen Raum.",
      );
      return;
    }
    if (!canSubmit) {
      formErrors.invalid(
        "Bitte wählen Sie eine Aktivität und einen Raum.",
        title.length > 0
          ? {}
          : { activity: "Bitte wählen oder schreiben Sie eine Aktivität." },
      );
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
          <IconTile
            shape="round"
            tone="greenFill"
            icon={<Plus aria-hidden="true" />}
          />
          <span className="min-w-0 flex-1">
            <span className="block text-sm font-semibold text-gray-900">
              Spontane Aktivität starten
            </span>
            <span className="block text-sm text-gray-600">
              {disabledReason ??
                "Aktivität wählen, Raum belegen, Kinder danach hinzufügen"}
            </span>
          </span>
        </button>
      </section>

      <FormModal
        isOpen={isOpen}
        onClose={resetAndClose}
        title="Spontane Aktivität"
        size="md"
        error={formErrors.error}
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
          ref={formRef}
          id="spontaneous-activity-form"
          className="space-y-4"
          onSubmit={handleSubmit}
        >
          <LoadErrorAlert error={loadError} />

          <SuggestInput
            name="activity"
            label="Aktivität"
            listLabel="Vorschläge für die Aktivität"
            error={formErrors.fieldError("activity")}
            controlSize="compact"
            leadingIcon={<Search />}
            value={activityInput}
            onValueChange={setActivityInput}
            suggestions={activityNames}
            maxSuggestions={8}
            highlightFirst
            placeholder="Aktivität suchen oder neu eingeben"
            required
          />

          {suggestedActivities.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {suggestedActivities.map((activity) => (
                <Button
                  key={activity.id}
                  type="button"
                  variant="surface"
                  size="compact"
                  onClick={() => setActivityInput(activity.name)}
                >
                  {activity.name}
                </Button>
              ))}
            </div>
          ) : null}

          <CustomSelect
            label="Raum"
            value={roomId}
            options={roomOptions}
            onChange={setRoomId}
            disabled={isLoadingRefs}
            required
            error={
              isSelectedRoomOccupied ? "Dieser Raum ist bereits belegt." : null
            }
            errorLive
            placeholder={isLoadingRefs ? "Lade Räume ..." : "Raum auswählen"}
          />

          {isLoadingRefs || !staffLoaded ? null : (
            <div className="space-y-2">
              <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
                <MotoConceptIcon concept={staffConcept} size={16} />
                Weitere Betreuer
              </div>
              {staff.length > 0 ? (
                <InsetPanel
                  framed
                  className="grid max-h-44 gap-2 overflow-y-auto"
                >
                  {staff.map((item) => {
                    const accountMark = item.isExternal
                      ? externalAccountMark(item.externalAccountStatus)
                      : null;
                    return (
                      <ChoiceTile
                        key={item.id}
                        selected={additionalStaffIds.includes(item.id)}
                        className="min-h-10 py-2 font-normal"
                      >
                        <Checkbox
                          checked={additionalStaffIds.includes(item.id)}
                          onChange={() => toggleStaff(item.id)}
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block break-words">
                            {staffLabel(item)}
                          </span>
                          {item.isExternal ? (
                            <span className="mt-1 flex flex-wrap gap-1">
                              <span className="max-w-full min-w-0 [overflow-wrap:anywhere] [&>span]:whitespace-normal">
                                <ExternalBadge
                                  organization={item.externalOrganization}
                                />
                              </span>
                              {accountMark ? (
                                <StatusBadge {...accountMark} compact />
                              ) : null}
                            </span>
                          ) : null}
                        </span>
                      </ChoiceTile>
                    );
                  })}
                </InsetPanel>
              ) : null}
              {canCreateExternalCaregiver ? (
                <ExternalCaregiverEntry
                  onAdded={addExternalCaregiver}
                  canGrantView={canGrantExternalView}
                />
              ) : null}
            </div>
          )}
        </form>
      </FormModal>
    </>
  );
}
