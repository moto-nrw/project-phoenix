"use client";

import { useSyncExternalStore } from "react";
import {
  isSessionBarrierClosed,
  subscribeSessionBarrier,
} from "./session-end-state";

/**
 * The SessionProvider's `refetchInterval`: none while a logout runs (#5399).
 * Its timer stops at once instead of asking for the ending session.
 */
export function useSessionPollInterval(seconds: number): number {
  const closed = useSyncExternalStore(
    subscribeSessionBarrier,
    isSessionBarrierClosed,
    () => false,
  );
  return closed ? 0 : seconds;
}
