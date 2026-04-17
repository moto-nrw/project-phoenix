"use client";

import React, { useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { useModalAnimation } from "~/hooks/useModalAnimation";
import { useModalBlurEffect } from "~/hooks/useModalBlurEffect";
import { useScrollLock } from "~/hooks/useScrollLock";
import {
  ModalWrapper,
  getContentAnimationClassName,
  renderButtonSpinner,
  renderModalCloseButton,
  renderModalErrorAlert,
  scrollableContentClassName,
} from "~/components/ui/modal-utils";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { fetchActivities } from "~/lib/activity-api";
import type { Activity, ActivitySupervisor } from "~/lib/activity-helpers";
import { staffService, type Staff } from "~/lib/staff-api";
import { fetchRooms } from "~/lib/rooms-api";
import type { Room } from "~/lib/rooms-helpers";
import {
  activeService,
  SessionConflictError,
  type SessionConflictInfo,
  type StartSessionResult,
} from "~/lib/active-service";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StartSupervisionModal" });

const PRIMARY_COLOR = LOCATION_COLORS.GROUP_ROOM; // #83CD2D — MOTO green, semantic "start/active"
const ERROR_COLOR = LOCATION_COLORS.HOME; // #FF3130 — MOTO red, for override CTA

interface StartSupervisionModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  /** Optional activity pre-selection — skips the activity picker. */
  readonly preselectedActivity?: Activity | null;
  /** Called after a session starts successfully. */
  readonly onStarted?: (result: StartSessionResult) => void;
}

