"use client";

import { useEffect } from "react";

const FLOATING_FAB_OFFSET_VAR = "--moto-floating-fab-offset";
const FLOATING_FAB_OFFSET = "4.5rem";

export const MOBILE_CREATE_FAB_MEDIA_QUERY = "(max-width: 767px)";

interface FloatingFabOffsetOptions {
  readonly active: boolean;
  readonly mediaQuery: string;
}

/** Publishes a FAB's height plus gap while its responsive variant is visible. */
export function useFloatingFabOffset({
  active,
  mediaQuery,
}: FloatingFabOffsetOptions): void {
  useEffect(() => {
    if (!active) return;

    const root = document.documentElement;
    const visibilityQuery = window.matchMedia(mediaQuery);
    const publishOffset = () => {
      if (visibilityQuery.matches) {
        root.style.setProperty(FLOATING_FAB_OFFSET_VAR, FLOATING_FAB_OFFSET);
      } else {
        root.style.removeProperty(FLOATING_FAB_OFFSET_VAR);
      }
    };

    publishOffset();
    visibilityQuery.addEventListener("change", publishOffset);
    return () => {
      visibilityQuery.removeEventListener("change", publishOffset);
      root.style.removeProperty(FLOATING_FAB_OFFSET_VAR);
    };
  }, [active, mediaQuery]);
}
