"use client";

import { createContext, useContext, useEffect, useRef } from "react";

export interface PrefetchResource {
  readonly key: string;
  readonly fetcher: () => Promise<unknown>;
}

export const DataPrefetchContext = createContext<
  ((href: string, resources?: readonly PrefetchResource[]) => void) | null
>(null);

const DATA_INTENT_DELAY_MS = 300;
const TOUCH_MOVE_TOLERANCE_PX = 8;
type TouchPoint = { identifier: number; clientX: number; clientY: number };
type TouchInput = {
  touches: ArrayLike<TouchPoint>;
  changedTouches: ArrayLike<TouchPoint>;
};
const moved = (start: TouchPoint, end: TouchPoint) =>
  Math.hypot(end.clientX - start.clientX, end.clientY - start.clientY) >
  TOUCH_MOVE_TOLERANCE_PX;

/** Shared by links and clickable rows, without importing a portal or API client. */
export function useDataPrefetchIntent(
  href: string | null,
  resources?: readonly PrefetchResource[],
) {
  const preload = useContext(DataPrefetchContext);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const touch = useRef<TouchPoint | null>(null);
  const touchFocus = useRef(false);
  const cancel = () => {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  };
  useEffect(
    () => () => {
      cancel();
      touch.current = null;
      touchFocus.current = false;
    },
    [href, resources, preload],
  );
  const start = () => {
    cancel();
    if (href) preload?.(href, resources);
  };
  const wait = () => {
    cancel();
    if (preload && href)
      timer.current = setTimeout(start, DATA_INTENT_DELAY_MS);
  };
  const cancelTouch = () => {
    cancel();
    touch.current = null;
  };
  return {
    onPointerEnter: (event: { pointerType: string }) => {
      if (event.pointerType === "mouse" || event.pointerType === "pen") wait();
    },
    onPointerLeave: cancel,
    onFocus: () => {
      if (!touchFocus.current) wait();
    },
    onBlur: () => {
      cancelTouch();
      touchFocus.current = false;
    },
    onTouchStart: (event: TouchInput) => {
      cancel();
      touchFocus.current = true;
      const point = event.touches?.[0];
      touch.current =
        event.touches?.length === 1 && point
          ? {
              identifier: point.identifier,
              clientX: point.clientX,
              clientY: point.clientY,
            }
          : null;
    },
    onTouchMove: (event: TouchInput) => {
      const point = event.touches?.[0];
      if (
        !point ||
        event.touches.length !== 1 ||
        !touch.current ||
        point.identifier !== touch.current.identifier ||
        moved(touch.current, point)
      )
        cancelTouch();
    },
    onTouchEnd: (event: TouchInput) => {
      const initial = touch.current;
      const end = event.changedTouches?.[0];
      cancelTouch();
      touchFocus.current = false;
      if (
        initial &&
        end &&
        event.touches.length === 0 &&
        initial.identifier === end.identifier &&
        !moved(initial, end)
      )
        start();
    },
    onTouchCancel: () => {
      cancelTouch();
      touchFocus.current = false;
    },
  };
}
