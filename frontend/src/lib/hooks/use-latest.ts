"use client";

import { useLayoutEffect, useRef } from "react";

/**
 * Keeps the most recently committed value available to asynchronous callbacks
 * without mutating refs during render.
 *
 * The dependency array avoids unnecessary synchronous layout effects for
 * stable primitive values while still updating freshly-created callbacks.
 */
export function useLatest<T>(value: T) {
  const ref = useRef(value);

  useLayoutEffect(() => {
    ref.current = value;
  }, [value]);

  return ref;
}