export function StartSupervisionModal({
  isOpen,
  onClose,
  preselectedActivity = null,
  onStarted,
}: StartSupervisionModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  useScrollLock(isOpen);
  useModalBlurEffect(isOpen);
  const { isAnimating, isExiting, handleClose } = useModalAnimation(
    isOpen,
    onClose,
  );

  const [activities, setActivities] = useState<Activity[]>([]);
  const [staff, setStaff] = useState<Staff[]>([]);
  const [rooms, setRooms] = useState<Room[]>([]);
  const [loadingLookups, setLoadingLookups] = useState(false);

  const [activityId, setActivityId] = useState<string>(
    preselectedActivity?.id ?? "",
  );
  const [supervisorIds, setSupervisorIds] = useState<string[]>([]);
  const [roomId, setRoomId] = useState<string>("");

  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<SessionConflictInfo | null>(null);

  // Reset state whenever the modal opens / preselection changes.
  useEffect(() => {
    if (!isOpen) return;
    setActivityId(preselectedActivity?.id ?? "");
    setError(null);
    setConflict(null);
  }, [isOpen, preselectedActivity?.id]);

  // Fetch activities, staff, and rooms when the modal opens.
  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;

    const load = async () => {
      setLoadingLookups(true);
      try {
        const [activitiesData, staffData, roomsData] = await Promise.all([
          fetchActivities(),
          staffService.getAllStaff(),
          fetchRooms(),
        ]);
        if (cancelled) return;
        setActivities(activitiesData);
        setStaff(staffData);
        setRooms(roomsData);
      } catch (err) {
        if (cancelled) return;
        logger.error("lookup_fetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError("Daten konnten nicht geladen werden. Bitte erneut versuchen.");
      } finally {
        if (!cancelled) setLoadingLookups(false);
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  const selectedActivity = useMemo<Activity | null>(() => {
    if (preselectedActivity && preselectedActivity.id === activityId) {
      return preselectedActivity;
    }
    return activities.find((a) => a.id === activityId) ?? null;
  }, [preselectedActivity, activityId, activities]);

  // Pre-fill supervisors + room whenever the selected activity changes.
  useEffect(() => {
    if (!selectedActivity) {
      setSupervisorIds([]);
      setRoomId("");
      return;
    }
    const configured = extractSupervisorIds(selectedActivity);
    setSupervisorIds(configured);
    setRoomId(selectedActivity.planned_room_id ?? "");
  }, [selectedActivity]);

  const canSubmit =
    !submitting && activityId.length > 0 && supervisorIds.length > 0;

  const submit = useCallback(
    async (force = false) => {
      if (!canSubmit && !force) return;
      setError(null);
      setSubmitting(true);
      try {
        const result = await activeService.startSession({
          activityId,
          supervisorIds,
          roomId: roomId.length > 0 ? roomId : undefined,
          force,
        });
        toastSuccess("Aufsicht gestartet");
        onStarted?.(result);
        handleClose();
      } catch (err) {
        if (err instanceof SessionConflictError) {
          setConflict(err.conflict);
          return;
        }
        logger.error("start_session_failed", {
          activity_id: activityId,
          error: err instanceof Error ? err.message : String(err),
        });
        const message =
          err instanceof Error ? err.message : "Start fehlgeschlagen";
        setError(message);
        toastError(message);
      } finally {
        setSubmitting(false);
      }
    },
    [
      activityId,
      canSubmit,
      handleClose,
      onStarted,
      roomId,
      supervisorIds,
      toastError,
      toastSuccess,
    ],
  );

  const confirmOverride = useCallback(async () => {
    setConflict(null);
    await submit(true);
  }, [submit]);

  if (!isOpen && !isExiting) return null;
  if (globalThis.window === undefined) return null;

  return createPortal(
    <ModalWrapper
      onClose={handleClose}
      isAnimating={isAnimating}
      isExiting={isExiting}
    >
      <div className={scrollableContentClassName}>
        <div className={getContentAnimationClassName(isAnimating, isExiting)}>
          <div className="absolute top-3 right-3">
            {renderModalCloseButton({ onClose: handleClose })}
          </div>
          <h2 className="mb-1 text-xl font-semibold text-gray-900">
            Aufsicht starten
          </h2>
          <p className="mb-5 text-sm text-gray-500">
            Wähle Aktivität, Betreuer und Raum, um eine aktive Sitzung zu
            starten.
          </p>

          {error && renderModalErrorAlert({ message: error })}

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void submit(false);
            }}
            className="space-y-5"
          >
            <ActivityField
              value={activityId}
              onChange={setActivityId}
              activities={activities}
              preselected={preselectedActivity}
              disabled={loadingLookups || submitting}
            />

            <SupervisorsField
              values={supervisorIds}
              onChange={setSupervisorIds}
              staff={staff}
              disabled={loadingLookups || submitting}
            />

            <RoomField
              value={roomId}
              onChange={setRoomId}
              rooms={rooms}
              disabled={loadingLookups || submitting}
            />

            <div className="flex items-center justify-end gap-3 pt-2">
              <button
                type="button"
                onClick={handleClose}
                disabled={submitting}
                className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition hover:bg-gray-50 disabled:opacity-50"
              >
                Abbrechen
              </button>
              <button
                type="submit"
                disabled={!canSubmit}
                className="inline-flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold text-white shadow-sm transition disabled:cursor-not-allowed disabled:opacity-50"
                style={{ backgroundColor: PRIMARY_COLOR }}
              >
                {submitting && renderButtonSpinner()}
                Aufsicht starten
              </button>
            </div>
          </form>
        </div>
      </div>

      {conflict && (
        <ConflictOverlay
          info={conflict}
          submitting={submitting}
          onCancel={() => setConflict(null)}
          onConfirm={confirmOverride}
        />
      )}
    </ModalWrapper>,
    document.body,
  );
}

function extractSupervisorIds(activity: Activity): string[] {
  const list: string[] = [];
  if (activity.supervisors && activity.supervisors.length > 0) {
    for (const sup of activity.supervisors as ActivitySupervisor[]) {
      if (sup.staff_id && !list.includes(sup.staff_id)) {
        list.push(sup.staff_id);
      }
    }
  }
  if (
    activity.supervisor_id &&
    activity.supervisor_id.length > 0 &&
    !list.includes(activity.supervisor_id)
  ) {
    list.push(activity.supervisor_id);
  }
  return list;
}

interface ActivityFieldProps {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly activities: Activity[];
  readonly preselected: Activity | null;
  readonly disabled: boolean;
}

function ActivityField({
  value,
  onChange,
  activities,
  preselected,
  disabled,
}: ActivityFieldProps) {
  if (preselected && preselected.id === value) {
    return (
      <div>
        <FieldLabel>Aktivität</FieldLabel>
        <div className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-2 text-sm font-medium text-gray-900">
          {preselected.name}
        </div>
      </div>
    );
  }

  return (
    <div>
      <FieldLabel>Aktivität</FieldLabel>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none disabled:opacity-50"
      >
        <option value="">Aktivität auswählen …</option>
        {activities.map((a) => (
          <option key={a.id} value={a.id}>
            {a.name}
          </option>
        ))}
      </select>
    </div>
  );
}

interface SupervisorsFieldProps {
  readonly values: string[];
  readonly onChange: (values: string[]) => void;
  readonly staff: Staff[];
  readonly disabled: boolean;
}

function SupervisorsField({
  values,
  onChange,
  staff,
  disabled,
}: SupervisorsFieldProps) {
  const toggle = (id: string) => {
    if (values.includes(id)) {
      onChange(values.filter((v) => v !== id));
    } else {
      onChange([...values, id]);
    }
  };

  const sortedStaff = useMemo(
    () => [...staff].sort((a, b) => a.name.localeCompare(b.name, "de-DE")),
    [staff],
  );

  return (
    <div>
      <FieldLabel>
        Betreuer
        <span className="ml-1 text-xs font-normal text-gray-500">
          ({values.length} ausgewählt)
        </span>
      </FieldLabel>
      <div className="max-h-48 overflow-y-auto rounded-lg border border-gray-200 bg-white p-2">
        {sortedStaff.length === 0 ? (
          <p className="px-2 py-3 text-sm text-gray-500">
            Keine Mitarbeiter gefunden.
          </p>
        ) : (
          <ul className="space-y-1">
            {sortedStaff.map((member) => {
              const checked = values.includes(member.id);
              return (
                <li key={member.id}>
                  <label
                    className={`flex cursor-pointer items-center gap-3 rounded-md px-2 py-1.5 text-sm transition ${
                      checked ? "bg-[#83CD2D]/10" : "hover:bg-gray-50"
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={checked}
                      onChange={() => toggle(member.id)}
                      disabled={disabled}
                      className="h-4 w-4 rounded border-gray-300 text-[#83CD2D] focus:ring-[#83CD2D]"
                    />
                    <span className="flex-1 truncate text-gray-900">
                      {member.name}
                    </span>
                    {member.isSupervising && (
                      <span className="rounded-full bg-[#F78C10]/10 px-2 py-0.5 text-xs font-medium text-[#F78C10]">
                        beschäftigt
                      </span>
                    )}
                  </label>
                </li>
              );
            })}
          </ul>
        )}
      </div>
    </div>
  );
}

interface RoomFieldProps {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly rooms: Room[];
  readonly disabled: boolean;
}

function RoomField({ value, onChange, rooms, disabled }: RoomFieldProps) {
  const sortedRooms = useMemo(
    () => [...rooms].sort((a, b) => a.name.localeCompare(b.name, "de-DE")),
    [rooms],
  );
  return (
    <div>
      <FieldLabel>Raum</FieldLabel>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none disabled:opacity-50"
      >
        <option value="">Geplanten Raum der Aktivität verwenden</option>
        {sortedRooms.map((r) => (
          <option key={r.id} value={r.id}>
            {r.name}
          </option>
        ))}
      </select>
    </div>
  );
}

function FieldLabel({ children }: { readonly children: React.ReactNode }) {
  return (
    <label className="mb-1 block text-sm font-medium text-gray-700">
      {children}
    </label>
  );
}

interface ConflictOverlayProps {
  readonly info: SessionConflictInfo;
  readonly submitting: boolean;
  readonly onCancel: () => void;
  readonly onConfirm: () => void;
}

function ConflictOverlay({
  info,
  submitting,
  onCancel,
  onConfirm,
}: ConflictOverlayProps) {
  return (
    <div className="absolute inset-0 flex items-center justify-center bg-black/30 p-4">
      <div className="w-full max-w-sm rounded-xl bg-white p-5 shadow-lg">
        <h3 className="mb-2 text-lg font-semibold text-gray-900">
          Aktivität läuft bereits
        </h3>
        <p className="mb-4 text-sm text-gray-700">
          {info.message}. Soll die bestehende Aufsicht beendet und eine neue
          gestartet werden?
        </p>
        <div className="flex items-center justify-end gap-3">
          <button
            type="button"
            onClick={onCancel}
            disabled={submitting}
            className="rounded-lg border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={submitting || !info.canOverride}
            className="inline-flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-semibold text-white shadow-sm disabled:opacity-50"
            style={{ backgroundColor: ERROR_COLOR }}
          >
            {submitting && renderButtonSpinner()}
            Trotzdem starten
          </button>
        </div>
      </div>
    </div>
  );
}
