"use client";

import { useLayoutEffect, useRef } from "react";

let savedScrollTop = 0;

/** Keeps the desktop topic list in place when the catch-all route remounts. */
export function usePrototypeSidebarScrollRestoration() {
  const scrollAreaRef = useRef<HTMLDivElement | null>(null);

  useLayoutEffect(() => {
    const scrollArea = scrollAreaRef.current;
    if (!scrollArea) return;

    scrollArea.scrollTop = savedScrollTop;

    return () => {
      savedScrollTop = scrollArea.scrollTop;
    };
  }, []);

  return scrollAreaRef;
}
