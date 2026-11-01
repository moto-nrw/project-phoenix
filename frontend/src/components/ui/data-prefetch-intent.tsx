"use client";

import { createContext, useContext, useEffect, useRef } from "react";

export interface PrefetchResource {
  readonly key: string;
  readonly fetcher: () => Promise<unknown>;
}

export const DataPrefetchContext = createContext<
  | (((href: string, resources?: readonly PrefetchResource[]) => void) & {
      prepare: (href: string) => void;
    })
  | null
>(null);

const DATA_HOVER_DELAY_MS = 300;

/** Shared by links and clickable rows, without importing a portal or API client. */
export function useDataPrefetchIntent(
  href: string | null,
  resources?: readonly PrefetchResource[],
) {
  const preload = useContext(DataPrefetchContext);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const cancel = () => {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  };
  useEffect(() => cancel, [href, resources, preload]);
  const start = () => {
    cancel();
    if (href) preload?.(href, resources);
  };
  return {
    prepare: () => {
      if (href && !resources) preload?.prepare(href);
    },
    onPointerEnter: (event: { pointerType: string }) => {
      cancel();
      if (!preload || !href) return;
      if (event.pointerType === "mouse") {
        timer.current = setTimeout(start, DATA_HOVER_DELAY_MS);
      } else if (event.pointerType === "pen") start();
    },
    onPointerLeave: cancel,
    onFocus: start,
    onTouchStart: start,
  };
}
