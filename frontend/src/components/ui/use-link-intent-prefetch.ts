"use client";

import {
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type FocusEvent,
  type PointerEvent,
} from "react";
import {
  useDataPrefetchIntent,
  type PrefetchResource,
} from "./data-prefetch-intent";
import type NextLink from "next/link";

/**
 * So lange muss die Maus auf dem Link bleiben, bevor er vorlädt. Wer mit der
 * Maus nur über die Seitenleiste fährt, löst damit nichts aus (#4987).
 */
export const HOVER_PREFETCH_DELAY_MS = 80;

type IntentPrefetchProps = Pick<
  ComponentProps<typeof NextLink>,
  | "target"
  | "prefetch"
  | "onPointerEnter"
  | "onPointerLeave"
  | "onFocus"
  | "onTouchStart"
> & { readonly href: string; readonly resources?: readonly PrefetchResource[] };

/** Shared intent policy, without portal progress or router dependencies. */
export function useLinkIntentPrefetch({
  href,
  target,
  prefetch: allowed,
  onPointerEnter,
  onPointerLeave,
  onFocus,
  onTouchStart,
  resources,
}: IntentPrefetchProps) {
  const [intent, setIntent] = useState({ href, target, prefetch: false });
  const sameDestination = intent.href === href && intent.target === target;
  const prefetch = sameDestination && intent.prefetch;
  // Reset before rendering next/link so a reused link never inherits intent.
  if (!sameDestination) setIntent({ href, target, prefetch: false });
  const hoverTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const canPrefetch =
    allowed !== false && target !== "_blank" && isInternalPath(href);

  const dataIntent = useDataPrefetchIntent(
    canPrefetch ? href : null,
    resources,
  );

  const cancelHover = () => {
    if (hoverTimer.current === null) return;
    clearTimeout(hoverTimer.current);
    hoverTimer.current = null;
  };
  useEffect(() => cancelHover, [href, target, allowed]);

  const startPrefetch = () => {
    cancelHover();
    if (canPrefetch) {
      dataIntent.prepare();
      setIntent({ href, target, prefetch: true });
    }
  };

  return {
    prefetch: prefetch && canPrefetch,
    onPointerEnter: (event: PointerEvent<HTMLAnchorElement>) => {
      onPointerEnter?.(event);
      if (event.defaultPrevented || !canPrefetch) return;
      dataIntent.onPointerEnter(event);
      if (prefetch) return;
      if (event.pointerType === "mouse") {
        cancelHover();
        hoverTimer.current = setTimeout(startPrefetch, HOVER_PREFETCH_DELAY_MS);
      } else {
        startPrefetch();
      }
    },
    onPointerLeave: (event: PointerEvent<HTMLAnchorElement>) => {
      onPointerLeave?.(event);
      cancelHover();
      dataIntent.onPointerLeave();
    },
    onTouchStart: (event: React.TouchEvent<HTMLAnchorElement>) => {
      onTouchStart?.(event);
      if (!event.defaultPrevented && canPrefetch) {
        dataIntent.onTouchStart();
        startPrefetch();
      }
    },
    onFocus: (event: FocusEvent<HTMLAnchorElement>) => {
      onFocus?.(event);
      if (event.defaultPrevented) return;
      startPrefetch();
      dataIntent.onFocus();
    },
  };
}

function isInternalPath(href: string): boolean {
  return href.startsWith("/") && !href.startsWith("//");
}
