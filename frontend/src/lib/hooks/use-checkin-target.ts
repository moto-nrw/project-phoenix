"use client";

// Where a check-in in the app puts the children (#6953): "Anwesend (ohne
// Raum)" as before, or straight into a room or a running activity. The
// choice stays until the person changes it, also across a reload on the same
// device, so a whole group can be checked in with one tap per child while a
// terminal is offline.

import { useCallback, useEffect, useMemo, useState } from "react";
import { useSession } from "next-auth/react";

import type { ActiveGroup, Supervisor } from "~/lib/active-helpers";
import { activeService } from "~/lib/active-service";
import { fetchActivities } from "~/lib/activity-api";
import type { Activity } from "~/lib/activity-helpers";
import {
  buildTargetRoomOptions,
  canUseAllMoveTargets,
  type TargetRoomOption,
} from "~/lib/move-targets";
import {
  mayWritePersonalStorage,
  personalStorageOwner,
} from "~/lib/personal-storage";
import type { Room } from "~/lib/room-helpers";
import { useSWRAuth } from "~/lib/swr";
import { me as meKeys } from "~/lib/swr/me";
import { checkinTargetKeys } from "~/lib/swr/rooms-keys";
import { rooms as roomsKeys } from "~/lib/swr/rooms";
import {
  useAttendanceWebEnabled,
  usePresenceMode,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import type { Staff } from "~/lib/usercontext-helpers";

/** The choice a device remembers: enough to find the target again. */
export interface StoredCheckinTarget {
  readonly value: string;
  readonly kind: TargetRoomOption["kind"];
  readonly roomId: string;
  readonly roomName: string;
}

/**
 * Finds the remembered target among today's targets. A session that was
 * restarted in the same room keeps the choice; a room without a target any
 * more is `lost`, so the screen can say that children now go without a room.
 */
export function resolveCheckinTarget(
  stored: StoredCheckinTarget | null,
  options: readonly TargetRoomOption[],
): { target: TargetRoomOption | null; lost: boolean } {
  if (!stored) return { target: null, lost: false };
  const exact = options.find((option) => option.value === stored.value);
  if (exact) return { target: exact, lost: false };
  const sameRoom = options.filter(
    (option) => option.kind === stored.kind && option.roomId === stored.roomId,
  );
  if (sameRoom.length === 1 && sameRoom[0]) {
    return { target: sameRoom[0], lost: false };
  }
  return { target: null, lost: true };
}

/** Name of a target in lists: the room, and the activity running there. */
export function checkinTargetLabel(option: TargetRoomOption): string {
  if (option.kind === "openRoom") return option.label;
  return option.activityName
    ? `${option.roomName} · ${option.activityName}`
    : option.roomName;
}

/**
 * Per school and account (#5000): the next person on a shared school iPad
 * starts without this person's room. The room name makes it personal
 * storage, cleared on logout and account switch (personal-storage.ts).
 */
function targetStorageKey(
  tenantSlug: string | null,
  accountId: string | undefined,
): string | null {
  return tenantSlug && accountId
    ? `moto:checkin-target:${tenantSlug}:${accountId}`
    : null;
}

/** A list response, or nothing while it loads or when it is no list. */
function listOf<T>(data: readonly T[] | undefined): readonly T[] {
  return Array.isArray(data) ? data : [];
}

function readStored(key: string | null): StoredCheckinTarget | null {
  if (!key) return null;
  try {
    const raw = globalThis.localStorage?.getItem(key);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as Partial<StoredCheckinTarget>;
    if (
      typeof parsed.value !== "string" ||
      (parsed.kind !== "session" && parsed.kind !== "openRoom") ||
      typeof parsed.roomId !== "string" ||
      typeof parsed.roomName !== "string"
    ) {
      return null;
    }
    return parsed as StoredCheckinTarget;
  } catch {
    // Private mode or blocked storage: the choice lasts for this page only.
    return null;
  }
}

function writeStored(
  key: string | null,
  owner: string | null,
  value: StoredCheckinTarget | null,
) {
  if (!key) return;
  try {
    if (value) {
      // After a logout or an account switch, also in another tab, the choice
      // of the previous account must not come back (#5000).
      if (!mayWritePersonalStorage(owner)) return;
      globalThis.localStorage?.setItem(key, JSON.stringify(value));
    } else {
      globalThis.localStorage?.removeItem(key);
    }
  } catch {
    // Storage unavailable: the choice still holds in memory.
  }
}

export interface CheckinTargetState {
  /** False in binary mode or without web attendance: there is no choice. */
  readonly available: boolean;
  readonly options: readonly TargetRoomOption[];
  readonly loading: boolean;
  readonly loadError: unknown;
  /** The chosen target; null means "Anwesend (ohne Raum)". */
  readonly target: TargetRoomOption | null;
  /** "" picks "Anwesend (ohne Raum)", otherwise an option's value. */
  readonly setTargetValue: (value: string) => void;
  /** The remembered target that is gone, until the person acts on it. */
  readonly lostTargetName: string | null;
  readonly dismissLost: () => void;
  /** Reloads the targets, e.g. after the backend refused a stale choice. */
  readonly refresh: () => Promise<void>;
}

/**
 * The check-in target of the school check-in mode. The targets are the move
 * targets of the room page for children without a room: rooms with a running
 * supervision, released rooms and running activities. Administrators see
 * every unambiguous session. The backend checks the choice again.
 */
export function useCheckinTarget(enabled: boolean): CheckinTargetState {
  const presenceMode = usePresenceMode();
  const webAttendance = useAttendanceWebEnabled();
  const tenantSlug = useTenantSlugSafe();
  const { data: session } = useSession();
  const available = enabled && webAttendance && presenceMode === "detailed";
  const showAllTargets = canUseAllMoveTargets(session);

  const groups = useSWRAuth<ActiveGroup[]>(
    available ? checkinTargetKeys.activeGroups() : null,
    () => activeService.getActiveGroups({ active: true }),
  );
  const roomList = useSWRAuth<Room[]>(
    available ? roomsKeys.key() : null,
    roomsKeys.fetcher,
  );
  const staff = useSWRAuth<Staff>(
    available && !showAllTargets ? meKeys.key() : null,
    meKeys.fetcher,
  );
  const staffId = staff.data?.id;
  const supervisions = useSWRAuth<Supervisor[]>(
    available && !showAllTargets && staffId
      ? checkinTargetKeys.supervisions(staffId)
      : null,
    () => activeService.getStaffActiveSupervisions(staffId ?? ""),
  );

  // The session list names rooms only; the activity names come from the
  // activity list, so „OGS-Raum 1 · Hausaufgaben“ tells a running activity
  // from the free stay in the same released room.
  const activities = useSWRAuth<Activity[]>(
    available ? checkinTargetKeys.activities() : null,
    () => fetchActivities(),
  );
  const options = useMemo(() => {
    if (!available) return [];
    const activityNames = new Map(
      listOf(activities.data).map((activity) => [activity.id, activity.name]),
    );
    const namedGroups = listOf(groups.data).map((group) => {
      const name = group.groupId ? activityNames.get(group.groupId) : undefined;
      return name && !group.actualGroup
        ? { ...group, actualGroup: { id: Number(group.groupId), name } }
        : group;
    });
    const own = new Set(
      listOf(supervisions.data)
        .filter((supervision) => supervision.isActive)
        .map((supervision) => supervision.activeGroupId),
    );
    return buildTargetRoomOptions(
      namedGroups,
      listOf(roomList.data),
      "",
      showAllTargets ? "all" : "supervised",
      own,
    );
  }, [
    activities.data,
    available,
    groups.data,
    roomList.data,
    showAllTargets,
    supervisions.data,
  ]);

  const loaded =
    groups.data !== undefined &&
    roomList.data !== undefined &&
    (showAllTargets || supervisions.data !== undefined || staff.error);
  const loading = available && !loaded;
  const loadError =
    groups.error ?? roomList.error ?? staff.error ?? supervisions.error;

  const storageKey = targetStorageKey(tenantSlug, session?.user?.id);
  const storageOwner = personalStorageOwner(session?.user);
  const [stored, setStored] = useState<StoredCheckinTarget | null>(null);
  const [lostTargetName, setLostTargetName] = useState<string | null>(null);
  useEffect(() => {
    setStored(readStored(storageKey));
  }, [storageKey]);

  const resolved = useMemo(
    () => resolveCheckinTarget(stored, options),
    [stored, options],
  );

  // A remembered target that is not offered any more (the supervision ended,
  // the release was removed) falls back to "Anwesend (ohne Raum)" and says
  // so; it never silently checks children into another room.
  useEffect(() => {
    if (!available || !loaded || loadError || !resolved.lost || !stored) {
      return;
    }
    setLostTargetName(stored.roomName);
    setStored(null);
    writeStored(storageKey, storageOwner, null);
  }, [
    available,
    loaded,
    loadError,
    resolved.lost,
    stored,
    storageKey,
    storageOwner,
  ]);

  const setTargetValue = useCallback(
    (value: string) => {
      setLostTargetName(null);
      const option = options.find((candidate) => candidate.value === value);
      const next: StoredCheckinTarget | null = option
        ? {
            value: option.value,
            kind: option.kind,
            roomId: option.roomId,
            roomName: option.roomName,
          }
        : null;
      setStored(next);
      writeStored(storageKey, storageOwner, next);
    },
    [options, storageKey, storageOwner],
  );

  const refreshGroups = groups.mutate;
  const refreshRooms = roomList.mutate;
  const refreshSupervisions = supervisions.mutate;
  const refresh = useCallback(async () => {
    await Promise.all([refreshGroups(), refreshRooms(), refreshSupervisions()]);
  }, [refreshGroups, refreshRooms, refreshSupervisions]);

  return {
    available,
    options,
    loading,
    loadError,
    target: available ? resolved.target : null,
    setTargetValue,
    lostTargetName: available ? lostTargetName : null,
    dismissLost: useCallback(() => setLostTargetName(null), []),
    refresh,
  };
}
