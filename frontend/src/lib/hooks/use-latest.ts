"use client";

import { useLayoutEffect, useRef } from "react";

/**
 * Keeps the most recently committed value available to asynchronous callbacks
 * without mutating refs during render.
 */
export function useLatest<T>(value: T) {
  const ref = useRef(value);

  useLayoutEffect(() => {
    ref.current = value;
  }, [value]);

  return ref;
}
