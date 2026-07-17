"use client";

import { useSyncExternalStore } from "react";

const listeners = new Set<() => void>();
let currentTimestamp = Date.now();
let timer: ReturnType<typeof setInterval> | undefined;

function emitCurrentTimestamp() {
  currentTimestamp = Date.now();
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  if (listeners.size === 1) {
    emitCurrentTimestamp();
    timer = setInterval(emitCurrentTimestamp, 60_000);
  }

  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && timer !== undefined) {
      clearInterval(timer);
      timer = undefined;
    }
  };
}

function getSnapshot() {
  return currentTimestamp;
}

function getServerSnapshot() {
  return 0;
}

/**
 * Returns a minute-resolution client clock without rendering wall-clock time
 * during SSR. The stable server snapshot keeps hydration deterministic.
 */
export function useCurrentTimestamp() {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}
