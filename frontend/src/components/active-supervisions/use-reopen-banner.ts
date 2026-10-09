"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

const REOPEN_STORAGE_KEY = "timetable-reopenable-instance";
const REOPEN_WINDOW_MS = 5 * 60 * 1000;

/**
 * The block that was just completed and can still be reopened. The banner
 * names it by `title` because it stays visible after the page has moved on to
 * another supervision (#3887); `roomId` lets the undo open the restored
 * session. Both are null for an entry stored before they existed.
 */
export interface ReopenableInstance {
  readonly instanceId: string;
  readonly title: string | null;
  readonly roomId: string | null;
}

interface StoredReopenable extends ReopenableInstance {
  readonly expiresAt: number;
}

function optionalString(value: unknown): string | null {
  return typeof value === "string" && value !== "" ? value : null;
}

function readStoredReopenBanner(): StoredReopenable | null {
  const raw = window.sessionStorage.getItem(REOPEN_STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as {
      instanceId?: string;
      expiresAt?: string | number;
      title?: unknown;
      roomId?: unknown;
    };
    if (!parsed.instanceId || parsed.expiresAt == null) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      return null;
    }
    const expiresAt =
      typeof parsed.expiresAt === "number"
        ? parsed.expiresAt
        : Date.parse(parsed.expiresAt);
    if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      return null;
    }
    return {
      instanceId: parsed.instanceId,
      title: optionalString(parsed.title),
      roomId: optionalString(parsed.roomId),
      expiresAt,
    };
  } catch {
    // Bewusst still: ein unlesbarer Eintrag heißt nur, dass das Angebot
    // „Rückgängig“ entfällt; die Aktivität selbst ist beendet.
    window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
    return null;
  }
}

/**
 * Tracks the recently completed timetable instance that can still be
 * reopened. The window survives a page reload via sessionStorage and expires
 * itself: after `reopenUntil` (fallback: five minutes) the banner disappears
 * without a render from outside.
 */
export function useReopenBanner(): {
  reopenable: ReopenableInstance | null;
  /**
   * Remember a just-completed instance. `reopenUntil` is the backend's
   * ISO deadline; an absent or invalid value falls back to now + 5 minutes,
   * an already-expired one clears the banner instead.
   */
  rememberReopenable: (
    instance: ReopenableInstance,
    reopenUntil: string | null | undefined,
  ) => void;
  clearReopenable: () => void;
} {
  const [storedReopen, setStoredReopen] = useState<StoredReopenable | null>(
    null,
  );

  useEffect(() => {
    setStoredReopen(readStoredReopenBanner());
  }, []);

  useEffect(() => {
    if (!storedReopen) return;
    const remainingMs = storedReopen.expiresAt - Date.now();
    if (remainingMs <= 0) {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      setStoredReopen(null);
      return;
    }
    const timeoutId = window.setTimeout(() => {
      window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
      setStoredReopen(null);
    }, remainingMs);
    return () => window.clearTimeout(timeoutId);
  }, [storedReopen]);

  const clearReopenable = useCallback(() => {
    window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
    setStoredReopen(null);
  }, []);

  const rememberReopenable = useCallback(
    (instance: ReopenableInstance, reopenUntil: string | null | undefined) => {
      const expiresAt = reopenUntil
        ? Date.parse(reopenUntil)
        : Date.now() + REOPEN_WINDOW_MS;
      if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
        window.sessionStorage.removeItem(REOPEN_STORAGE_KEY);
        setStoredReopen(null);
        return;
      }
      const stored: StoredReopenable = {
        instanceId: instance.instanceId,
        title: instance.title,
        roomId: instance.roomId,
        expiresAt,
      };
      window.sessionStorage.setItem(REOPEN_STORAGE_KEY, JSON.stringify(stored));
      setStoredReopen(stored);
    },
    [],
  );

  const reopenable = useMemo<ReopenableInstance | null>(
    () =>
      storedReopen
        ? {
            instanceId: storedReopen.instanceId,
            title: storedReopen.title,
            roomId: storedReopen.roomId,
          }
        : null,
    [storedReopen],
  );

  return { reopenable, rememberReopenable, clearReopenable };
}
